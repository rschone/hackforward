package hackforward

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/miekg/dns"
)

var timeoutErr = errors.New("request timeouted")
var writeNotReady = errors.New("writer not ready")

var pipeIDGen atomic.Int32

type Pipe struct {
	primary         bool
	driver          PipeDriver
	dialTimeout     time.Duration
	readTimeout     time.Duration
	writeTimeout    time.Duration
	finalizeTimeout time.Duration
	conn            *dns.Conn
	//readChan  chan *dns.Msg
	writeChan chan *dns.Msg

	writeReady bool
	writeLock  sync.Mutex

	reqTimeout time.Duration
	doneR      chan struct{}
	doneW      chan struct{}
	cache      SenderCache

	id int
}

func NewPipe(driver PipeDriver, primary bool, config ConnConfig) *Pipe {
	p := Pipe{
		id:              int(pipeIDGen.Add(1)),
		primary:         primary,
		cache:           SenderCache{cache: make(map[uint16]*Sender)},
		driver:          driver,
		dialTimeout:     1 * time.Second,
		readTimeout:     500 * time.Millisecond,
		writeTimeout:    5 * time.Millisecond,
		finalizeTimeout: 2 * time.Second,
		reqTimeout:      time.Second,
		doneR:           make(chan struct{}),
		doneW:           make(chan struct{}),
		writeChan:       make(chan *dns.Msg),
	}
	p.log("initialized primary(%v)", primary)

	go p.initConn(config)

	return &p
}

func (p *Pipe) initConn(cfg ConnConfig) {
	var err error
	if p.conn, err = dns.DialTimeout("tcp", fmt.Sprintf("%s:%d", cfg.Hostname, cfg.Port), p.dialTimeout); err != nil {
		p.log("%x: Initiating connection '%s:%d' failed", p, cfg.Hostname, cfg.Port)
		p.driver.pipeInitFailed(p)
		return
	}
	go p.readLoop()
	go p.writeLoop()
	go p.finalize()
	p.driver.pipeReady(p)
}

func (p *Pipe) finalize() {
	<-p.doneR
	<-p.doneW
	p.log("finalizing")
	if p.conn != nil {
		p.conn.Close()
	}
	p.driver = nil
	close(p.writeChan)
}

func (p *Pipe) isWriteReady() bool {
	p.writeLock.Lock()
	defer p.writeLock.Unlock()
	return p.writeReady
}

func (p *Pipe) setWriteReady(ready bool) {
	p.writeLock.Lock()
	defer p.writeLock.Unlock()
	p.log("W-goroutine ready (%v)", ready)
	p.writeReady = ready
}

func (p *Pipe) process(msg *dns.Msg) (*dns.Msg, error) {
	p.log("processing message (%d, %v)", msg.Id, msg.Question[0].Name)
	if !p.isWriteReady() {
		p.log("W-goroutine not ready")
		return nil, writeNotReady
	}

	oldMsgID, sender := p.cache.add(msg)
	p.writeChan <- msg

	select {
	case resp := <-sender.responseChan:
		p.log("message responded %v", resp.Answer[0])
		msg.Id = oldMsgID
		resp.Id = oldMsgID
		return resp, nil
	case err := <-sender.errChan:
		msg.Id = oldMsgID
		p.log("message error: %v", err)
		return nil, err
	case <-time.After(p.reqTimeout):
		p.log("message timeout id(%d)", msg.Id)
		p.cache.getAndRemove(msg.Id)
		return nil, timeoutErr
	}
}

func (p *Pipe) readLoop() {
	for {
		select {
		case <-p.doneR:
			return
		default:
			p.log("R initiated")
			err := p.conn.SetReadDeadline(time.Now().Add(p.readTimeout))
			if err != nil {
				p.log("R setting deadline failed -> killing pipe")
				p.closeRW(p.doneR, p.doneW)
				return
			}

			resp, err := p.conn.ReadMsg()
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					p.log("R deadlined")
					continue
				}
				p.log("R read failed %v -> killing pipe", err)
				p.closeRW(p.doneR, p.doneW)
				return
			}

			p.log("R received (%d)", resp.Id)
			sender := p.cache.getAndRemove(resp.Id)
			if sender != nil {
				sender.responseChan <- resp
			}
		}
	}
}

func (p *Pipe) closeRW(now chan struct{}, later chan struct{}) {
	p.setWriteReady(false)
	safeClose(now)
	p.driver.removePipe(p)
	p.resurrectReqs()
	safeClose(later)
}

func safeClose(ch chan struct{}) {
	select {
	case <-ch:
		return
	default:
		close(ch)
	}
}

func (p *Pipe) writeLoop() {
	p.setWriteReady(true)
	for {
		select {
		case <-p.doneW:
			p.log("W #")
			return
		case req := <-p.writeChan:
			p.log("W receiving (%d)", req.Id)
			err := p.conn.SetWriteDeadline(time.Now().Add(p.writeTimeout))
			if err != nil { //} || rand.Intn(3) != 0 {
				p.log("W deadline failure")
				p.closeWriteLoop(req)
				return
			}

			err = p.conn.WriteMsg(req)
			if err != nil {
				p.log("W write err: %v", err)
				p.closeWriteLoop(req)
				return
			}

			p.log("W write success (%d)", req.Id)
		}
	}
}

func (p *Pipe) closeWriteLoop(req *dns.Msg) {
	p.setWriteReady(false)
	p.driver.removePipe(p)
	safeClose(p.doneW)
	time.AfterFunc(p.finalizeTimeout, func() { safeClose(p.doneR) })
	if sender := p.cache.getAndRemove(req.Id); sender != nil {
		sender.errChan <- writeNotReady
	}
	p.resurrectReqs()
}

func (p *Pipe) resurrectReqs() {
	for {
		select {
		case req := <-p.writeChan:
			if sender := p.cache.getAndRemove(req.Id); sender != nil {
				p.log("Resurrecting request (%d)", req.Id)
				sender.errChan <- writeNotReady
			}
		default:
			return
		}
	}
}

func (p *Pipe) log(format string, a ...any) {
	s := fmt.Sprintf("%d [%d] ", time.Now().UnixNano()/1000, p.id) + fmt.Sprintf(format, a...)
	fmt.Println(s)
}

func log(format string, a ...any) {
	s := fmt.Sprintf("%d ", time.Now().UnixNano()/1000) + fmt.Sprintf(format, a...)
	fmt.Println(s)
}
