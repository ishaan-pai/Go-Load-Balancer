package main

import (
	"net/http"
	"testing"
)

// fakeServer is a stand-in backend for tests. It satisfies the Server
// interface but never makes a network call, so tests are fast and don't
// need real servers running.
type fakeServer struct {
	addr  string
	alive bool
}

func (f *fakeServer) address() string                               { return f.addr }
func (f *fakeServer) isAlive() bool                                 { return f.alive }
func (f *fakeServer) setAlive(alive bool)                           { f.alive = alive }
func (f *fakeServer) serve(rw http.ResponseWriter, r *http.Request) {}

// Healthy servers should be picked in order, wrapping back to the start.
func TestRoundRobinOrder(t *testing.T) {
	a := &fakeServer{addr: "a", alive: true}
	b := &fakeServer{addr: "b", alive: true}
	c := &fakeServer{addr: "c", alive: true}
	lb := NewLoadBalancer("8000", []Server{a, b, c})

	want := []string{"a", "b", "c", "a"}
	for i, w := range want {
		got := lb.getNextAvailableServer()
		if got == nil || got.address() != w {
			t.Fatalf("request %d: got %v, want %s", i+1, got, w)
		}
	}
}

// A server marked dead should be skipped.
func TestSkipsDeadServer(t *testing.T) {
	a := &fakeServer{addr: "a", alive: true}
	b := &fakeServer{addr: "b", alive: false}
	c := &fakeServer{addr: "c", alive: true}
	lb := NewLoadBalancer("8000", []Server{a, b, c})

	want := []string{"a", "c", "a"}
	for i, w := range want {
		got := lb.getNextAvailableServer()
		if got == nil || got.address() != w {
			t.Fatalf("request %d: got %v, want %s", i+1, got, w)
		}
	}
}

// If every server is dead, there's nothing to pick.
func TestAllDeadReturnsNil(t *testing.T) {
	a := &fakeServer{addr: "a", alive: false}
	b := &fakeServer{addr: "b", alive: false}
	lb := NewLoadBalancer("8000", []Server{a, b})

	if got := lb.getNextAvailableServer(); got != nil {
		t.Fatalf("got %s, want nil", got.address())
	}
}
