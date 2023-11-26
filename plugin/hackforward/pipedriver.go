package hackforward

import (
	"errors"
	"github.com/miekg/dns"
	"k8s.io/apimachinery/pkg/util/rand"
	"sync"
	"time"
)

const (
	PRIMARY_PIPES_MAX   = 50
	SECONDARY_PIPES_MAX = 50
)

type PipeDriverImpl struct {
	upstreams      []ConnConfig
	primaryLimit   int
	secondaryLimit int
	pipes          []*Pipe
	pipesLock      sync.RWMutex

	primaryLoading   int
	secondaryLoading int
	loadingLock      sync.Mutex
}

type PipeDriver interface {
	removePipe(pipe *Pipe)
	pipeReady(pipe *Pipe)
	pipeInitFailed(pipe *Pipe)
	process(msg *dns.Msg, w dns.ResponseWriter) (int, error)
}

func NewDriver(upstreams []ConnConfig) *PipeDriverImpl {
	d := PipeDriverImpl{
		upstreams:      upstreams,
		primaryLimit:   PRIMARY_PIPES_MAX,
		secondaryLimit: SECONDARY_PIPES_MAX,
	}
	return &d
}

func (pd *PipeDriverImpl) removePipe(pipe *Pipe) {
	pd.pipesLock.Lock()
	defer pd.pipesLock.Unlock()
	for i := 0; i < len(pd.pipes); i++ {
		if pd.pipes[i] == pipe {
			log("Driver: pipe removed [%d]", pipe.id)
			pd.pipes = remove(pd.pipes, i)
			return
		}
	}
}

func (pd *PipeDriverImpl) pipeReady(pipe *Pipe) {
	log("Driver: pipe ready [%d]", pipe.id)

	pd.pipesLock.Lock()
	defer pd.pipesLock.Unlock()
	pd.pipes = append(pd.pipes, pipe)

	pd.loadingLock.Lock()
	if pipe.primary {
		pd.primaryLoading--
	} else {
		pd.secondaryLoading--
	}
	pd.loadingLock.Unlock()
}

func (pd *PipeDriverImpl) pipeInitFailed(pipe *Pipe) {
	log("Driver: pipe init failed [%d]", pipe.id)

	NewPipe(pd, pipe.primary, pd.selectUpstream(pipe.primary))
}

func (pd *PipeDriverImpl) selectUpstream(primary bool) ConnConfig {
	if primary || len(pd.upstreams) == 1 {
		return pd.upstreams[0]
	}
	return pd.upstreams[rand.IntnRange(1, len(pd.upstreams))]
}

func (pd *PipeDriverImpl) process(msg *dns.Msg, w dns.ResponseWriter) (int, error) {
	deadline := time.Now().Add(500 * time.Millisecond)
	for {
		log("Driver: process (%s)", msg.Question[0].Name)
		var pipe *Pipe
		pd.pipesLock.RLock()
		if len(pd.pipes) == 0 {
			pd.loadPipes()
		} else {
			pipeId := rand.Intn(len(pd.pipes))
			pipe = pd.pipes[pipeId]
		}
		pd.pipesLock.RUnlock()

		if pipe == nil {
			if time.Now().Before(deadline) {
				log("Driver: no pipe available -> retrying")
				time.Sleep(100 * time.Millisecond)
				continue
			}
			log("Driver: deadline exceeded")
			return dns.RcodeServerFailure, errors.New("no pipe available")
		}

		resp, err := pipe.process(msg)

		if err == nil || !errors.Is(err, writeNotReady) {
			if err == nil {
				if err = w.WriteMsg(resp); err != nil {
					return dns.RcodeServerFailure, err
				}
				return dns.RcodeSuccess, nil
			}
			return dns.RcodeServerFailure, err
		}
	}
}

func (pd *PipeDriverImpl) loadPipes() {
	pd.loadingLock.Lock()
	primary, secondary := pd.countPipes()
	if primary == 0 {
		loading := 0
		for i := 0; i < pd.primaryLimit-pd.primaryLoading; i++ {
			loading++
			NewPipe(pd, true, pd.selectUpstream(true))
		}
		pd.primaryLoading = loading + pd.primaryLoading

		loading = 0
		for i := 0; i < pd.secondaryLimit-secondary-pd.secondaryLoading; i++ {
			loading++
			NewPipe(pd, false, pd.selectUpstream(false))
		}
		pd.secondaryLoading = loading + pd.secondaryLoading
	} else {
		loading := 0
		for i := 0; i < pd.primaryLimit-primary-pd.primaryLoading; i++ {
			loading++
			NewPipe(pd, true, pd.selectUpstream(true))
		}
		pd.primaryLoading = loading + pd.primaryLoading
	}
	pd.loadingLock.Unlock()
}

func (pd *PipeDriverImpl) countPipes() (primary int, secondary int) {
	for i := 0; i < len(pd.pipes); i++ {
		if pd.pipes[i].primary {
			primary++
		} else {
			secondary++
		}
	}
	return
}

func remove[T any](slice []T, s int) []T {
	return append(slice[:s], slice[s+1:]...)
}
