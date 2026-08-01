package application

import (
	"context"
	"crypto/ed25519"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fluxdm/fluxdm/internal/update"
)

type updateServiceRepository struct{ state update.StoredState }

func (r *updateServiceRepository) Load(context.Context) (update.StoredState, error) {
	return r.state, nil
}
func (r *updateServiceRepository) Save(_ context.Context, state update.StoredState) error {
	r.state = state
	return nil
}

type updateServiceVerifier struct{}

func (updateServiceVerifier) VerifyProductionInstaller(string) error { return nil }

type updateServiceLauncher struct{}

func (updateServiceLauncher) Launch(context.Context, string, update.Handoff) error { return nil }

func TestUpdateServiceReturnsSafeCheckError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		http.Error(response, "upstream unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	manager, err := update.NewManager(update.Config{
		Repository:       "test/repository",
		ReleaseAPIURL:    server.URL,
		CacheDir:         t.TempDir(),
		CurrentVersion:   "1.0.0",
		StablePublicKey:  make(ed25519.PublicKey, ed25519.PublicKeySize),
		PreviewPublicKey: make(ed25519.PublicKey, ed25519.PublicKeySize),
		HTTPClient:       server.Client(),
		Verifier:         updateServiceVerifier{},
		Launcher:         updateServiceLauncher{},
	}, &updateServiceRepository{})
	if err != nil {
		t.Fatal(err)
	}

	_, err = NewUpdateService(manager).Check(context.Background())
	var applicationError *Error
	if !errors.As(err, &applicationError) {
		t.Fatalf("error type = %T, want *Error", err)
	}
	if applicationError.Code != ErrInternal || applicationError.Message != "Could not check for application updates. Check your connection and try again." {
		t.Fatalf("unexpected application error: %#v", applicationError)
	}
	if applicationError.Cause == nil {
		t.Fatal("safe update error must preserve the backend cause")
	}
}
