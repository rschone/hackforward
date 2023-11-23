package hackforward

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/miekg/dns"
)

var timeoutErr = errors.New("request timeouted")
var writeNotReady = errors.New("writer not ready")

type Pipe struct {
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
	ready      chan *Pipe // channel provided by Driver to let her know that connection is ready to be used
	cache      SenderCache
}

func NewPipe(driver PipeDriver, ready chan *Pipe, config ConnConfig) *Pipe {
	p := Pipe{
		cache:           SenderCache{cache: make(map[uint16]Sender)},
		driver:          driver,
		dialTimeout:     1 * time.Second,
		readTimeout:     500 * time.Millisecond,
		writeTimeout:    5 * time.Millisecond,
		finalizeTimeout: 1 * time.Second,
		reqTimeout:      time.Second,
		doneR:           make(chan struct{}),
		doneW:           make(chan struct{}),
		writeChan:       make(chan *dns.Msg),
		ready:           ready,
	}
	//p.cache = make(map[uint16]chan *dns.Msg)

	go p.initConn(config)

	return &p
}

func (p *Pipe) initConn(cfg ConnConfig) {
	var err error
	if p.conn, err = dns.DialTimeout("tcp", fmt.Sprintf("%s:%d", cfg.Hostname, cfg.Port), p.dialTimeout); err != nil {
		log("%x: Initiating connection '%s:%d' failed", p, cfg.Hostname, cfg.Port)
		// TODO: let driver know that initialization failed
		return
	}
	go p.readLoop()
	go p.writeLoop()
	go p.finalize()
	p.ready <- p
}

func (p *Pipe) finalize() {
	<-p.doneR
	<-p.doneW
	log("Pipe finalize")
}

func (p *Pipe) isWriteReady() bool {
	p.writeLock.Lock()
	defer p.writeLock.Unlock()
	return p.writeReady
}

func (p *Pipe) setWriteReady(ready bool) {
	p.writeLock.Lock()
	defer p.writeLock.Unlock()
	log("Pipe set W = %v", ready)
	p.writeReady = ready
}

func (p *Pipe) process(msg *dns.Msg) (*dns.Msg, error) {
	log("Pipe entered: %v", msg.Question[0].Name)
	if !p.isWriteReady() {
		log("Pipe W not ready")
		return nil, writeNotReady
	}

	oldMsgID, responseChan := p.cache.add(msg)
	p.writeChan <- msg

	select {
	case resp := <-responseChan:
		log("Pipe responded %v", resp.Answer[0])
		resp.Id = oldMsgID
		return resp, nil
	case <-time.After(p.reqTimeout):
		// TODO: what with the timeouted request?
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
			log("R initiated")
			err := p.conn.SetReadDeadline(time.Now().Add(p.readTimeout))
			if err != nil {
				log("R setting deadline failed -> killing pipe")
				p.closeRW(p.doneR, p.doneW)
				log("R #")
				return
			}

			resp, err := p.conn.ReadMsg()
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					log("R deadlined")
					continue
				}
				log("R read failed %v -> killing pipe", err)
				p.closeRW(p.doneR, p.doneW)
				log("R #")
				return
			}

			log("R Received %d", resp.Id)
			respChan := p.cache.getAndRemove(resp.Id)
			if respChan != nil {
				respChan <- resp
			}
		}
	}
}

func (p *Pipe) closeRW(now chan struct{}, later chan struct{}) {
	safeClose(now)
	p.setWriteReady(false)
	p.driver.removePipe(p)
	time.AfterFunc(p.finalizeTimeout, func() { safeClose(later) })
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
			log("W #")
			return
		case req := <-p.writeChan:
			log("W reciving %d", req.Id)
			err := p.conn.SetWriteDeadline(time.Now().Add(p.writeTimeout))
			if err != nil {
				// TODO: let sender know, that it failed
				// TODO: kill the pipe
			}

			err = p.conn.WriteMsg(req)
			if err != nil {
				log("W write err: %v", err)
				// TODO: let sender know, that it failed
				// TODO: kill the pipe
			}

			log("W write success %d", req.Id)
		}
	}
}

//func (p *Pipe) cleanup() {
//	close(p.readChan)
//	close(p.writeChan)
//
//	if p.conn != nil {
//		p.conn.Close()
//	}
//}

func log(format string, a ...any) {
	fmt.Printf(format+"\n", a...)
}
