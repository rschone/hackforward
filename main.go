package main

import (
	"bytes"
	"fmt"
	"github.com/coredns/coredns/core/dnsserver"
	_ "github.com/coredns/coredns/core/plugin"
	"github.com/coredns/coredns/coremain"
	_ "hackforward/plugin/hackforward"
	"os/exec"
	"time"
)

func init() {
	dnsserver.Directives = []string{
		"hack_forward",
	}
}

func main() {
	//reqs := []string{"seznam.cz"}
	reqs := []string{"seznam.cz", "google.com", "atlas.cz", "example.com", "zive.cz"}
	time.AfterFunc(1000*time.Millisecond, func() {
		for _, req := range reqs {
			cmd := exec.Command("dig", "@localhost", req)
			var out bytes.Buffer
			var stderr bytes.Buffer
			cmd.Stdout = &out
			cmd.Stderr = &stderr
			err := cmd.Run()
			if err != nil {
				fmt.Println(fmt.Sprint(err) + ": " + stderr.String())
				return
			}
			//fmt.Println("Command output:\n", out.String())
			time.Sleep(1000 * time.Millisecond)
		}
	})

	coremain.Run()
}
