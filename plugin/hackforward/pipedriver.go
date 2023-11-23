package hackforward

import (
	"errors"
	"github.com/miekg/dns"
	"k8s.io/apimachinery/pkg/util/rand"
	"sync"
)

type PipeDriverImpl struct {
	pipes []*Pipe
	lock  sync.RWMutex
	ready chan *Pipe
}

type PipeDriver interface {
	removePipe(pipe *Pipe)
	process(msg *dns.Msg, w dns.ResponseWriter) (int, error)
}

func NewDriver(connConfig ConnConfig) *PipeDriverImpl {
	d := PipeDriverImpl{
		ready: make(chan *Pipe),
	}
	go d.pipeManager(connConfig)
	return &d
}

func (pd *PipeDriverImpl) removePipe(pipe *Pipe) {
	pd.lock.Lock()
	defer pd.lock.Unlock()
	for i := 0; i < len(pd.pipes); i++ {
		if pd.pipes[i] == pipe {
			log("Driver: pipe removed")
			pd.pipes = remove(pd.pipes, i)
			return
		}
	}
}

func (pd *PipeDriverImpl) process(msg *dns.Msg, w dns.ResponseWriter) (int, error) {
	for {
		pd.lock.RLock()
		pipeId := rand.Intn(len(pd.pipes))
		pipe := pd.pipes[pipeId]
		pd.lock.RUnlock()
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

func (pd *PipeDriverImpl) pipeManager(connConfig ConnConfig) {
	NewPipe(pd, pd.ready, connConfig)

	for {
		select {
		case pipe := <-pd.ready:
			pd.lock.Lock()
			pd.pipes = append(pd.pipes, pipe)
			pd.lock.Unlock()
		}
	}
}

func remove[T any](slice []T, s int) []T {
	return append(slice[:s], slice[s+1:]...)
}
