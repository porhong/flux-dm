package main

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/fluxdm/fluxdm/internal/application"
	"github.com/fluxdm/fluxdm/internal/browserintegration"
	"github.com/fluxdm/fluxdm/internal/download"
	"github.com/fluxdm/fluxdm/internal/events"
	"github.com/fluxdm/fluxdm/internal/persistence"
	"github.com/fluxdm/fluxdm/internal/transport"
)

func TestListPendingBrowserDownloadsReturnsMetadataWithoutConsumingRequest(t *testing.T) {
	app := NewApp(application.Paths{}, nil)
	now := time.Now()
	pendingID := app.pending.Put(now, browserintegration.PendingRequest{
		URL:               "https://example.test/archive.zip",
		SuggestedFilename: "archive.zip",
		Referrer:          "https://example.test/page",
		Cookies:           "session=secret",
	})

	requests, err := app.ListPendingBrowserDownloads()
	if err != nil {
		t.Fatal(err)
	}
	if len(requests) != 1 {
		t.Fatalf("request count=%d want=1", len(requests))
	}
	got := requests[0]
	if got.PendingID != pendingID || got.URL != "https://example.test/archive.zip" || got.SuggestedFilename != "archive.zip" || got.Referrer != "https://example.test/page" {
		t.Fatalf("unexpected request: %+v", got)
	}
	if _, ok := app.pending.Take(time.Now(), pendingID); !ok {
		t.Fatal("listing pending browser downloads should not consume the request")
	}
}

func TestConfirmBrowserDownloadRetainsRequestAfterDestinationValidationError(t *testing.T) {
	ctx := context.Background()
	database, _, err := persistence.OpenRecovering(ctx, filepath.Join(t.TempDir(), "fluxdm.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	service := application.NewDownloadService(
		ctx,
		database.Downloads(),
		download.NewProber(transport.NewHTTPClient()),
		download.NewEngine(transport.NewHTTPClient()),
		events.NewBus(),
	)
	defer service.Close()

	app := NewApp(application.Paths{}, nil)
	app.ctx = ctx
	app.downloads = service
	pendingID := app.pending.Put(time.Now(), browserintegration.PendingRequest{
		URL:               "https://example.test/archive.zip",
		SuggestedFilename: "archive.zip",
		Cookies:           "session=secret",
	})

	if _, err := app.ConfirmBrowserDownload(pendingID, filepath.Join(t.TempDir(), "missing"), "archive.zip", 4, false); err == nil {
		t.Fatal("expected invalid destination error")
	}
	if requests, err := app.ListPendingBrowserDownloads(); err != nil || len(requests) != 1 || requests[0].PendingID != pendingID {
		t.Fatalf("pending request was not retained after validation error: requests=%+v err=%v", requests, err)
	}

	created, err := app.ConfirmBrowserDownload(pendingID, t.TempDir(), "archive.zip", 4, false)
	if err != nil {
		t.Fatal(err)
	}
	if created.State != "queued" {
		t.Fatalf("state=%q want queued", created.State)
	}
	if requests, err := app.ListPendingBrowserDownloads(); err != nil || len(requests) != 0 {
		t.Fatalf("confirmed request was not consumed: requests=%+v err=%v", requests, err)
	}
}
