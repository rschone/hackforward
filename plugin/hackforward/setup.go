package hackforward

import (
	"github.com/coredns/caddy"
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	"hackforward/pkg/corefile"
)

const (
	pluginName = "hack_forward"
)

func init() {
	plugin.Register(pluginName, setup)
}

func setup(c *caddy.Controller) error {
	var cfg config
	if err := corefile.Parse(c, &cfg); err != nil {
		return err
	}

	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		return &handler{Next: next}
	})

	return nil
}
