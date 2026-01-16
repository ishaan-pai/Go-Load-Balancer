package main

import (
	"fmt"
	"log"
	"net/http"
	"time"
)

func startBackend(port int, name string) {
	mux := http.NewServeMux()

	// Main response so you can see which backend answered.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "%s says hi\npath=%s\n", name, r.URL.Path)
	})

	// Health endpoint for later.
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		Handler:           mux,
		ReadHeaderTimeout: 2 * time.Second,
	}

	go func() {
		log.Printf("backend %s listening on http://localhost:%d", name, port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("backend %s failed: %v", name, err)
		}
	}()
}
