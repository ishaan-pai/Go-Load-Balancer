package main

import (
	"log"
	"net/http"
	"os"
)

func main() {
	cfg, err := loadConfig(os.Getenv)
	if err != nil {
		log.Fatalf("invalid config: %v", err)
	}

	if cfg.Demo {
		log.Printf("LB_BACKENDS not set, starting built-in demo backends")
		startBackend(9001, "temp1")
		startBackend(9002, "temp2")
		startBackend(9003, "temp3")
	}

	servers := make([]Server, 0, len(cfg.Backends))
	for _, addr := range cfg.Backends {
		s, err := newSimpleServer(addr, true)
		if err != nil {
			log.Fatalf("creating backend: %v", err)
		}
		servers = append(servers, s)
	}

	loadbalancer := NewLoadBalancer(cfg.Port, servers)
	loadbalancer.startHealthCheckLoop(cfg.HealthInterval, cfg.HealthTimeout)

	http.HandleFunc("/", loadbalancer.serveProxy)

	log.Printf("serving requests at 'localhost:%s' across %d backends", cfg.Port, len(servers))
	log.Fatal(http.ListenAndServe(":"+cfg.Port, nil))
}
