package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync/atomic"
)

type simpleServer struct {
	addr       string
	proxy      *httputil.ReverseProxy
	aliveState atomic.Bool
}

type proxyErrKey struct{}

func newSimpleServer(addr string, aliveState bool) (*simpleServer, error) {
	serverUrl, err := url.Parse(addr)

	if err != nil {
		return nil, fmt.Errorf("parsing backend address %q: %w", addr, err)
	}

	s := &simpleServer{
		addr:  addr,
		proxy: httputil.NewSingleHostReverseProxy(serverUrl),
	}
	s.aliveState.Store(aliveState)

	s.proxy.ErrorHandler = func(rw http.ResponseWriter, r *http.Request, err error) {
		if slot, ok := r.Context().Value(proxyErrKey{}).(*error); ok {
			*slot = err
			return
		}
		http.Error(rw, "bad gateway", http.StatusBadGateway)
	}

	return s, nil
}

func (s *simpleServer) address() string { return s.addr }

func (s *simpleServer) isAlive() bool { return s.aliveState.Load() }

func (s *simpleServer) setAlive(alive bool) { s.aliveState.Store(alive) }

func (s *simpleServer) serve(rw http.ResponseWriter, req *http.Request) error {
	var proxyErr error
	ctx := context.WithValue(req.Context(), proxyErrKey{}, &proxyErr)
	s.proxy.ServeHTTP(rw, req.WithContext(ctx))
	return proxyErr
}
