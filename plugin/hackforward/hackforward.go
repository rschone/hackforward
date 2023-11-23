package hackforward

import (
	"context"

	"github.com/coredns/coredns/plugin"
	"github.com/miekg/dns"
)

type handler struct {
	Next       plugin.Handler
	pipeDriver PipeDriver
}

func (h *handler) Name() string { return pluginName }

func (h *handler) ServeDNS(_ context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	log("forward: %v", r.Question[0].Name)
	return h.pipeDriver.process(r, w)
}
