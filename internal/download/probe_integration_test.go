package download_test

import (
	"context"
	"testing"

	"github.com/fluxdm/fluxdm/internal/download"
	"github.com/fluxdm/fluxdm/tests/testserver"
)

func TestProbeDetectsMetadataAndVerifiedRanges(t *testing.T) {
	server := testserver.New()
	defer server.Close()
	result, err := download.NewProber(server.HTTP.Client()).Probe(context.Background(), server.URL("/file"))
	if err != nil {
		t.Fatal(err)
	}
	if result.FileName != "fixture.bin" || result.TotalBytes != int64(len(server.Payload)) || !result.RangeSupported {
		t.Fatalf("unexpected probe result: %+v", result)
	}
	if result.ETag != `"fixture-v1"` || result.MIMEType != "application/octet-stream" {
		t.Fatalf("missing metadata: %+v", result)
	}
}

func TestProbeFollowsRedirects(t *testing.T) {
	server := testserver.New()
	defer server.Close()
	result, err := download.NewProber(server.HTTP.Client()).Probe(context.Background(), server.URL("/redirect"))
	if err != nil {
		t.Fatal(err)
	}
	if result.FinalURL != server.URL("/file") {
		t.Fatalf("expected final URL %q, got %q", server.URL("/file"), result.FinalURL)
	}
}

func TestProbeFallsBackForUnknownLength(t *testing.T) {
	server := testserver.New()
	defer server.Close()
	result, err := download.NewProber(server.HTTP.Client()).Probe(context.Background(), server.URL("/unknown"))
	if err != nil {
		t.Fatal(err)
	}
	if result.TotalBytes != -1 || result.RangeSupported {
		t.Fatalf("unexpected unknown-length result: %+v", result)
	}
}
