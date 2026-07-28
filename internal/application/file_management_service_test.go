package application_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fluxdm/fluxdm/internal/application"
	"github.com/fluxdm/fluxdm/internal/download"
	fluxfs "github.com/fluxdm/fluxdm/internal/filesystem"
	"github.com/fluxdm/fluxdm/internal/persistence"
)

func TestFileManagementMovesCompletedDownloadsAndSkipsTransfers(t *testing.T) {
	ctx := context.Background()
	database, err := persistence.Open(ctx, filepath.Join(t.TempDir(), "files.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	sourceDirectory := t.TempDir()
	targetDirectory := t.TempDir()
	completed := createCompletedFileDownload(t, database.Downloads(), sourceDirectory, "report.txt", download.StateCompleted)
	queued := createCompletedFileDownload(t, database.Downloads(), sourceDirectory, "pending.txt", download.StateQueued)
	service := application.NewFileManagementService(database.Downloads(), fluxfs.NewCompletedFileManager(nil), nil)

	result, err := service.Move(ctx, application.MoveCompletedDownloadsInput{DownloadIDs: []string{completed.ID, queued.ID}, DestinationDir: targetDirectory})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Updated) != 1 || result.Updated[0].ID != completed.ID || len(result.SkippedIDs) != 1 || result.SkippedIDs[0] != queued.ID || len(result.Failures) != 0 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if _, err := os.Stat(completed.DestinationPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("source file still exists: %v", err)
	}
	movedPath := filepath.Join(targetDirectory, "report.txt")
	if data, err := os.ReadFile(movedPath); err != nil || string(data) != "completed data" {
		t.Fatalf("moved file mismatch: %q, %v", data, err)
	}
	persisted, err := database.Downloads().Get(ctx, completed.ID)
	if err != nil || persisted.DestinationPath != movedPath || persisted.FileName != "report.txt" {
		t.Fatalf("unexpected persisted task: %+v, %v", persisted, err)
	}
}

func TestFileManagementRenamesWithCollisionAndRemovesHistory(t *testing.T) {
	ctx := context.Background()
	database, err := persistence.Open(ctx, filepath.Join(t.TempDir(), "files.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "archive.txt"), []byte("existing"), 0o600); err != nil {
		t.Fatal(err)
	}
	task := createCompletedFileDownload(t, database.Downloads(), directory, "report.txt", download.StateCompleted)
	service := application.NewFileManagementService(database.Downloads(), fluxfs.NewCompletedFileManager(nil), nil)

	renamed, err := service.Rename(ctx, task.ID, "archive.txt")
	if err != nil {
		t.Fatal(err)
	}
	if renamed.FileName != "archive (1).txt" {
		t.Fatalf("unexpected collision name: %q", renamed.FileName)
	}
	removed, err := service.RemoveHistory(ctx, []string{task.ID})
	if err != nil || len(removed.RemovedIDs) != 1 || removed.RemovedIDs[0] != task.ID {
		t.Fatalf("unexpected removal: %+v, %v", removed, err)
	}
	if _, err := database.Downloads().Get(ctx, task.ID); !errors.Is(err, download.ErrNotFound) {
		t.Fatalf("expected history deletion, got %v", err)
	}
	if _, err := os.Stat(renamed.DestinationPath); err != nil {
		t.Fatalf("history-only removal deleted the file: %v", err)
	}
}

func TestFileManagementRejectsNonCompletedDownloads(t *testing.T) {
	ctx := context.Background()
	database, err := persistence.Open(ctx, filepath.Join(t.TempDir(), "files.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	task := createCompletedFileDownload(t, database.Downloads(), t.TempDir(), "queued.txt", download.StateQueued)
	service := application.NewFileManagementService(database.Downloads(), fluxfs.NewCompletedFileManager(nil), nil)
	if err := service.Open(ctx, task.ID); err == nil {
		t.Fatal("expected non-completed file action to fail")
	}
}

func createCompletedFileDownload(t *testing.T, repository download.Repository, directory, name string, state download.State) download.Download {
	t.Helper()
	path := filepath.Join(directory, name)
	if err := os.WriteFile(path, []byte("completed data"), 0o600); err != nil {
		t.Fatal(err)
	}
	task := download.Download{ID: "file-" + name, URL: "https://example.test/" + name, FileName: name, DestinationPath: path, TempPath: path + ".fluxpart", State: state, TotalBytes: int64(len("completed data")), DownloadedBytes: int64(len("completed data")), CreatedAt: time.Now().UTC(), Connections: 1}
	if err := repository.Create(context.Background(), task); err != nil {
		t.Fatal(err)
	}
	return task
}
