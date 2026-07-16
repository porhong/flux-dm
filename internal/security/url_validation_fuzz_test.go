package security

import "testing"

func FuzzValidateHTTPURL(f *testing.F) {
	for _, seed := range []string{"https://example.test/file", "file:///etc/passwd", "https://user:pass@example.test/", "https://example.test/%0d%0a"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, value string) {
		parsed, err := ValidateHTTPURL(value)
		if err == nil && (parsed.Hostname() == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil) {
			t.Fatalf("unsafe URL accepted: %q", value)
		}
	})
}
