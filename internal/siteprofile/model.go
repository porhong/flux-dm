package siteprofile

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
)

type AuthType string

const (
	AuthNone   AuthType = "none"
	AuthBasic  AuthType = "basic"
	AuthBearer AuthType = "bearer"
)

type Profile struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	HostPattern    string    `json:"hostPattern"`
	AuthType       AuthType  `json:"authType"`
	ProxyURL       string    `json:"proxyUrl"`
	HasCredentials bool      `json:"hasCredentials"`
	HasCookies     bool      `json:"hasCookies"`
	HeaderNames    []string  `json:"headerNames"`
	CreatedAt      time.Time `json:"createdAt"`
}
type SecretPayload struct {
	Username      string            `json:"username,omitempty"`
	Password      string            `json:"password,omitempty"`
	BearerToken   string            `json:"bearerToken,omitempty"`
	Cookies       string            `json:"cookies,omitempty"`
	Headers       map[string]string `json:"headers,omitempty"`
	ProxyUsername string            `json:"proxyUsername,omitempty"`
	ProxyPassword string            `json:"proxyPassword,omitempty"`
}
type Record struct {
	Profile          Profile
	EncryptedSecrets []byte
}
type Repository interface {
	List(context.Context) ([]Record, error)
	Get(context.Context, string) (Record, error)
	Save(context.Context, Record) error
	Delete(context.Context, string) error
	SaveDownloadSecret(context.Context, string, []byte) error
	GetDownloadSecret(context.Context, string) ([]byte, error)
	DeleteDownloadSecret(context.Context, string) error
}

var headerNamePattern = regexp.MustCompile(`^[!#$%&'*+.^_` + "`" + `|~0-9A-Za-z-]+$`)
var hostPattern = regexp.MustCompile(`^(\*\.)?[a-z0-9](?:[a-z0-9.-]{0,251}[a-z0-9])?$`)

func ValidateProfile(profile Profile, secrets SecretPayload) error {
	if strings.TrimSpace(profile.Name) == "" || len(profile.Name) > 100 {
		return fmt.Errorf("invalid profile name")
	}
	profile.HostPattern = strings.ToLower(strings.TrimSpace(profile.HostPattern))
	if !hostPattern.MatchString(profile.HostPattern) || strings.Contains(profile.HostPattern, "..") {
		return fmt.Errorf("invalid host pattern")
	}
	switch profile.AuthType {
	case AuthNone:
	case AuthBasic:
		if secrets.Username == "" {
			return fmt.Errorf("basic username required")
		}
	case AuthBearer:
		if secrets.BearerToken == "" {
			return fmt.Errorf("bearer token required")
		}
	default:
		return fmt.Errorf("invalid auth type")
	}
	if profile.ProxyURL != "" {
		parsed, err := url.Parse(profile.ProxyURL)
		if err != nil || parsed.Hostname() == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil {
			return fmt.Errorf("invalid proxy URL")
		}
	}
	if len(secrets.Cookies) > 32768 || strings.ContainsAny(secrets.Cookies, "\r\n") {
		return fmt.Errorf("invalid cookies")
	}
	if len(secrets.Headers) > 50 {
		return fmt.Errorf("too many headers")
	}
	for name, value := range secrets.Headers {
		canonical := http.CanonicalHeaderKey(strings.TrimSpace(name))
		if !headerNamePattern.MatchString(canonical) || reservedHeader(canonical) || len(value) > 8192 || strings.ContainsAny(value, "\r\n") {
			return fmt.Errorf("invalid custom header")
		}
	}
	return nil
}
func Match(rawURL string, records []Record) *Record {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil
	}
	host := strings.ToLower(parsed.Hostname())
	matches := make([]Record, 0)
	for _, record := range records {
		pattern := strings.ToLower(record.Profile.HostPattern)
		if pattern == host || (strings.HasPrefix(pattern, "*.") && (host == strings.TrimPrefix(pattern, "*.") || strings.HasSuffix(host, "."+strings.TrimPrefix(pattern, "*.")))) {
			matches = append(matches, record)
		}
	}
	if len(matches) == 0 {
		return nil
	}
	sort.Slice(matches, func(i, j int) bool {
		left, right := matches[i].Profile.HostPattern, matches[j].Profile.HostPattern
		if len(left) == len(right) {
			return matches[i].Profile.ID < matches[j].Profile.ID
		}
		return len(left) > len(right)
	})
	return &matches[0]
}
func reservedHeader(name string) bool {
	switch strings.ToLower(name) {
	case "host", "content-length", "range", "if-range", "proxy-authorization", "connection", "transfer-encoding", "cookie", "authorization":
		return true
	default:
		return false
	}
}
