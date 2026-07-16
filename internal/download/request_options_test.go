package download

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestProfileHeadersAreRemovedAcrossHostRedirect(t *testing.T) {
	var leaked atomic.Bool
	target := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("X-Secret") != "" {
			leaked.Store(true)
		}
		writer.Header().Set("Content-Length", "1")
		if request.Method != http.MethodHead {
			_, _ = writer.Write([]byte("x"))
		}
	}))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, target.URL+"/file", http.StatusFound)
	}))
	defer source.Close()
	prober := NewProber(source.Client())
	if _, err := prober.ProbeWithOptions(context.Background(), source.URL+"/start", RequestOptions{Headers: map[string]string{"X-Secret": "value"}}); err != nil {
		t.Fatal(err)
	}
	if leaked.Load() {
		t.Fatal("custom header leaked to redirected host")
	}
}
