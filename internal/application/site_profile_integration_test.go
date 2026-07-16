package application_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/fluxdm/fluxdm/internal/application"
	"github.com/fluxdm/fluxdm/internal/download"
	"github.com/fluxdm/fluxdm/internal/events"
	"github.com/fluxdm/fluxdm/internal/persistence"
	"github.com/fluxdm/fluxdm/internal/siteprofile"
)

type copyingProtector struct{}

func (copyingProtector) Protect(value []byte) ([]byte, error) {
	return append([]byte(nil), value...), nil
}
func (copyingProtector) Unprotect(value []byte) ([]byte, error) {
	return append([]byte(nil), value...), nil
}

func TestProtectedDownloadUsesSiteProfileBasicAuth(t *testing.T) {
	payload := []byte("protected payload")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		username, password, ok := request.BasicAuth()
		if !ok || username != "flux" || password != "secret" {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		writer.Header().Set("Content-Length", strconv.Itoa(len(payload)))
		if request.Method != http.MethodHead {
			_, _ = writer.Write(payload)
		}
	}))
	defer server.Close()
	parsed, _ := url.Parse(server.URL)
	ctx := context.Background()
	database, err := persistence.Open(ctx, filepath.Join(t.TempDir(), "protected.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	profiles := application.NewSiteProfileService(database.SiteProfiles(), copyingProtector{})
	saved, err := profiles.Save(ctx, application.SaveSiteProfileInput{Name: "Protected", HostPattern: parsed.Hostname(), AuthType: siteprofile.AuthBasic, Username: "flux", Password: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	bus := events.NewBus()
	service := application.NewDownloadService(ctx, database.Downloads(), download.NewProber(server.Client()), download.NewEngine(server.Client()), bus)
	service.SetRequestProfileResolver(profiles)
	defer service.Close()
	created, err := service.Create(ctx, application.CreateDownloadInput{URL: server.URL + "/file", DestinationDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if created.SiteProfileID != saved.ID {
		t.Fatalf("profile=%q", created.SiteProfileID)
	}
	if err := service.Start(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
	completed := waitForState(t, service, created.ID, "completed")
	if completed.DownloadedBytes != int64(len(payload)) {
		t.Fatalf("completed=%+v", completed)
	}
}

func TestProtectedDownloadUsesBearerCookiesAndCustomHeaders(t *testing.T) {
	payload := []byte("profile payload")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer token-value" ||
			request.Header.Get("Cookie") != "session=browser-cookie" ||
			request.Header.Get("X-API-Version") != "2" {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		writer.Header().Set("Content-Length", strconv.Itoa(len(payload)))
		if request.Method != http.MethodHead {
			_, _ = writer.Write(payload)
		}
	}))
	defer server.Close()
	parsed, _ := url.Parse(server.URL)
	ctx := context.Background()
	database, err := persistence.Open(ctx, filepath.Join(t.TempDir(), "protected-bearer.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	profiles := application.NewSiteProfileService(database.SiteProfiles(), copyingProtector{})
	_, err = profiles.Save(ctx, application.SaveSiteProfileInput{
		Name:        "API profile",
		HostPattern: parsed.Hostname(),
		AuthType:    siteprofile.AuthBearer,
		BearerToken: "token-value",
		Cookies:     "session=browser-cookie",
		Headers:     map[string]string{"X-API-Version": "2"},
	})
	if err != nil {
		t.Fatal(err)
	}
	service := application.NewDownloadService(ctx, database.Downloads(), download.NewProber(server.Client()), download.NewEngine(server.Client()), events.NewBus())
	service.SetRequestProfileResolver(profiles)
	defer service.Close()
	created, err := service.Create(ctx, application.CreateDownloadInput{URL: server.URL + "/file", DestinationDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Start(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
	completed := waitForState(t, service, created.ID, "completed")
	if completed.DownloadedBytes != int64(len(payload)) {
		t.Fatalf("completed=%+v", completed)
	}
}
