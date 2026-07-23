package application_test

import (
	"context"
	"crypto/sha256"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fluxdm/fluxdm/internal/application"
	"github.com/fluxdm/fluxdm/internal/download"
	"github.com/fluxdm/fluxdm/internal/events"
	"github.com/fluxdm/fluxdm/internal/persistence"
	"github.com/fluxdm/fluxdm/tests/testserver"
)

func TestDownloadServiceCompletesAndPersistsDownload(t *testing.T) {
	server := testserver.New()
	defer server.Close()
	service, database, serviceBus := newTestService(t, server)
	defer database.Close()
	defer service.Close()

	var progressEvents atomic.Int32
	unsubscribe := serviceBus.Subscribe(events.DownloadProgress, func(events.Event) { progressEvents.Add(1) })
	defer unsubscribe()

	directory := t.TempDir()
	created, err := service.Create(context.Background(), application.CreateDownloadInput{
		URL: server.URL("/redirect"), DestinationDir: directory,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Start(context.Background(), created.ID); err != nil {
		t.Fatal(err)
	}
	completed := waitForState(t, service, created.ID, "completed")
	if completed.DownloadedBytes != int64(len(server.Payload)) || completed.Connections != 4 || completed.SegmentCount != 4 || progressEvents.Load() == 0 {
		t.Fatalf("unexpected completion: %+v, progress events: %d", completed, progressEvents.Load())
	}
	actual, err := os.ReadFile(completed.DestinationPath)
	if err != nil || string(actual) != string(server.Payload) {
		t.Fatalf("invalid completed file: %v", err)
	}
	listed, err := service.List(context.Background())
	if err != nil || len(listed) != 1 || listed[0].State != "completed" {
		t.Fatalf("download was not persisted: %+v, %v", listed, err)
	}
}

func TestDownloadServiceRemovesCompletedRecordAndKeepsFile(t *testing.T) {
	server := testserver.New()
	defer server.Close()
	service, database, _ := newTestService(t, server)
	defer database.Close()
	defer service.Close()

	created, err := service.Create(context.Background(), application.CreateDownloadInput{URL: server.URL("/file"), DestinationDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Start(context.Background(), created.ID); err != nil {
		t.Fatal(err)
	}
	completed := waitForState(t, service, created.ID, "completed")
	if err := service.RemoveRecord(context.Background(), completed.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(completed.DestinationPath); err != nil {
		t.Fatalf("record removal deleted the completed file: %v", err)
	}
	if _, err := service.Get(context.Background(), completed.ID); err == nil {
		t.Fatal("removed download record was still available")
	}
}

func TestDownloadServiceDeletesCompletedFileAndRecord(t *testing.T) {
	server := testserver.New()
	defer server.Close()
	service, database, _ := newTestService(t, server)
	defer database.Close()
	defer service.Close()

	created, err := service.Create(context.Background(), application.CreateDownloadInput{URL: server.URL("/file"), DestinationDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Start(context.Background(), created.ID); err != nil {
		t.Fatal(err)
	}
	completed := waitForState(t, service, created.ID, "completed")
	if err := service.DeleteCompletedFile(context.Background(), completed.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(completed.DestinationPath); !os.IsNotExist(err) {
		t.Fatalf("completed file was not deleted: %v", err)
	}
	if _, err := service.Get(context.Background(), completed.ID); err == nil {
		t.Fatal("deleted download record was still available")
	}
}

func TestDownloadServiceKeepsRecordWhenCompletedFileIsMissing(t *testing.T) {
	server := testserver.New()
	defer server.Close()
	service, database, _ := newTestService(t, server)
	defer database.Close()
	defer service.Close()

	created, err := service.Create(context.Background(), application.CreateDownloadInput{URL: server.URL("/file"), DestinationDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Start(context.Background(), created.ID); err != nil {
		t.Fatal(err)
	}
	completed := waitForState(t, service, created.ID, "completed")
	if err := os.Remove(completed.DestinationPath); err != nil {
		t.Fatal(err)
	}
	if err := service.DeleteCompletedFile(context.Background(), completed.ID); err == nil {
		t.Fatal("expected missing completed file to be rejected")
	}
	if _, err := service.Get(context.Background(), completed.ID); err != nil {
		t.Fatalf("record should remain after failed file deletion: %v", err)
	}
}

func TestDownloadServiceRejectsUnsupportedConnectionCount(t *testing.T) {
	server := testserver.New()
	defer server.Close()
	service, database, _ := newTestService(t, server)
	defer database.Close()
	defer service.Close()
	directory := t.TempDir()
	if _, err := service.Create(context.Background(), application.CreateDownloadInput{URL: server.URL("/file"), DestinationDir: directory, Connections: 3}); err == nil {
		t.Fatal("expected unsupported connection count to be rejected")
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("invalid request reserved %d files", len(entries))
	}
}

func TestDownloadServicePersistsBandwidthLimitConfiguration(t *testing.T) {
	server := testserver.New()
	defer server.Close()
	service, database, _ := newTestService(t, server)
	defer database.Close()
	defer service.Close()
	created, err := service.Create(context.Background(), application.CreateDownloadInput{
		URL: server.URL("/file"), DestinationDir: t.TempDir(), BandwidthLimit: 1024,
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.BandwidthLimit != 1024 {
		t.Fatalf("created limit = %d", created.BandwidthLimit)
	}
	if err := service.SetDownloadBandwidthLimit(context.Background(), created.ID, 2048); err != nil {
		t.Fatal(err)
	}
	configured, err := service.Get(context.Background(), created.ID)
	if err != nil || configured.BandwidthLimit != 2048 {
		t.Fatalf("configured download = %+v, %v", configured, err)
	}
	if err := service.SetGlobalBandwidthLimit(-1); err == nil {
		t.Fatal("expected negative global limit to fail")
	}
}

func TestDownloadServiceCancelsActiveDownload(t *testing.T) {
	server := testserver.New()
	defer server.Close()
	service, database, _ := newTestService(t, server)
	defer database.Close()
	defer service.Close()

	created, err := service.Create(context.Background(), application.CreateDownloadInput{
		URL: server.URL("/slow"), DestinationDir: t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Start(context.Background(), created.ID); err != nil {
		t.Fatal(err)
	}
	waitForState(t, service, created.ID, "downloading")
	if err := service.Cancel(context.Background(), created.ID); err != nil {
		t.Fatal(err)
	}
	cancelled := waitForState(t, service, created.ID, "cancelled")
	if _, err := os.Stat(cancelled.DestinationPath); !os.IsNotExist(err) {
		t.Fatalf("cancelled download created final file: %v", err)
	}
}

func TestDownloadServicePausesAndResumes(t *testing.T) {
	server := testserver.New()
	defer server.Close()
	service, database, _ := newTestService(t, server)
	defer database.Close()
	defer service.Close()

	created, err := service.Create(context.Background(), application.CreateDownloadInput{URL: server.URL("/slow"), DestinationDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Start(context.Background(), created.ID); err != nil {
		t.Fatal(err)
	}
	waitForState(t, service, created.ID, "downloading")
	waitForFileGrowth(t, created.TempPath, 0)
	if err := service.Pause(context.Background(), created.ID); err != nil {
		t.Fatal(err)
	}
	paused := waitForState(t, service, created.ID, "paused")
	if paused.DownloadedBytes == 0 {
		t.Fatal("pause did not checkpoint progress")
	}
	if err := service.Resume(context.Background(), created.ID); err != nil {
		t.Fatal(err)
	}
	completed := waitForState(t, service, created.ID, "completed")
	if completed.DownloadedBytes != completed.TotalBytes {
		t.Fatalf("resume did not complete: %+v", completed)
	}
}

func TestResumeRequiresRestartWhenRangesDisappear(t *testing.T) {
	server := testserver.New()
	defer server.Close()
	service, database, _ := newTestService(t, server)
	defer database.Close()
	defer service.Close()

	created, err := service.Create(context.Background(), application.CreateDownloadInput{URL: server.URL("/no-range-slow"), DestinationDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Start(context.Background(), created.ID); err != nil {
		t.Fatal(err)
	}
	waitForState(t, service, created.ID, "downloading")
	waitForFileGrowth(t, created.TempPath, 0)
	if err := service.Pause(context.Background(), created.ID); err != nil {
		t.Fatal(err)
	}
	waitForState(t, service, created.ID, "paused")
	if err := service.Resume(context.Background(), created.ID); err != nil {
		t.Fatal(err)
	}
	failed := waitForState(t, service, created.ID, "failed")
	if !failed.RestartRequired {
		t.Fatalf("expected restart option: %+v", failed)
	}
}

func TestResumeDetectsRemoteResourceChange(t *testing.T) {
	server := testserver.New()
	defer server.Close()
	service, database, _ := newTestService(t, server)
	defer database.Close()
	defer service.Close()

	created, err := service.Create(context.Background(), application.CreateDownloadInput{URL: server.URL("/mutable"), DestinationDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Start(context.Background(), created.ID); err != nil {
		t.Fatal(err)
	}
	waitForState(t, service, created.ID, "downloading")
	waitForFileGrowth(t, created.TempPath, 0)
	if err := service.Pause(context.Background(), created.ID); err != nil {
		t.Fatal(err)
	}
	waitForState(t, service, created.ID, "paused")
	server.SetMutableVersion(2)
	if err := service.Resume(context.Background(), created.ID); err != nil {
		t.Fatal(err)
	}
	failed := waitForState(t, service, created.ID, "failed")
	if !failed.RestartRequired || !strings.Contains(failed.LastError, "changed") {
		t.Fatalf("remote change was not reported safely: %+v", failed)
	}
}

func TestStartupRecoveryReconcilesPartialFile(t *testing.T) {
	server := testserver.New()
	defer server.Close()
	ctx := context.Background()
	database, err := persistence.Open(ctx, filepath.Join(t.TempDir(), "recovery.db"))
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	tempPath := filepath.Join(directory, "recovery.bin.fluxpart")
	partial := make([]byte, 96*1024)
	if err := os.WriteFile(tempPath, partial, 0o600); err != nil {
		t.Fatal(err)
	}
	task := download.Download{
		ID: "recovery", URL: server.URL("/file"), FinalURL: server.URL("/file"),
		FileName: "recovery.bin", DestinationPath: filepath.Join(directory, "recovery.bin"), TempPath: tempPath,
		State: download.StateDownloading, TotalBytes: int64(len(server.Payload)), DownloadedBytes: 32 * 1024,
		RangeSupported: true, ETag: `"fixture-v1"`, CreatedAt: time.Now().UTC(),
		Connections: 1,
		Segments:    []download.Segment{{ID: "recovery:0", DownloadID: "recovery", EndByte: int64(len(server.Payload)) - 1, CurrentByte: 32 * 1024, State: download.SegmentDownloading, TempPath: tempPath}},
	}
	if err := database.Downloads().Create(ctx, task); err != nil {
		t.Fatal(err)
	}
	service := application.NewDownloadService(ctx, database.Downloads(), download.NewProber(server.HTTP.Client()), download.NewEngine(server.HTTP.Client()), events.NewBus())
	defer service.Close()
	defer database.Close()
	if err := service.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	recovered, err := service.Get(ctx, task.ID)
	if err != nil || recovered.State != "paused" || recovered.DownloadedBytes != int64(len(partial)) {
		t.Fatalf("unexpected recovery: %+v, %v", recovered, err)
	}
}

func TestStartupRecoveryResumesPersistedSegmentsWithoutCorruption(t *testing.T) {
	server := testserver.New()
	defer server.Close()
	ctx := context.Background()
	database, err := persistence.Open(ctx, filepath.Join(t.TempDir(), "segmented-recovery.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	directory := t.TempDir()
	tempPath := filepath.Join(directory, "segmented.bin.fluxpart")
	finalPath := filepath.Join(directory, "segmented.bin")
	total := int64(len(server.Payload))
	segments, err := download.PlanSegments("segmented-recovery", tempPath, total, 4)
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(tempPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(total); err != nil {
		t.Fatal(err)
	}
	segments[0].CurrentByte = segments[0].EndByte + 1
	segments[0].State = download.SegmentCompleted
	segments[1].CurrentByte = segments[1].StartByte + (segments[1].EndByte-segments[1].StartByte+1)/2
	segments[1].State = download.SegmentDownloading
	for _, segment := range segments[:2] {
		if segment.CurrentByte > segment.StartByte {
			if _, err := file.WriteAt(server.Payload[segment.StartByte:segment.CurrentByte], segment.StartByte); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	downloaded := (segments[0].CurrentByte - segments[0].StartByte) + (segments[1].CurrentByte - segments[1].StartByte)
	task := download.Download{
		ID: "segmented-recovery", URL: server.URL("/file"), FinalURL: server.URL("/file"),
		FileName: "segmented.bin", DestinationPath: finalPath, TempPath: tempPath,
		State: download.StateDownloading, TotalBytes: total, DownloadedBytes: downloaded,
		RangeSupported: true, ETag: `"fixture-v1"`, CreatedAt: time.Now().UTC(), Connections: 4, Segments: segments,
	}
	if err := database.Downloads().Create(ctx, task); err != nil {
		t.Fatal(err)
	}
	service := application.NewDownloadService(ctx, database.Downloads(), download.NewProber(server.HTTP.Client()), download.NewEngine(server.HTTP.Client()), events.NewBus())
	defer service.Close()
	if err := service.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	recovered, err := service.Get(ctx, task.ID)
	if err != nil || recovered.State != "paused" || recovered.DownloadedBytes != downloaded || recovered.SegmentCount != 4 {
		t.Fatalf("unexpected segmented recovery: %+v, %v", recovered, err)
	}
	if err := service.Resume(ctx, task.ID); err != nil {
		t.Fatal(err)
	}
	waitForState(t, service, task.ID, "completed")
	actual, err := os.ReadFile(finalPath)
	if err != nil {
		t.Fatal(err)
	}
	if sha256.Sum256(actual) != sha256.Sum256(server.Payload) {
		t.Fatal("segmented crash recovery corrupted the file")
	}
}

func TestPauseResumeOneHundredCyclesWithoutLeaks(t *testing.T) {
	baseline := runtime.NumGoroutine()
	server := testserver.New()
	service, database, _ := newTestService(t, server)
	created, err := service.Create(context.Background(), application.CreateDownloadInput{URL: server.URL("/pause-loop"), DestinationDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Start(context.Background(), created.ID); err != nil {
		t.Fatal(err)
	}
	var lastPaused application.DownloadDTO
	for cycle := 0; cycle < 100; cycle++ {
		waitForState(t, service, created.ID, "downloading")
		time.Sleep(5 * time.Millisecond)
		if err := service.Pause(context.Background(), created.ID); err != nil {
			t.Fatalf("pause cycle %d: %v", cycle, err)
		}
		lastPaused = waitForState(t, service, created.ID, "paused")
		probePath := created.TempPath + ".handle-check"
		if err := os.Rename(created.TempPath, probePath); err != nil {
			t.Fatalf("temporary file remains open after cycle %d: %v", cycle, err)
		}
		if err := os.Rename(probePath, created.TempPath); err != nil {
			t.Fatal(err)
		}
		if err := service.Resume(context.Background(), created.ID); err != nil {
			t.Fatalf("resume cycle %d: %v", cycle, err)
		}
	}
	if lastPaused.DownloadedBytes == 0 {
		t.Fatal("pause/resume stress test made no download progress")
	}
	waitForState(t, service, created.ID, "downloading")
	if err := service.Cancel(context.Background(), created.ID); err != nil {
		t.Fatal(err)
	}
	waitForState(t, service, created.ID, "cancelled")
	service.Close()
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	server.Close()
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	if current := runtime.NumGoroutine(); current > baseline+10 {
		t.Fatalf("goroutines grew from %d to %d", baseline, current)
	}
}

func TestDownloadServiceFailsOnInterruptedConnectionAndCanRetry(t *testing.T) {
	server := testserver.New()
	defer server.Close()
	service, database, _ := newTestService(t, server)
	defer database.Close()
	defer service.Close()

	created, err := service.Create(context.Background(), application.CreateDownloadInput{
		URL: server.URL("/interrupt"), DestinationDir: t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Start(context.Background(), created.ID); err != nil {
		t.Fatal(err)
	}
	waitForState(t, service, created.ID, "failed")
	if err := service.Start(context.Background(), created.ID); err != nil {
		t.Fatal(err)
	}
	failedAgain := waitForState(t, service, created.ID, "failed")
	if failedAgain.RetryCount != 1 {
		t.Fatalf("expected one retry, got %+v", failedAgain)
	}
}

func TestDownloadServiceValidatesCreateInput(t *testing.T) {
	server := testserver.New()
	defer server.Close()
	service, database, _ := newTestService(t, server)
	defer database.Close()
	defer service.Close()

	if _, err := service.Create(context.Background(), application.CreateDownloadInput{URL: "file:///unsafe", DestinationDir: t.TempDir()}); err == nil {
		t.Fatal("expected invalid URL to be rejected")
	}
	if _, err := service.Create(context.Background(), application.CreateDownloadInput{URL: server.URL("/file"), DestinationDir: filepath.Join(t.TempDir(), "missing")}); err == nil {
		t.Fatal("expected invalid destination to be rejected")
	}
}

func newTestService(t *testing.T, server *testserver.Server) (*application.DownloadService, *persistence.Database, *events.Bus) {
	t.Helper()
	database, err := persistence.Open(context.Background(), filepath.Join(t.TempDir(), "service.db"))
	if err != nil {
		t.Fatal(err)
	}
	bus := events.NewBus()
	service := application.NewDownloadService(context.Background(), database.Downloads(), download.NewProber(server.HTTP.Client()), download.NewEngine(server.HTTP.Client()), bus)
	return service, database, bus
}

func waitForState(t *testing.T, service *application.DownloadService, id, expected string) application.DownloadDTO {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		item, err := service.Get(context.Background(), id)
		if err == nil && item.State == expected {
			return item
		}
		time.Sleep(10 * time.Millisecond)
	}
	item, err := service.Get(context.Background(), id)
	t.Fatalf("download did not reach %q: %+v, %v", expected, item, err)
	return application.DownloadDTO{}
}

func waitForFileGrowth(t *testing.T, path string, previous int64) int64 {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if info, err := os.Stat(path); err == nil && info.Size() > previous {
			return info.Size()
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("temporary file %q did not grow beyond %d", path, previous)
	return 0
}
