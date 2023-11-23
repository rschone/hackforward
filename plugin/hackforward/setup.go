package hackforward

import (
	"errors"
	"github.com/coredns/caddy"
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	"hackforward/pkg/corefile"
	"strconv"
	"strings"
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

	h := handler{}
	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		h.Next = next
		return &h
	})

	upstreams, err := convertUpstreams(cfg.Upstreams)
	if err != nil {
		return err
	}

	c.OnStartup(func() error {
		h.pipeDriver = NewDriver(upstreams)
		return nil
	})

	return nil
}

func convertUpstreams(upstreams []string) (cfgs []ConnConfig, err error) {
	for _, upstream := range upstreams {
		parts := strings.Split(upstream, ":")
		if len(parts) == 0 || len(parts) > 2 {
			return nil, errors.New("upstream parsing failed")
		}
		cfg := ConnConfig{Hostname: parts[0], Port: 53}
		if len(parts) == 2 {
			if port, err := strconv.Atoi(parts[1]); err == nil {
				cfg.Port = port
			} else {
				return nil, err
			}
		}
		cfgs = append(cfgs, cfg)
	}
	return cfgs, err
}
