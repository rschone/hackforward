package hackforward

import (
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/miekg/dns"
)

var timeoutErr = errors.New("request timeouted")

type Forward struct {
	conn      net.Conn
	readChan  chan *dns.Msg
	writeChan chan *dns.Msg
	cache     map[uint16]chan *dns.Msg
	timeout   time.Duration
}

func (dp *Forward) process(msg *dns.Msg) (error, *dns.Msg) {
	// precislovat msg id
	reqID := msg.Id
	respChan := make(chan *dns.Msg)
	dp.cache[reqID] = respChan
	defer delete(dp.cache, reqID)

	dp.writeChan <- msg

	select {
	case resp := <-respChan:
		return nil, resp
	case <-time.After(dp.timeout):
		return timeoutErr, nil
	}
}

// TODO: tcp/udp - jiny dialer
func (dp *Forward) init() error {
	var err error
	if dp.conn, err = net.Dial("tcp", "localhost:53"); err != nil {
		return err
	}

	dp.readChan = make(chan *dns.Msg)
	dp.writeChan = make(chan *dns.Msg)
	dp.cache = make(map[uint16]chan *dns.Msg)

	go dp.readLoop()
	go dp.writeLoop()

	return nil
}

func (dp *Forward) readLoop() {
	for {
		resp, err := dns.ReadMsg(dp.conn)
		if err != nil {
			fmt.Printf("Read error: %v\n", err)
			dp.cleanup()
			return
		}

		dp.readChan <- resp
	}
}

func (dp *Forward) writeLoop() {
	for {
		select {
		case req := <-dp.writeChan:
			err := dns.WriteMsg(dp.conn, req)
			if err != nil {
				fmt.Printf("Write error: %v\n", err)
				dp.cleanup()
				return
			}
		}
	}
}

func (dp *Forward) cleanup() {
	close(dp.readChan)
	close(dp.writeChan)

	if dp.conn != nil {
		dp.conn.Close()
	}
}
