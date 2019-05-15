// Package octwirp provides opencensus metrics and tracing for twirp services
package octwirp

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"go.opencensus.io/plugin/ochttp"
	"go.opencensus.io/trace/propagation"

	"github.com/twitchtv/twirp"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"go.opencensus.io/trace"
)

var (
	// PackageName is the twirp package
	PackageName, _ = tag.NewKey("twirp.package")

	// ServiceName is the twirp service
	ServiceName, _ = tag.NewKey("twirp.service")

	// MethodName is the twirp method
	MethodName, _ = tag.NewKey("twirp.method")

	// StatusCode is the twirp code
	StatusCode, _ = tag.NewKey("twirp.status")

	// ServerLatency measures server side latency
	ServerLatency = stats.Float64(
		"twirp/server/latency",
		"End-to-end latency",
		stats.UnitMilliseconds)

	// ServerLatencyView measures the latency distribution of HTTP requests
	ServerLatencyView = &view.View{
		Name:        "twirp/server/latency",
		Description: "Latency distribution of HTTP requests",
		Measure:     ServerLatency,
		Aggregation: ochttp.DefaultLatencyDistribution,
		TagKeys:     []tag.Key{PackageName, ServiceName, MethodName, StatusCode},
	}

	// ServerResponseView measures the server response count
	ServerResponseView = &view.View{
		Name:        "twirp/server/response_count",
		Description: "Server response count",
		TagKeys:     []tag.Key{PackageName, ServiceName, MethodName, StatusCode},
		Measure:     ServerLatency,
		Aggregation: view.Count(),
	}

	// ClientRoundtripLatency measures end to end latency from the client perspective.
	ClientRoundtripLatency = &view.View{
		Name:        "twirp/client/roundtrip_latency",
		Measure:     ochttp.ClientRoundtripLatency,
		Aggregation: ochttp.DefaultLatencyDistribution,
		Description: "End-to-end latency",
		TagKeys:     []tag.Key{PackageName, ServiceName, MethodName, ochttp.StatusCode},
	}
)

// WrapTransport wraps the ochttp transport to inject twirp metadata.
func WrapTransport(base *ochttp.Transport) http.RoundTripper {
	// before -> ochttp -> after -> base
	o := *base

	orig := o.Base
	if orig == nil {
		orig = http.DefaultTransport
	}

	a := afterTransport{
		next: orig,
	}

	o.Base = &a

	b := beforeTransport{
		next: &o,
	}
	return &b
}

type beforeTransport struct {
	next http.RoundTripper
}

func (t *beforeTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	ctx := r.Context()

	p, _ := twirp.PackageName(ctx)
	s, _ := twirp.ServiceName(ctx)
	m, _ := twirp.MethodName(ctx)

	ctx, _ = tag.New(ctx,
		tag.Insert(PackageName, p),
		tag.Insert(ServiceName, s),
		tag.Insert(MethodName, m),
	)
	r = r.WithContext(ctx)

	return t.next.RoundTrip(r)
}

type afterTransport struct {
	next http.RoundTripper
}

func (t *afterTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	ctx := r.Context()

	if span := trace.FromContext(ctx); span != nil {
		p, _ := twirp.PackageName(ctx)
		s, _ := twirp.ServiceName(ctx)
		m, _ := twirp.MethodName(ctx)

		span.AddAttributes(
			trace.StringAttribute(PackageName.Name(), p),
			trace.StringAttribute(ServiceName.Name(), s),
			trace.StringAttribute(MethodName.Name(), m),
		)
	}

	return t.next.RoundTrip(r)
}

type contextKey struct{}

var hookStateKey = contextKey{}

type hookState struct {
	startTime time.Time
	span      *trace.Span
}

// Tracer adds Opencensus tracing and metrics to twirp servers.
type Tracer struct {
	Propagation  propagation.HTTPFormat
	StartOptions trace.StartOptions
}

// WrapHandler wraps an http handler to inject Opencensus tracing and metrics.
func (t *Tracer) WrapHandler(handler http.Handler) http.Handler {
	o := ochttp.Handler{
		Propagation:  t.Propagation,
		StartOptions: t.StartOptions,
		Handler:      handler,
	}

	return &o
}

// ServerHooks creates twrip server hooks for Opencensus tracing and metrics.
func (t *Tracer) ServerHooks() *twirp.ServerHooks {
	return &twirp.ServerHooks{
		RequestReceived: t.requestReceived,
		RequestRouted:   t.requestRouted,
		ResponseSent:    t.responseSent,
	}
}

func (t *Tracer) requestReceived(ctx context.Context) (context.Context, error) {
	// method name has not been set by twirp yet
	p, _ := twirp.PackageName(ctx)
	s, _ := twirp.ServiceName(ctx)

	ctx, _ = tag.New(ctx,
		tag.Insert(PackageName, p),
		tag.Insert(ServiceName, s),
	)

	// TODO: package name is not required?
	ctx, span := trace.StartSpan(ctx,
		fmt.Sprintf(p+"."+s),
		trace.WithSampler(t.StartOptions.Sampler),
		trace.WithSpanKind(trace.SpanKindServer),
	)

	span.AddAttributes(
		trace.StringAttribute(PackageName.Name(), p),
		trace.StringAttribute(ServiceName.Name(), s),
	)

	hs := hookState{
		startTime: time.Now(),
		span:      span,
	}

	ctx = context.WithValue(ctx, hookStateKey, &hs)

	return ctx, nil
}

func (t *Tracer) requestRouted(ctx context.Context) (context.Context, error) {
	hs, ok := ctx.Value(hookStateKey).(*hookState)
	if !ok {
		return ctx, nil
	}
	m, _ := twirp.MethodName(ctx)

	ctx, _ = tag.New(ctx,
		tag.Insert(MethodName, m),
	)

	hs.span.AddAttributes(
		trace.StringAttribute(MethodName.Name(), m),
	)

	return ctx, nil
}

func (t *Tracer) responseSent(ctx context.Context) {
	s, _ := twirp.StatusCode(ctx)

	ctx, _ = tag.New(ctx, tag.Insert(StatusCode, s))

	hs, ok := ctx.Value(hookStateKey).(*hookState)
	if !ok {
		return
	}

	hs.span.AddAttributes(trace.StringAttribute(StatusCode.Name(), s))

	hs.span.End()

	diff := time.Since(hs.startTime)
	stats.Record(ctx, ServerLatency.M(diff.Seconds()*1000))
}
