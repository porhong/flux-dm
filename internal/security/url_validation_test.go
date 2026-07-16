package security

import "testing"

func TestValidateHTTPURL(t *testing.T) {
	for _, raw := range []string{"https://example.test/file", "http://localhost:8080/a?b=c"} {
		if _, err := ValidateHTTPURL(raw); err != nil {
			t.Fatalf("expected %q to be valid: %v", raw, err)
		}
	}
}

func TestValidateHTTPURLRejectsUnsafeInput(t *testing.T) {
	for _, raw := range []string{"", "file:///tmp/a", "javascript:alert(1)", "https://user:pass@example.test/a", "https:///missing-host"} {
		if _, err := ValidateHTTPURL(raw); err == nil {
			t.Fatalf("expected %q to be rejected", raw)
		}
	}
}
