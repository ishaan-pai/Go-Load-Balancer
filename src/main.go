package main

import (
	"fmt"
	"net/http"
	"time"
)

func main() {
	startBackend(9001, "temp1")
	startBackend(9002, "temp2")
	startBackend(9003, "temp3")

	servers := []Server{
		newSimpleServer("http://localhost:9001", true),
		newSimpleServer("http://localhost:9002", true),
		newSimpleServer("http://localhost:9003", true),
	}

	loadbalancer := NewLoadBalancer("8000", servers)
	loadbalancer.startHealthCheckLoop(2 * time.Second)

	handleRedirect := func(rw http.ResponseWriter, req *http.Request) {
		loadbalancer.serveProxy(rw, req)
	}

	http.HandleFunc("/", handleRedirect)

	fmt.Printf("serving requests at 'localhost:%s'\n", loadbalancer.port)
	http.ListenAndServe(":"+loadbalancer.port, nil)
}
