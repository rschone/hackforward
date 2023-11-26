# Hackforward

Hackforward is a proof-of-concept DNS forwarder using TCP pipelining, written in Go. 

## Overview

The main purpose of this project is to demonstrate & test the use of TCP pipelining in forwarding DNS requests.

## Benchmarking

1. Compile & run `hackforward`. It will start accepting DNS requests on 127.0.0.1:53.
2. Execute `dnspyre` to benchmark it:

```
dnspyre --duration 60s -c 10 --server 127.0.0.1:53 google.com
```

## Results

* Even a single slow transmission can slow down the whole pipeline.
* Aggressive upstream connection policy (unused connections are killed in ~2-3s) implies the need of a connection management.
* RFCs recommends single connection using pipelining per upstream, but this is not usable for higher traffic. For a traffic hitting lets say 200k-1M reqs/second, we would need at least hundreds of connections.
* Proper connection selection would have to be implemented over watching the connection activity.  

