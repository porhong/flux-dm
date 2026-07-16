package security

import (
	"fmt"
	"net/url"
	"strings"
)

const maxURLLength = 8192

func ValidateHTTPURL(raw string) (*url.URL, error) {
	if len(raw) == 0 || len(raw) > maxURLLength {
		return nil, fmt.Errorf("URL length must be between 1 and %d characters", maxURLLength)
	}
	parsed, err := url.ParseRequestURI(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("unsupported URL scheme %q", parsed.Scheme)
	}
	if parsed.Hostname() == "" || strings.ContainsAny(parsed.Host, "\r\n") {
		return nil, fmt.Errorf("URL must contain a valid host")
	}
	if parsed.User != nil {
		return nil, fmt.Errorf("credentials in URLs are not supported")
	}
	return parsed, nil
}
