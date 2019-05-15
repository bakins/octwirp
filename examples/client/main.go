// based on https://github.com/twitchtv/twirp/blob/337e90237d72193bf7f9fa387b5b9946436b7733/example/cmd/client/main.go
// Copyright 2018 Twitch Interactive, Inc.  All Rights Reserved.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/twitchtv/twirp"
	"github.com/twitchtv/twirp/example"
	"go.opencensus.io/plugin/ochttp"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/trace"

	"github.com/bakins/octwirp"
)

func main() {

	trace.RegisterExporter(logTraceExporter{})
	view.RegisterExporter(logMetricsExporter{})
	view.SetReportingPeriod(time.Second)

	if err := view.Register(octwirp.ClientRoundtripLatency); err != nil {
		log.Fatalf("failed to register metrics views: %v", err)
	}

	t := ochttp.Transport{
		StartOptions: trace.StartOptions{
			Sampler: trace.AlwaysSample(),
		},
	}

	c := http.Client{
		Transport: octwirp.WrapTransport(&t),
	}

	client := example.NewHaberdasherJSONClient("http://localhost:8080", &c)

	var (
		hat *example.Hat
		err error
	)
	for i := 0; i < 5; i++ {
		hat, err = client.MakeHat(context.Background(), &example.Size{Inches: 12})
		if err != nil {
			if twerr, ok := err.(twirp.Error); ok {
				if twerr.Meta("retryable") != "" {
					// Log the error and go again.
					log.Printf("got error %q, retrying", twerr)
					continue
				}
			}
			// This was some fatal error!
			log.Fatal(err)
		}
	}
	fmt.Printf("%+v", hat)

	time.Sleep(5 * time.Second)
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

type logMetricsExporter struct{}

func (l logMetricsExporter) ExportView(v *view.Data) {
	log.Println(v.View.Name)
	for _, r := range v.Rows {
		var tags []string
		for _, t := range r.Tags {
			tags = append(tags, t.Key.Name()+"="+t.Value)
		}
		sort.Strings(tags)
		log.Printf("  %s", strings.Join(tags, ","))
	}
}
