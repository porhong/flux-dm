package download

import (
	"context"
	"github.com/fluxdm/fluxdm/internal/transport"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestProbeUsesAuthenticatedProxy(t *testing.T) {
	var authenticated atomic.Bool
	proxy := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		username, password, ok := request.BasicAuth()
		_ = username
		_ = password
		if request.Header.Get("Proxy-Authorization") == "Basic cHJveHk6c2VjcmV0" {
			authenticated.Store(true)
		}
		if !ok {
			writer.Header().Set("Content-Length", "4")
		}
		if request.Method != http.MethodHead {
			_, _ = writer.Write([]byte("data"))
		}
	}))
	defer proxy.Close()
	prober := NewProber(transport.NewHTTPClient())
	result, err := prober.ProbeWithOptions(context.Background(), "http://origin.invalid/file", RequestOptions{ProxyURL: proxy.URL, ProxyUsername: "proxy", ProxyPassword: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	if !authenticated.Load() || result.TotalBytes != 4 {
		t.Fatalf("authenticated=%v result=%+v", authenticated.Load(), result)
	}
}
