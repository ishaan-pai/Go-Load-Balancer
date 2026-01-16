package main

import (
	"fmt"
	"net/http"
)

type LoadBalancer struct {
	port            string
	roundRobinCount int
	servers         []Server
}

func NewLoadBalancer(port string, servers []Server) *LoadBalancer {
	return &LoadBalancer{
		port:            port,
		roundRobinCount: 0,
		servers:         servers,
	}
}

func (loadbalancer *LoadBalancer) getNextAvailableServer() Server {
	server := loadbalancer.servers[loadbalancer.roundRobinCount%len(loadbalancer.servers)]
	for !server.isAlive() {
		loadbalancer.roundRobinCount++
		server = loadbalancer.servers[loadbalancer.roundRobinCount%len(loadbalancer.servers)]
	}
	loadbalancer.roundRobinCount++
	return server
}

func (loadbalancer *LoadBalancer) serveProxy(rw http.ResponseWriter, req *http.Request) {
	targetServer := loadbalancer.getNextAvailableServer()
	fmt.Printf("forwarding request to address %q\n", targetServer.address())
	targetServer.serve(rw, req)
}
