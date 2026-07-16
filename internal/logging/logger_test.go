package logging

import (
	"bytes"
	"strings"
	"testing"
)

func TestLoggerRedactsSensitiveFieldsAndSignedQueries(t *testing.T) {
	var output bytes.Buffer
	logger := NewForWriter(&output)
	logger.Info("request token=abc123&file=x", map[string]any{
		"authorization": "Bearer secret",
		"host":          "example.test",
	})

	text := output.String()
	for _, secret := range []string{"abc123", "Bearer secret"} {
		if strings.Contains(text, secret) {
			t.Fatalf("log leaked %q: %s", secret, text)
		}
	}
	if !strings.Contains(text, "example.test") {
		t.Fatalf("expected safe field in log: %s", text)
	}
}
