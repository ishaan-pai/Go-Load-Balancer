package main

import (
	"net/http"
)

type Server interface {
	address() string
	isAlive() bool
	setAlive(bool)
	serve(rw http.ResponseWriter, r *http.Request)
}
