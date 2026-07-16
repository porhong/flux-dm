package transport

import (
	"fmt"
	"net/http"
	"time"

	"github.com/fluxdm/fluxdm/internal/security"
)

func NewHTTPClient() *http.Client {
	clientTransport := &http.Transport{
		Proxy:                  http.ProxyFromEnvironment,
		MaxIdleConns:           64,
		MaxIdleConnsPerHost:    16,
		MaxConnsPerHost:        16,
		IdleConnTimeout:        90 * time.Second,
		TLSHandshakeTimeout:    15 * time.Second,
		ResponseHeaderTimeout:  30 * time.Second,
		ExpectContinueTimeout:  time.Second,
		ForceAttemptHTTP2:      true,
		MaxResponseHeaderBytes: 1 << 20,
	}
	return &http.Client{
		Transport: clientTransport,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			_, err := security.ValidateHTTPURL(request.URL.String())
			return err
		},
	}
}
