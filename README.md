[![GoDoc](https://godoc.org/github.com/bakins/octwirp?status.svg)](https://godoc.org/github.com/bakins/octwirp)

# OpenCensus Tracing and Metrics for Twirp in Go

`octwirp` provides server hooks for [twirp](https://twitchtv.github.io/twirp/docs/intro.html) servers for 
metrics and tracing.

`octwirp` also provides an [http roundtripper](https://golang.org/pkg/net/http/#RoundTripper) for [twirp](https://twitchtv.github.io/twirp/docs/intro.html) clients for metrics and tracing.

## Usage

To install locally, `go get -u github.com/bakins/octwirp` or use a Go dependency manager to install.

### Server

To enable tracing and metrics for twirp servers:

```go
import (
    "net/http"

    "github.com/twitchtv/twirp/example"
    "github.com/bakins/octwirp"
    "go.opencensus.io/stats/view"
    "go.opencensus.io/plugin/ochttp"
)

func main() {
    if err := view.Register(octwirp.ServerLatencyView, octwirp.ServerResponseView); err != nil {
		log.Fatalf("failed to register metrics views: %v", err)
    }
    
    t := &octwirp.Tracer{}
    server := example.NewHaberdasherServer(&randomHaberdasher{}, t.ServerHooks())
    handler := t.WrapHandler(server)
    http.Handle(server.PathPrefix(), handler)
	log.Fatal(http.ListenAndServe("127.0.0.1:8080", nil))
}
```

You also need to register [Opencensus exporters](https://opencensus.io/exporters/) for metrics and tracing.

For a more complete server example, see [./examples/server/main.go](./examples/server/main.go)

### Client

To enable metrics and tracing for a twirp client:

```go
import (
    "net/http"

    "github.com/bakins/octwirp"
    "go.opencensus.io/stats/view"
    "github.com/twitchtv/twirp/example"
)

// see https://godoc.org/go.opencensus.io/plugin/ochttp
t := ochttp.Transport{}

c := http.Client{
		Transport: octwirp.WrapTransport(&t),
    }
    
client := example.NewHaberdasherProtobufClient("http://localhost:8080", &c)
```

For a more complete client example, see [./examples/client/main.go](./examples/client/main.go)

## Status

This project is under active development.  I have only recently began using twirp, so I am sure there are better ways to do things.

## LICENSE

See [LICENSE](./LICENSE)