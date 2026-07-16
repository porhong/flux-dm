package download

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

func clientForOptions(base *http.Client, options RequestOptions) (*http.Client, error) {
	cloned := *base
	if options.ProxyURL != "" {
		proxyURL, err := url.Parse(options.ProxyURL)
		if err != nil || proxyURL.Hostname() == "" || (proxyURL.Scheme != "http" && proxyURL.Scheme != "https") {
			return nil, fmt.Errorf("invalid proxy URL")
		}
		if options.ProxyUsername != "" {
			proxyURL.User = url.UserPassword(options.ProxyUsername, options.ProxyPassword)
		}
		transport, ok := base.Transport.(*http.Transport)
		if !ok {
			return nil, fmt.Errorf("proxy requires HTTP transport")
		}
		clonedTransport := transport.Clone()
		clonedTransport.Proxy = http.ProxyURL(proxyURL)
		cloned.Transport = clonedTransport
	}
	baseRedirect := base.CheckRedirect
	cloned.CheckRedirect = func(request *http.Request, via []*http.Request) error {
		if baseRedirect != nil {
			if err := baseRedirect(request, via); err != nil {
				return err
			}
		}
		if baseRedirect == nil && len(via) >= 10 {
			return fmt.Errorf("too many redirects")
		}
		if len(via) > 0 && !strings.EqualFold(request.URL.Host, via[0].URL.Host) {
			for name := range options.Headers {
				request.Header.Del(name)
			}
		}
		return nil
	}
	return &cloned, nil
}
func applyRequestOptions(request *http.Request, options RequestOptions) {
	for name, value := range options.Headers {
		request.Header.Set(name, value)
	}
}
