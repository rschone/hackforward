package hackforward

import (
	"github.com/miekg/dns"
	"sync"
)

type SenderCache struct {
	cache     map[uint16]*Sender
	cacheLock sync.Mutex
	msgIDGen  uint16
}

type Sender struct {
	responseChan chan *dns.Msg
	errChan      chan error
}

func (c *SenderCache) add(msg *dns.Msg) (uint16, *Sender) {
	c.cacheLock.Lock()
	defer c.cacheLock.Unlock()
	oldMsgId := msg.Id
	c.msgIDGen++
	msg.Id = c.msgIDGen
	s := &Sender{
		responseChan: make(chan *dns.Msg),
		errChan:      make(chan error),
	}
	log("Cache: adding %d", msg.Id)
	c.cache[msg.Id] = s
	return oldMsgId, s
}

func (c *SenderCache) getAndRemove(id uint16) *Sender {
	c.cacheLock.Lock()
	defer c.cacheLock.Unlock()
	sender, ok := c.cache[id]
	if !ok {
		log("Cache: unknown id %d", id)
		return nil
	}
	log("Cache: get %d", id)
	delete(c.cache, id)
	return sender
}
