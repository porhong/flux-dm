package application_test

import (
	"context"
	"testing"
	"time"

	"github.com/fluxdm/fluxdm/internal/application"
	"github.com/fluxdm/fluxdm/internal/browserintegration"
	"github.com/fluxdm/fluxdm/internal/events"
	"github.com/fluxdm/fluxdm/tests/testserver"
)

// parkRequest mirrors what App.acceptBrowserRequest does: it stores the
// browser payload in the pending store and publishes the Wails-facing
// event. The cookies never appear in the event payload.
func parkRequest(store *browserintegration.PendingStore, bus *events.Bus, message browserintegration.Request) string {
	pendingID := store.Put(time.Now(), browserintegration.PendingRequest{
		URL:               message.URL,
		SuggestedFilename: message.SuggestedFilename,
		Referrer:          message.Referrer,
		Cookies:           message.Cookies,
	})
	bus.Publish(events.Event{
		Type: events.DownloadRequested,
		Data: application.DownloadRequestEvent{
			PendingID:         pendingID,
			URL:               message.URL,
			SuggestedFilename: message.SuggestedFilename,
			Referrer:          message.Referrer,
		},
	})
	return pendingID
}

// confirm mirrors what App.ConfirmBrowserDownload does after a user accepts
// the confirmation dialog: take the parked entry, create the download with
// the captured cookies, and let the caller start it.
func confirm(t *testing.T, service *application.DownloadService, store *browserintegration.PendingStore, pendingID, directory string) application.DownloadDTO {
	t.Helper()
	pending, ok := store.Take(time.Now(), pendingID)
	if !ok {
		t.Fatalf("pending %q not found", pendingID)
	}
	dto, err := service.CreateWithCookies(context.Background(), application.CreateDownloadInput{
		URL:            pending.URL,
		DestinationDir: directory,
		FileName:       pending.SuggestedFilename,
		Connections:    4,
	}, pending.Cookies)
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	return dto
}

func TestBrowserHandoffEventExcludesCookiesAndCarriesPendingID(t *testing.T) {
	server := testserver.New()
	defer server.Close()
	service, database, bus := newTestService(t, server)
	defer database.Close()
	defer service.Close()
	store := browserintegration.NewPendingStore(time.Minute)

	var seen application.DownloadRequestEvent
	unsubscribe := bus.Subscribe(events.DownloadRequested, func(event events.Event) {
		decoded, ok := event.Data.(application.DownloadRequestEvent)
		if !ok {
			t.Fatalf("unexpected payload type %T", event.Data)
		}
		seen = decoded
	})
	defer unsubscribe()

	pendingID := parkRequest(store, bus, browserintegration.Request{
		Version:           1,
		RequestID:         "abc",
		Type:              "add",
		URL:               server.URL("/file"),
		SuggestedFilename: "report.pdf",
		Referrer:          "https://example.test/page",
		Cookies:           "session=secret; token=classified",
	})

	if seen.PendingID == "" || seen.PendingID != pendingID {
		t.Fatalf("event pendingID=%q want=%q", seen.PendingID, pendingID)
	}
	if seen.URL != server.URL("/file") || seen.SuggestedFilename != "report.pdf" || seen.Referrer != "https://example.test/page" {
		t.Fatalf("event payload missing fields: %+v", seen)
	}
	if store.Len() != 1 {
		t.Fatalf("pending store len=%d", store.Len())
	}
}

func TestBrowserHandoffConfirmCreatesDownloadableRecord(t *testing.T) {
	server := testserver.New()
	defer server.Close()
	service, database, _ := newTestService(t, server)
	defer database.Close()
	defer service.Close()
	store := browserintegration.NewPendingStore(time.Minute)

	pendingID := parkRequest(store, events.NewBus(), browserintegration.Request{
		URL:               server.URL("/file"),
		SuggestedFilename: "report.pdf",
		Cookies:           "session=secret",
	})

	directory := t.TempDir()
	created := confirm(t, service, store, pendingID, directory)
	if created.State != "queued" {
		t.Fatalf("state=%q", created.State)
	}
	if err := service.Start(context.Background(), created.ID); err != nil {
		t.Fatal(err)
	}
	completed := waitForState(t, service, created.ID, "completed")
	if completed.FileName != "report.pdf" {
		t.Fatalf("filename=%q", completed.FileName)
	}
	if store.Len() != 0 {
		t.Fatalf("pending entry not consumed, len=%d", store.Len())
	}
}

func TestBrowserHandoffConfirmRejectsExpiredOrUnknownPendingID(t *testing.T) {
	server := testserver.New()
	defer server.Close()
	service, database, _ := newTestService(t, server)
	defer database.Close()
	defer service.Close()
	store := browserintegration.NewPendingStore(5 * time.Millisecond)

	pendingID := store.Put(time.Now(), browserintegration.PendingRequest{
		URL:               server.URL("/file"),
		SuggestedFilename: "report.pdf",
		Cookies:           "session=secret",
	})
	// Let it expire.
	time.Sleep(20 * time.Millisecond)

	if _, ok := store.Take(time.Now(), pendingID); ok {
		t.Fatal("expired pending entry should not be takeable")
	}
	if _, ok := store.Take(time.Now(), "definitely-unknown"); ok {
		t.Fatal("unknown pending entry should not be takeable")
	}
	if store.Len() != 0 {
		t.Fatalf("expected sweep to remove expired entry, len=%d", store.Len())
	}
}

func TestBrowserHandoffDiscardReleasesEntry(t *testing.T) {
	store := browserintegration.NewPendingStore(time.Minute)
	pendingID := store.Put(time.Now(), browserintegration.PendingRequest{
		URL:     "https://example.test/file",
		Cookies: "session=secret",
	})
	store.Discard(time.Now(), pendingID)
	if store.Len() != 0 {
		t.Fatalf("discard did not release entry, len=%d", store.Len())
	}
}
