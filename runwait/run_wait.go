package main

import (
	"flag"

	"github.com/simple-rules/harmony-benchmark/p2p"
	"github.com/simple-rules/harmony-benchmark/waitnode"
)

func main() {
	ip := flag.String("ip", "127.0.0.0", "IP of the node")
	port := flag.String("port", "8080", "port of the node")
	flag.Parse()
	peer := p2p.Peer{Ip: *ip, Port: *port}
	idcpeer := p2p.Peer{Ip: "127.0.0.0", Port: "9000"} //Hardcoded here.
	node := waitnode.New(peer)
	node.ConnectIdentityChain(idcpeer)
}
