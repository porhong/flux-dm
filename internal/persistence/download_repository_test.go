package persistence

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/fluxdm/fluxdm/internal/download"
)

func TestDownloadRepositoryRoundTrip(t *testing.T) {
	ctx := context.Background()
	database, err := Open(ctx, filepath.Join(t.TempDir(), "repository.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	repository := database.Downloads()
	now := time.Now().UTC().Truncate(time.Microsecond)
	task := download.Download{
		ID: "download-1", URL: "https://example.test/file", FileName: "file.bin",
		DestinationPath: `C:\Downloads\file.bin`, TempPath: `C:\Downloads\file.bin.fluxpart`,
		State: download.StateQueued, TotalBytes: 16, CreatedAt: now, Connections: 4, BandwidthLimit: 1024,
		CategoryID: "category-a", QueueID: "queue-a", QueuePosition: 99, Priority: 7,
	}
	task.Segments, err = download.PlanSegments(task.ID, task.TempPath, task.TotalBytes, task.Connections)
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.Create(ctx, task); err != nil {
		t.Fatal(err)
	}
	actual, err := repository.Get(ctx, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if actual.ID != task.ID || actual.State != download.StateQueued || !actual.CreatedAt.Equal(now) || actual.Connections != 4 || actual.BandwidthLimit != 1024 || actual.CategoryID != "category-a" || actual.QueueID != "queue-a" || actual.QueuePosition != 99 || actual.Priority != 7 || len(actual.Segments) != 4 {
		t.Fatalf("unexpected round trip: %+v", actual)
	}
	if err := actual.Transition(download.StateProbing); err != nil {
		t.Fatal(err)
	}
	if err := repository.Save(ctx, actual); err != nil {
		t.Fatal(err)
	}
	listed, err := repository.List(ctx)
	if err != nil || len(listed) != 1 || listed[0].State != download.StateProbing {
		t.Fatalf("unexpected list: %+v, %v", listed, err)
	}
}

func TestDownloadRepositoryNotFound(t *testing.T) {
	ctx := context.Background()
	database, err := Open(ctx, filepath.Join(t.TempDir(), "repository.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if _, err := database.Downloads().Get(ctx, "missing"); !errors.Is(err, download.ErrNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestDownloadRepositoryDelete(t *testing.T) {
	ctx := context.Background()
	database, err := Open(ctx, filepath.Join(t.TempDir(), "repository.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	repository := database.Downloads()
	task := download.Download{
		ID: "download-1", URL: "https://example.test/file", FileName: "file.bin",
		DestinationPath: filepath.Join(t.TempDir(), "file.bin"), TempPath: filepath.Join(t.TempDir(), "file.bin.fluxpart"),
		State: download.StateCompleted, TotalBytes: 16, DownloadedBytes: 16, CreatedAt: time.Now().UTC(), Connections: 1,
	}
	if err := repository.Create(ctx, task); err != nil {
		t.Fatal(err)
	}
	if err := repository.Delete(ctx, task.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Get(ctx, task.ID); !errors.Is(err, download.ErrNotFound) {
		t.Fatalf("expected deleted download to be absent, got %v", err)
	}
}
