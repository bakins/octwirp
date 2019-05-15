package octwirp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/twitchtv/twirp"
	"github.com/twitchtv/twirp/example"
	"go.opencensus.io/plugin/ochttp"
	"go.opencensus.io/trace"
)

func TestServer(t *testing.T) {
	tests := []struct {
		name    string
		status  int64
		hatSize int32
	}{
		{
			name:    "simple 200",
			status:  200,
			hatSize: 10,
		},
		{
			name:    "invalid size",
			status:  400,
			hatSize: 0,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			// exporters are global :(
			exp := testTraceExporter{}
			trace.RegisterExporter(&exp)
			defer trace.UnregisterExporter(&exp)

			defer func() { exp.spans = nil }()

			tr := &Tracer{
				StartOptions: trace.StartOptions{
					Sampler: trace.AlwaysSample(),
				},
			}
			server := example.NewHaberdasherServer(&testHaberdasher{}, tr.ServerHooks())
			handler := tr.WrapHandler(server)
			svr := httptest.NewServer(handler)
			defer svr.Close()

			client := example.NewHaberdasherJSONClient(svr.URL, &http.Client{})

			_, err := client.MakeHat(context.Background(), &example.Size{Inches: test.hatSize})

			// the twirp span is a sub-span of the normal http span
			require.Len(t, exp.spans, 2)
			require.Equal(t, test.status, exp.spans[1].Attributes[ochttp.StatusCodeAttribute])
			if test.status != 200 {
				require.Error(t, err)
			}
		})
	}
}

func TestClient(t *testing.T) {
	tests := []struct {
		name    string
		status  int64
		hatSize int32
	}{
		{
			name:    "simple 200",
			status:  200,
			hatSize: 10,
		},
		{
			name:    "invalid size",
			status:  400,
			hatSize: 0,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			// exporters are global :(
			exp := testTraceExporter{}
			trace.RegisterExporter(&exp)
			defer trace.UnregisterExporter(&exp)

			defer func() { exp.spans = nil }()

			server := example.NewHaberdasherServer(&testHaberdasher{}, nil)
			svr := httptest.NewServer(server)
			defer svr.Close()

			o := ochttp.Transport{
				StartOptions: trace.StartOptions{
					Sampler: trace.AlwaysSample(),
				},
			}

			c := http.Client{
				Transport: WrapTransport(&o),
			}

			client := example.NewHaberdasherJSONClient(svr.URL, &c)

			_, err := client.MakeHat(context.Background(), &example.Size{Inches: test.hatSize})

			// the twirp span is a sub-span of the normal http span
			require.Len(t, exp.spans, 2)
			require.Equal(t, test.status, exp.spans[1].Attributes[ochttp.StatusCodeAttribute])
			if test.status != 200 {
				require.Error(t, err)
			}
		})
	}
}

type testHaberdasher struct{}

func (h *testHaberdasher) MakeHat(ctx context.Context, size *example.Size) (*example.Hat, error) {
	if size.Inches <= 0 {
		return nil, twirp.InvalidArgumentError("Inches", "I can't make a hat that small!")
	}
	return &example.Hat{
		Size:  size.Inches,
		Color: "black",
		Name:  "derby",
	}, nil
}

type testTraceExporter struct {
	spans []trace.SpanData
}

func (t *testTraceExporter) ExportSpan(s *trace.SpanData) {
	t.spans = append(t.spans, *s)
}
