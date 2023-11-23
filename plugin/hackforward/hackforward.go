package hackforward

import (
	"context"

	"github.com/coredns/coredns/plugin"
	"github.com/miekg/dns"
)

type handler struct {
	Next plugin.Handler
}

func (h *handler) Name() string { return pluginName }

func (h *handler) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {

	return dns.RcodeSuccess, nil
}
