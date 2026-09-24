package main

import (
	"log"
	"net/http"
	"sync/atomic"
	"time"
)

type LoadBalancer struct {
	port            string
	roundRobinCount atomic.Uint64
	servers         []Server
}

func NewLoadBalancer(port string, servers []Server) *LoadBalancer {
	return &LoadBalancer{
		port:    port,
		servers: servers,
	}
}

func (loadbalancer *LoadBalancer) getNextAvailableServer() Server {
	n := uint64(len(loadbalancer.servers))

	if n == 0 {
		return nil
	}

	for i := uint64(0); i < n; i++ {
		idx := (loadbalancer.roundRobinCount.Add(1) - 1) % n
		server := loadbalancer.servers[idx]

		if server.isAlive() {
			return server
		}
	}

	return nil
}

func (loadbalancer *LoadBalancer) startHealthCheckLoop(interval, timeout time.Duration) {
	client := &http.Client{Timeout: timeout}
	ticker := time.NewTicker(interval)

	go func() {
		for range ticker.C {
			for _, s := range loadbalancer.servers {
				alive := healthCheck(client, s.address())
				if alive != s.isAlive() {
					log.Printf("health check: %s alive=%t", s.address(), alive)
				}
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
	// Try each backend at most once.
	for attempt := 0; attempt < len(loadbalancer.servers); attempt++ {
		target := loadbalancer.getNextAvailableServer()
		if target == nil {
			break
		}

		err := target.serve(rw, req)
		if err == nil {
			return
		}

		if req.Context().Err() != nil {
			return
		}

		log.Printf("proxy to %s failed, marking it down: %v", target.address(), err)
		target.setAlive(false)

		if !isRetryable(req) {
			http.Error(rw, "bad gateway", http.StatusBadGateway)
			return
		}
	}
	http.Error(rw, "no healthy backends available", http.StatusServiceUnavailable)
}

func isRetryable(req *http.Request) bool {
	switch req.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	default:
		return false
	}
}
