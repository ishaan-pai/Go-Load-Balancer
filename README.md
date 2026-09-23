# Go Load Balancer
 
A round-robin HTTP load balancer written in Go using only the standard library. It proxies incoming requests across a pool of backends, runs periodic health checks, and skips any backend that fails one.
 
This is a learning project, not a production load balancer. The goal was to build the core mechanics behind something like an AWS ALB (request distribution, health checking, reverse proxying) from scratch and see how they fit together.
 
## How it works
 
```
client ──▶ :8000 load balancer ──▶ :9001 temp1
                               ├──▶ :9002 temp2
                               └──▶ :9003 temp3
```
 
**Round robin.** The balancer keeps a counter and picks `servers[count % n]` for each request, incrementing as it goes. If the chosen backend is marked unhealthy, it moves to the next one, trying each backend at most once. If none are healthy, the client gets a `503 Service Unavailable`.
 
**Health checks.** A background goroutine runs on a 2-second ticker and sends `GET /health` to every backend with a 1-second timeout. A backend is marked alive only if it returns `200 OK`; timeouts and connection errors mark it dead. Backends that recover are picked up again on the next tick.
 
**Proxying.** Each backend wraps an `httputil.ReverseProxy`, so the request path, headers, and body are forwarded as-is and the backend's response is streamed back to the client.
 
**Test backends.** `main.go` starts three small HTTP servers in the same process on ports 9001–9003. Each one replies with its name and the requested path, which makes the rotation easy to see, and exposes a `/health` endpoint.
 
## Project structure
 
| File | What it is |
| --- | --- |
| `src/main.go` | Starts the test backends, builds the server pool, starts the health-check loop, and listens on `:8000`. |
| `src/LoadBalancer.go` | `LoadBalancer` type: round-robin selection, health-check loop, and the proxy handler. |
| `src/Server.go` | `Server` interface (`address`, `isAlive`, `setAlive`, `serve`), so the balancer isn't tied to one backend implementation. |
| `src/simpleServer.go` | `Server` implementation backed by `httputil.NewSingleHostReverseProxy`. |
| `src/backends.go` | The local test backends used for demoing. |
| `Dockerfile` | Builds and runs the balancer in a `golang:1.22-alpine` image. |
 
## Running it
 
Requires Go 1.22 or newer. No external dependencies.
 
```bash
cd src
go run .
```
 
Then hit it a few times:
 
```bash
curl localhost:8000/foo
```
 
```
temp1 says hi
path=/foo
temp2 says hi
path=/foo
temp3 says hi
path=/foo
temp1 says hi
path=/foo
```
 
### With Docker
 
```bash
docker build -t go-load-balancer .
docker run -p 8000:8000 go-load-balancer
```
 
The test backends run inside the same container, so only port 8000 needs to be published.
 
## Limitations
 
These are known gaps, left in because the project is scoped as a proof of concept:
 
- The round-robin counter and each backend's alive flag are shared between request goroutines and the health-check goroutine without a mutex or atomics, so there's a data race under concurrent load.
- Backend addresses, ports, and the health-check interval are hardcoded in `main.go` rather than read from config.
- Backends are only marked dead by the periodic health check. A request that fails mid-proxy isn't retried on another backend and doesn't mark that backend down.
- Plain round robin only; no weighting, least-connections, or sticky sessions.
## License
 
MIT
