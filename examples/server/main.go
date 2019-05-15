// based on https://github.com/twitchtv/twirp/blob/337e90237d72193bf7f9fa387b5b9946436b7733/example/cmd/server/main.go
// Copyright 2018 Twitch Interactive, Inc.  All Rights Reserved.

package main

import (
	"context"
	"log"
	"math/rand"
	"net/http"
	"sort"
	"strings"
	"time"

	"contrib.go.opencensus.io/exporter/prometheus"
	"github.com/twitchtv/twirp"
	"github.com/twitchtv/twirp/example"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/trace"

	"github.com/bakins/octwirp"
)

type randomHaberdasher struct{}

func (h *randomHaberdasher) MakeHat(ctx context.Context, size *example.Size) (*example.Hat, error) {
	if size.Inches <= 0 {
		return nil, twirp.InvalidArgumentError("Inches", "I can't make a hat that small!")
	}
	return &example.Hat{
		Size:  size.Inches,
		Color: []string{"white", "black", "brown", "red", "blue"}[rand.Intn(4)],
		Name:  []string{"bowler", "baseball cap", "top hat", "derby"}[rand.Intn(3)],
	}, nil
}

func main() {

	if err := view.Register(octwirp.ServerLatencyView, octwirp.ServerResponseView); err != nil {
		log.Fatalf("failed to register metrics views: %v", err)
	}

	pe, err := prometheus.NewExporter(prometheus.Options{})
	if err != nil {
		log.Fatalf("failed to create prometheus metrics exporter: %v", err)
	}

	view.RegisterExporter(pe)
	view.SetReportingPeriod(time.Second)

	http.Handle("/metrics", pe)

	trace.RegisterExporter(logTraceExporter{})

	t := &octwirp.Tracer{
		StartOptions: trace.StartOptions{
			Sampler: trace.AlwaysSample(),
		},
	}

	server := example.NewHaberdasherServer(&randomHaberdasher{}, t.ServerHooks())
	handler := t.WrapHandler(server)
	http.Handle(server.PathPrefix(), handler)

	log.Fatal(http.ListenAndServe("127.0.0.1:8080", nil))
}

type logTraceExporter struct{}

func (l logTraceExporter) ExportSpan(d *trace.SpanData) {
	var attrs []string

	for k, v := range d.Attributes {
		val, ok := v.(string)
		if !ok {
			continue
		}
		attrs = append(attrs, k+"="+val)
	}
	sort.Strings(attrs)
	log.Printf("%s %s %s %s",
		d.Name, d.TraceID, d.SpanID, strings.Join(attrs, ","))
}
