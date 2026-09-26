# Go Load Balancer
 
A round-robin HTTP load balancer written in Go using only the standard library. It proxies incoming requests across a pool of backends, runs periodic health checks, skips any backend that fails one, and retries a failed request on the next backend.
 
This is a learning project, not a production load balancer. The goal was to build the core mechanics behind something like an AWS ALB (request distribution, health checking, reverse proxying) from scratch and see how they fit together to avoid simply treating it as a black box. 

This project has now been hosted on an AWS EC2 instance. (tested and successfully working using curl http://35.182.240.101:8000/foo)
 
## How it works
 
```
client ──▶ :8000 load balancer ──▶ :9001 temp1
                               ├──▶ :9002 temp2
                               └──▶ :9003 temp3
```
 
**Round robin.** The balancer keeps a counter and picks `servers[count % n]` for each request, incrementing as it goes. If the chosen backend is marked unhealthy, it moves to the next one, trying each backend at most once. If none are healthy, the client gets a `503 Service Unavailable`.
 
**Concurrency.** Every request runs in its own goroutine, and the health checker runs in another, so the round-robin counter and each backend's alive flag are read and written at the same time. Both use `sync/atomic` (`atomic.Uint64` and `atomic.Bool`), which keeps them race-free without a lock on the request path. CI runs the tests with `-race`, and `TestConcurrentAccessIsRaceFree` fails if either goes back to a plain field.
 
**Health checks.** A background goroutine sends `GET /health` to every backend on a ticker (2 seconds by default) with a timeout (1 second by default). A backend is marked alive only if it returns `200 OK`; timeouts and connection errors mark it dead. Backends that recover are picked up again on the next tick.
 
**Failure detection and retries.** A backend can die between two health checks. When a proxied request can't reach its backend, the balancer marks that backend down right away instead of waiting for the next tick. `GET`, `HEAD` and `OPTIONS` requests are then retried on the next healthy backend, so the client never sees the failure. Other methods get a `502 Bad Gateway` instead, because a `POST` that partly reached a backend could be applied twice if it were replayed. If the client disconnects mid-request, the backend is left alone, since that failure isn't its fault.
 
**Proxying.** Each backend wraps an `httputil.ReverseProxy`, so the request path, headers, and body are forwarded as-is and the backend's response is streamed back to the client.
 
**Test backends.** When `LB_BACKENDS` isn't set, `main.go` starts three small HTTP servers in the same process on ports 9001–9003 and balances across them. Each one replies with its name and the requested path, which makes the rotation easy to see, and exposes a `/health` endpoint.
 
## Project structure
 
| File | What it is |
| --- | --- |
| `src/main.go` | Loads config, starts the demo backends if needed, builds the server pool, starts the health-check loop, and listens. |
| `src/config.go` | Reads and validates the `LB_*` environment variables. |
| `src/LoadBalancer.go` | `LoadBalancer` type: round-robin selection, health-check loop, and the proxy handler with retries. |
| `src/Server.go` | `Server` interface (`address`, `isAlive`, `setAlive`, `serve`), so the balancer isn't tied to one backend implementation. |
| `src/simpleServer.go` | `Server` implementation backed by `httputil.NewSingleHostReverseProxy`. Reports connection failures back to the balancer instead of answering with a 502 itself. |
| `src/backends.go` | The local test backends used for demoing. |
| `Dockerfile` | Builds and runs the balancer in a `golang:1.22-alpine` image. |
| `.github` | Sets up CI/CD pipeline workflows |
 
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
 
The image is also built and published by CI on every push to `main`:
 
```bash
docker run -p 8000:8000 ghcr.io/ishaan-pai/go-load-balancer:latest
```
 
## Configuration
 
Everything is set through environment variables, and all of them are optional. See `.env.example`.
 
| Variable | Default | What it does |
| --- | --- | --- |
| `LB_PORT` | `8000` | Port the balancer listens on. |
| `LB_BACKENDS` | *(unset)* | Comma-separated backend URLs, e.g. `http://app1:8080,http://app2:8080`. When unset, the three demo backends are started. |
| `LB_HEALTH_INTERVAL` | `2s` | How often each backend is health checked. Any Go duration (`500ms`, `5s`, `1m`). |
| `LB_HEALTH_TIMEOUT` | `1s` | How long a health check waits for a reply. Must be shorter than the interval. |
 
Values are validated at startup, and the process exits with a clear message if one is wrong:
 
```bash
LB_BACKENDS=localhost:9001 go run .
# invalid config: LB_BACKENDS: "localhost:9001" must start with http:// or https://
```
 
With Docker, pass them with `-e`. Inside a container, `localhost` is the container itself, so point at the backends' real hostnames:
 
```bash
docker run -p 8000:8000 -e LB_BACKENDS=http://app1:8080,http://app2:8080 go-load-balancer
```
 
## Limitations
 
These are known gaps, left in because the project is scoped as a proof of concept:
 
- Plain round robin only; no weighting, least-connections, or sticky sessions.
- Only `GET`, `HEAD` and `OPTIONS` are retried. A failed `POST`, `PUT`, `PATCH` or `DELETE` returns `502` rather than risk running twice.
- A retry only happens when the backend can't be reached at all. If a backend accepts the request and then dies partway through its response, the client gets a cut-off response.
- Config is read once at startup, so changing the backend list means restarting the balancer.
