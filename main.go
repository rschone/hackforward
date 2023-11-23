package main

import (
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/coremain"
)

func init() {
	dnsserver.Directives = []string{
		"log",
		"hackforward",
	}
}

func main() {
	coremain.Run()
}

//func main() {
//	dp := Forward{
//		timeout: time.Second * 5,
//	}
//
//	err := dp.init()
//	if err != nil {
//		fmt.Printf("Initialization error: %v\n", err)
//		return
//	}
//
//	// Example usage
//	req := new(dns.Msg)
//	req.SetQuestion("example.com.", dns.TypeA)
//
//	err, resp := dp.process(req)
//	if err != nil {
//		fmt.Printf("Processing error: %v\n", err)
//		return
//	}
//
//	fmt.Printf("Response: %v\n", resp)
//}