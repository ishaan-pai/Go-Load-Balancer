package main

import (
	"fmt"
	"net/http"
)

func main() {
	servers := []Server{
		newSimpleServer("https://www.youtube.com/"),
		newSimpleServer("https://www.bing.com/"),
		newSimpleServer("https://www.google.com/"),
	}

	loadbalancer := NewLoadBalancer("8000", servers)

	handleRedirect := func(rw http.ResponseWriter, req *http.Request) {
		loadbalancer.serveProxy(rw, req)
	}

	http.HandleFunc("/", handleRedirect)

	fmt.Printf("serving requests at 'localhost:%s'\n", loadbalancer.port)
	http.ListenAndServe(":"+loadbalancer.port, nil)
}
