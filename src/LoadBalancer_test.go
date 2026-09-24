package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// fakeServer is a stand-in backend for tests. It satisfies the Server
// interface but never makes a network call, so tests are fast and don't
// need real servers running.
type fakeServer struct {
	addr  string
	alive atomic.Bool
}

func newFake(addr string, alive bool) *fakeServer {
	f := &fakeServer{addr: addr}
	f.alive.Store(alive)
	return f
}

func (f *fakeServer) address() string     { return f.addr }
func (f *fakeServer) isAlive() bool       { return f.alive.Load() }
func (f *fakeServer) setAlive(alive bool) { f.alive.Store(alive) }
func (f *fakeServer) serve(rw http.ResponseWriter, r *http.Request) error {
	return nil
}

func TestGetNextAvailableServer(t *testing.T) {
	tests := []struct {
		name  string
		alive []bool // one entry per server: a, b, c...
		want  []string
	}{
		{"round robin wraps around", []bool{true, true, true}, []string{"a", "b", "c", "a"}},
		{"skips dead server", []bool{true, false, true}, []string{"a", "c", "a"}},
		{"all dead returns nil", []bool{false, false}, []string{""}},
		{"no servers returns nil", []bool{}, []string{""}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var servers []Server
			for i, alive := range tt.alive {
				servers = append(servers, newFake(string(rune('a'+i)), alive))
			}
			lb := NewLoadBalancer("8000", servers)

			for i, want := range tt.want {
				got := lb.getNextAvailableServer()
				gotAddr := ""
				if got != nil {
					gotAddr = got.address()
				}
				if gotAddr != want {
					t.Fatalf("request %d: got %q, want %q", i+1, gotAddr, want)
				}
			}
		})
	}
}

// Many requests at once while the health checker flips servers up and down.
// Before the atomics fix, `go test -race` fails here.
func TestConcurrentAccessIsRaceFree(t *testing.T) {
	servers := []Server{newFake("a", true), newFake("b", true), newFake("c", true)}
	lb := NewLoadBalancer("8000", servers)

	var wg sync.WaitGroup
	for g := 0; g < 50; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				lb.getNextAvailableServer()
			}
		}()
	}
	wg.Add(1)
	go func() { // plays the role of the health checker
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			servers[1].setAlive(i%2 == 0)
		}
	}()
	wg.Wait()
}

// With every server healthy, concurrent requests should still be spread
// perfectly evenly. A racy counter loses increments and skews this.
func TestConcurrentRequestsSpreadEvenly(t *testing.T) {
	servers := []Server{newFake("a", true), newFake("b", true), newFake("c", true)}
	lb := NewLoadBalancer("8000", servers)

	var mu sync.Mutex
	counts := map[string]int{}
	var wg sync.WaitGroup
	for g := 0; g < 30; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				addr := lb.getNextAvailableServer().address()
				mu.Lock()
				counts[addr]++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	for _, addr := range []string{"a", "b", "c"} {
		if counts[addr] != 1000 {
			t.Errorf("server %s got %d requests, want 1000 (all counts: %v)", addr, counts[addr], counts)
		}
	}
}

// --- Retry and passive failure detection, using real HTTP servers ---

// liveBackend starts a real HTTP server that replies with its name.
func liveBackend(t *testing.T, name string) *simpleServer {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "%s says hi", name)
	}))
	t.Cleanup(ts.Close)
	s, err := newSimpleServer(ts.URL, true)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// deadBackend returns a backend whose server has already shut down, so every
// connection is refused. It still starts out marked alive, just like a
// backend that crashed between two health checks.
func deadBackend(t *testing.T) *simpleServer {
	t.Helper()
	ts := httptest.NewServer(http.NotFoundHandler())
	ts.Close()
	s, err := newSimpleServer(ts.URL, true)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func send(lb *LoadBalancer, method string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, "/foo", strings.NewReader(""))
	lb.serveProxy(rec, req)
	return rec
}

func TestFailedGetIsRetriedOnNextBackend(t *testing.T) {
	dead := deadBackend(t)
	live := liveBackend(t, "live")
	lb := NewLoadBalancer("8000", []Server{dead, live})

	rec := send(lb, http.MethodGet)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %q)", rec.Code, rec.Body.String())
	}
	if body := rec.Body.String(); body != "live says hi" {
		t.Errorf("body = %q, want %q", body, "live says hi")
	}
	if dead.isAlive() {
		t.Error("dead backend should have been marked down after the failed request")
	}
	if !live.isAlive() {
		t.Error("live backend should still be marked alive")
	}
}

func TestFailedPostIsNotRetried(t *testing.T) {
	dead := deadBackend(t)
	live := liveBackend(t, "live")
	lb := NewLoadBalancer("8000", []Server{dead, live})

	rec := send(lb, http.MethodPost)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", rec.Code)
	}
	if dead.isAlive() {
		t.Error("dead backend should have been marked down")
	}

	// The next request skips the dead backend without waiting for a health check.
	rec = send(lb, http.MethodPost)
	if rec.Code != http.StatusOK || rec.Body.String() != "live says hi" {
		t.Errorf("follow-up request: status %d body %q, want 200 from live", rec.Code, rec.Body.String())
	}
}

func TestAllBackendsFailingReturns503(t *testing.T) {
	a, b := deadBackend(t), deadBackend(t)
	lb := NewLoadBalancer("8000", []Server{a, b})

	rec := send(lb, http.MethodGet)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	if a.isAlive() || b.isAlive() {
		t.Error("both backends should have been marked down")
	}
}

// If the client disconnects, the failure isn't the backend's fault.
func TestClientCancelDoesNotMarkBackendDown(t *testing.T) {
	block := make(chan struct{})
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-block
	}))
	t.Cleanup(ts.Close)
	t.Cleanup(func() { close(block) })

	backend, err := newSimpleServer(ts.URL, true)
	if err != nil {
		t.Fatal(err)
	}
	lb := NewLoadBalancer("8000", []Server{backend})

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		lb.serveProxy(rec, req)
		close(done)
	}()
	cancel()
	<-done

	if !backend.isAlive() {
		t.Error("backend was marked down because the client cancelled")
	}
}

// serve() must hand back the connection error instead of writing a 502,
// so the load balancer can still retry.
func TestServeReturnsErrorWithoutWriting(t *testing.T) {
	dead := deadBackend(t)
	rec := httptest.NewRecorder()

	err := dead.serve(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if err == nil {
		t.Fatal("expected an error from a backend that refuses connections")
	}
	if rec.Body.Len() != 0 {
		body, _ := io.ReadAll(rec.Body)
		t.Errorf("serve wrote %q to the response; it should leave that to the caller", body)
	}
	if errors.Is(err, context.Canceled) {
		t.Errorf("got context.Canceled, want a connection error: %v", err)
	}
}

func TestIsRetryable(t *testing.T) {
	for method, want := range map[string]bool{
		http.MethodGet: true, http.MethodHead: true, http.MethodOptions: true,
		http.MethodPost: false, http.MethodPut: false, http.MethodPatch: false, http.MethodDelete: false,
	} {
		if got := isRetryable(httptest.NewRequest(method, "/", nil)); got != want {
			t.Errorf("isRetryable(%s) = %t, want %t", method, got, want)
		}
	}
}
