package main

import (
	"net/http"
	"time"
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
	temp := len(loadbalancer.servers)
	if temp == 0 {
		return nil
	}
	for i := 0; i < temp; i++ {
		server := loadbalancer.servers[loadbalancer.roundRobinCount%temp]
		loadbalancer.roundRobinCount++
		if server.isAlive() {
			return server
		}
	}
	return nil

}

func (loadbalancer *LoadBalancer) startHealthCheckLoop(interval time.Duration) {
	client := &http.Client{Timeout: 1 * time.Second}
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			for _, s := range loadbalancer.servers {
				alive := healthCheck(client, s.address())
				s.setAlive(alive)
			}
		}
	}()
}

func healthCheck(client *http.Client, baseAddr string) bool {
	resp, err := client.Get(baseAddr + "/health")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func (loadbalancer *LoadBalancer) serveProxy(rw http.ResponseWriter, req *http.Request) {
	targetServer := loadbalancer.getNextAvailableServer()
	if targetServer == nil {
		http.Error(rw, "no healthy backends available", http.StatusServiceUnavailable)
		return
	}
	targetServer.serve(rw, req)
}
