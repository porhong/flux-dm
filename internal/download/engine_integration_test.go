package download_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fluxdm/fluxdm/internal/download"
	"github.com/fluxdm/fluxdm/tests/testserver"
)

func TestEngineSegmentedDownloadsMatchSHA256(t *testing.T) {
	server := testserver.New()
	defer server.Close()
	want := sha256.Sum256(server.Payload)
	for _, connections := range []int{1, 2, 4, 8, 16} {
		t.Run(fmt.Sprintf("%d_connections", connections), func(t *testing.T) {
			task := segmentedTask(t, server.URL("/file"), t.TempDir(), int64(len(server.Payload)), connections)
			if err := download.NewEngine(server.HTTP.Client()).Download(context.Background(), task, nil); err != nil {
				t.Fatal(err)
			}
			actual, err := os.ReadFile(task.DestinationPath)
			if err != nil {
				t.Fatal(err)
			}
			if got := sha256.Sum256(actual); got != want {
				t.Fatalf("SHA-256 mismatch for %d connections", connections)
			}
		})
	}
}

func TestEngineDynamicallySplitsSlowTailWithoutCorruption(t *testing.T) {
	payload := deterministicPayload(8 * 1024 * 1024)
	server := newPayloadServer(payload, 6*1024*1024)
	defer server.Close()
	task := segmentedTask(t, server.URL, t.TempDir(), int64(len(payload)), 4)
	engine := download.NewEngineWithOptions(server.Client(), download.EngineOptions{
		DynamicSplitMinBytes: 256 * 1024, SlowSegmentThreshold: 20 * time.Millisecond,
		ProgressInterval: 10 * time.Millisecond,
	})
	var last download.Progress
	if err := engine.Download(context.Background(), task, func(progress download.Progress) { last = progress }); err != nil {
		t.Fatal(err)
	}
	if len(last.Segments) <= 4 {
		t.Fatalf("slow tail was not split: %d segments", len(last.Segments))
	}
	if err := download.ValidateSegments(last.Segments, int64(len(payload))); err != nil {
		t.Fatal(err)
	}
	actual, err := os.ReadFile(task.DestinationPath)
	if err != nil || sha256.Sum256(actual) != sha256.Sum256(payload) {
		t.Fatalf("dynamic split corrupted output: %v", err)
	}
}

func TestEnginePerDownloadBandwidthLimitAndSpeedEstimate(t *testing.T) {
	payload := deterministicPayload(512 * 1024)
	server := newPayloadServer(payload, -1)
	defer server.Close()
	task := segmentedTask(t, server.URL, t.TempDir(), int64(len(payload)), 2)
	task.BandwidthLimit = 512 * 1024
	engine := download.NewEngineWithOptions(server.Client(), download.EngineOptions{BufferSize: 32 * 1024, ProgressInterval: 50 * time.Millisecond})
	var mu sync.Mutex
	var observedSpeed bool
	started := time.Now()
	if err := engine.Download(context.Background(), task, func(progress download.Progress) {
		mu.Lock()
		observedSpeed = observedSpeed || (progress.SpeedBytesPerSecond > 0 && progress.ETASeconds >= 0)
		mu.Unlock()
	}); err != nil {
		t.Fatal(err)
	}
	elapsed := time.Since(started)
	if elapsed < 850*time.Millisecond || elapsed > 2*time.Second {
		t.Fatalf("per-download limit completed in %v, want about 1s", elapsed)
	}
	mu.Lock()
	speedSeen := observedSpeed
	mu.Unlock()
	if !speedSeen {
		t.Fatal("smoothed speed and ETA were not reported")
	}
}

func TestEngineGlobalBandwidthLimitIsShared(t *testing.T) {
	payload := deterministicPayload(256 * 1024)
	server := newPayloadServer(payload, -1)
	defer server.Close()
	engine := download.NewEngineWithOptions(server.Client(), download.EngineOptions{BufferSize: 32 * 1024})
	if err := engine.SetGlobalBandwidthLimit(256 * 1024); err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	tasks := []download.Download{
		segmentedTaskWithID(t, "global-a", server.URL, directory, int64(len(payload)), 2),
		segmentedTaskWithID(t, "global-b", server.URL, directory, int64(len(payload)), 2),
	}
	started := time.Now()
	errorsChannel := make(chan error, len(tasks))
	for index := range tasks {
		go func(task download.Download) { errorsChannel <- engine.Download(context.Background(), task, nil) }(tasks[index])
	}
	for range tasks {
		if err := <-errorsChannel; err != nil {
			t.Fatal(err)
		}
	}
	elapsed := time.Since(started)
	if elapsed < 1700*time.Millisecond || elapsed > 3*time.Second {
		t.Fatalf("shared global limit completed in %v, want about 2s", elapsed)
	}
}

func TestEngineQueueBandwidthLimitIsShared(t *testing.T) {
	payload := deterministicPayload(256 * 1024)
	server := newPayloadServer(payload, -1)
	defer server.Close()
	engine := download.NewEngineWithOptions(server.Client(), download.EngineOptions{BufferSize: 32 * 1024})
	directory := t.TempDir()
	tasks := []download.Download{
		segmentedTaskWithID(t, "queue-a", server.URL, directory, int64(len(payload)), 2),
		segmentedTaskWithID(t, "queue-b", server.URL, directory, int64(len(payload)), 2),
	}
	for index := range tasks {
		tasks[index].QueueID = "limited-queue"
		tasks[index].QueueBandwidthLimit = 256 * 1024
	}
	started := time.Now()
	errorsChannel := make(chan error, len(tasks))
	for index := range tasks {
		go func(task download.Download) { errorsChannel <- engine.Download(context.Background(), task, nil) }(tasks[index])
	}
	for range tasks {
		if err := <-errorsChannel; err != nil {
			t.Fatal(err)
		}
	}
	elapsed := time.Since(started)
	if elapsed < 1700*time.Millisecond || elapsed > 3*time.Second {
		t.Fatalf("shared queue limit completed in %v, want about 2s", elapsed)
	}
}

func TestEngineRetriesSegmentConnectionResets(t *testing.T) {
	server := testserver.New()
	defer server.Close()
	task := segmentedTask(t, server.URL("/range-reset"), t.TempDir(), int64(len(server.Payload)), 4)
	var last download.Progress
	if err := download.NewEngine(server.HTTP.Client()).Download(context.Background(), task, func(progress download.Progress) { last = progress }); err != nil {
		t.Fatal(err)
	}
	for _, segment := range last.Segments {
		if segment.RetryCount == 0 {
			t.Fatalf("segment %d did not record its reset retry", segment.Index)
		}
	}
	actual, err := os.ReadFile(task.DestinationPath)
	if err != nil || sha256.Sum256(actual) != sha256.Sum256(server.Payload) {
		t.Fatalf("reset recovery corrupted output: %v", err)
	}
}

func TestEngineRetries429500And503PerSegment(t *testing.T) {
	for _, status := range []int{429, 500, 503} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			server := testserver.New()
			defer server.Close()
			task := segmentedTask(t, server.URL(fmt.Sprintf("/range-%d", status)), t.TempDir(), int64(len(server.Payload)), 4)
			var last download.Progress
			if err := download.NewEngine(server.HTTP.Client()).Download(context.Background(), task, func(progress download.Progress) { last = progress }); err != nil {
				t.Fatal(err)
			}
			for _, segment := range last.Segments {
				if segment.RetryCount == 0 {
					t.Fatalf("segment %d did not record HTTP %d retry", segment.Index, status)
				}
			}
			actual, err := os.ReadFile(task.DestinationPath)
			if err != nil || sha256.Sum256(actual) != sha256.Sum256(server.Payload) {
				t.Fatalf("status retry corrupted output: %v", err)
			}
		})
	}
}

func TestEngineCompletesWithSlowSegments(t *testing.T) {
	server := testserver.New()
	defer server.Close()
	task := segmentedTask(t, server.URL("/range-slow"), t.TempDir(), int64(len(server.Payload)), 8)
	if err := download.NewEngine(server.HTTP.Client()).Download(context.Background(), task, nil); err != nil {
		t.Fatal(err)
	}
}

func TestEngineFallsBackWhenContentRangeIsInvalid(t *testing.T) {
	server := testserver.New()
	defer server.Close()
	task := segmentedTask(t, server.URL("/invalid-range"), t.TempDir(), int64(len(server.Payload)), 8)
	var last download.Progress
	if err := download.NewEngine(server.HTTP.Client()).Download(context.Background(), task, func(progress download.Progress) { last = progress }); err != nil {
		t.Fatal(err)
	}
	if len(last.Segments) != 1 {
		t.Fatalf("fallback kept %d segments, want 1", len(last.Segments))
	}
	actual, err := os.ReadFile(task.DestinationPath)
	if err != nil || sha256.Sum256(actual) != sha256.Sum256(server.Payload) {
		t.Fatalf("fallback corrupted output: %v", err)
	}
}

func TestEngineDownloadsAndAtomicallyRenames(t *testing.T) {
	server := testserver.New()
	defer server.Close()
	directory := t.TempDir()
	task := testTask(server.URL("/file"), directory, int64(len(server.Payload)))
	var lastProgress download.Progress
	if err := download.NewEngine(server.HTTP.Client()).Download(context.Background(), task, func(progress download.Progress) { lastProgress = progress }); err != nil {
		t.Fatal(err)
	}
	actual, err := os.ReadFile(task.DestinationPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(actual) != string(server.Payload) {
		t.Fatal("downloaded content did not match fixture")
	}
	if _, err := os.Stat(task.TempPath); !os.IsNotExist(err) {
		t.Fatalf("temporary file still exists: %v", err)
	}
	if lastProgress.DownloadedBytes != int64(len(server.Payload)) {
		t.Fatalf("unexpected final progress: %+v", lastProgress)
	}
}

func TestEngineSupportsUnknownContentLength(t *testing.T) {
	server := testserver.New()
	defer server.Close()
	task := testTask(server.URL("/unknown"), t.TempDir(), -1)
	if err := download.NewEngine(server.HTTP.Client()).Download(context.Background(), task, nil); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(task.DestinationPath)
	if err != nil || info.Size() != int64(len(server.Payload)) {
		t.Fatalf("unexpected completed file: %v, %v", info, err)
	}
}

func TestEngineResumesWithRangeAndIfRange(t *testing.T) {
	server := testserver.New()
	defer server.Close()
	directory := t.TempDir()
	task := testTask(server.URL("/file"), directory, int64(len(server.Payload)))
	checkpoint := int64(len(server.Payload) / 3)
	task.ETag = `"fixture-v1"`
	task.Segments[0].CurrentByte = checkpoint
	if err := os.WriteFile(task.TempPath, server.Payload[:checkpoint], 0o600); err != nil {
		t.Fatal(err)
	}
	if err := download.NewEngine(server.HTTP.Client()).Download(context.Background(), task, nil); err != nil {
		t.Fatal(err)
	}
	actual, err := os.ReadFile(task.DestinationPath)
	if err != nil || string(actual) != string(server.Payload) {
		t.Fatalf("resumed file mismatch: %v", err)
	}
}

func TestEngineRejectsResumeWhenServerIgnoresRange(t *testing.T) {
	server := testserver.New()
	defer server.Close()
	task := testTask(server.URL("/unknown"), t.TempDir(), int64(len(server.Payload)))
	task.Segments[0].CurrentByte = 32
	if err := os.WriteFile(task.TempPath, server.Payload[:32], 0o600); err != nil {
		t.Fatal(err)
	}
	err := download.NewEngine(server.HTTP.Client()).Download(context.Background(), task, nil)
	if !errors.Is(err, download.ErrRangeUnsupported) {
		t.Fatalf("expected range error, got %v", err)
	}
	info, statErr := os.Stat(task.TempPath)
	if statErr != nil || info.Size() != 32 {
		t.Fatalf("partial file was modified: %v, %v", info, statErr)
	}
}

func TestEngineCancellationStopsBeforeRename(t *testing.T) {
	server := testserver.New()
	defer server.Close()
	task := testTask(server.URL("/slow"), t.TempDir(), 32*1024*1024)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	err := download.NewEngine(server.HTTP.Client()).Download(ctx, task, nil)
	if !errors.Is(err, download.ErrCancelled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
	if _, err := os.Stat(task.DestinationPath); !os.IsNotExist(err) {
		t.Fatalf("final file should not exist: %v", err)
	}
}

func TestEngineDetectsNetworkInterruption(t *testing.T) {
	server := testserver.New()
	defer server.Close()
	task := testTask(server.URL("/interrupt"), t.TempDir(), 100)
	if err := download.NewEngine(server.HTTP.Client()).Download(context.Background(), task, nil); err == nil {
		t.Fatal("expected interrupted download to fail")
	}
	if _, err := os.Stat(task.DestinationPath); !os.IsNotExist(err) {
		t.Fatalf("final file should not exist: %v", err)
	}
}

func testTask(rawURL, directory string, total int64) download.Download {
	finalPath := filepath.Join(directory, "fixture.bin")
	end := total - 1
	if total < 0 {
		end = -1
	}
	return download.Download{
		ID: "test-download", URL: rawURL, FinalURL: rawURL, FileName: "fixture.bin",
		DestinationPath: finalPath, TempPath: finalPath + ".fluxpart", TotalBytes: total,
		RangeSupported: true, Connections: 1,
		Segments: []download.Segment{{ID: "test-download:0", DownloadID: "test-download", Index: 0, StartByte: 0, EndByte: end, CurrentByte: 0, State: download.SegmentPending, TempPath: finalPath + ".fluxpart"}},
	}
}

func segmentedTask(t *testing.T, rawURL, directory string, total int64, connections int) download.Download {
	return segmentedTaskWithID(t, "test-download", rawURL, directory, total, connections)
}

func segmentedTaskWithID(t *testing.T, id, rawURL, directory string, total int64, connections int) download.Download {
	t.Helper()
	task := testTask(rawURL, directory, total)
	task.ID = id
	task.DestinationPath = filepath.Join(directory, id+".bin")
	task.TempPath = task.DestinationPath + ".fluxpart"
	task.Connections = connections
	segments, err := download.PlanSegments(task.ID, task.TempPath, total, connections)
	if err != nil {
		t.Fatal(err)
	}
	task.Segments = segments
	return task
}

func deterministicPayload(size int) []byte {
	payload := make([]byte, size)
	for index := range payload {
		payload[index] = byte((index*31 + 17) % 251)
	}
	return payload
}

func newPayloadServer(payload []byte, slowFrom int64) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Accept-Ranges", "bytes")
		if r.Method == http.MethodHead {
			w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
			return
		}
		start, end := int64(0), int64(len(payload)-1)
		if value := r.Header.Get("Range"); value != "" {
			parts := strings.Split(strings.TrimPrefix(value, "bytes="), "-")
			start, _ = strconv.ParseInt(parts[0], 10, 64)
			end, _ = strconv.ParseInt(parts[1], 10, 64)
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(payload)))
			w.WriteHeader(http.StatusPartialContent)
		}
		for offset := start; offset <= end; {
			next := offset + 32*1024
			if next > end+1 {
				next = end + 1
			}
			if _, err := w.Write(payload[offset:next]); err != nil {
				return
			}
			if slowFrom >= 0 && start >= slowFrom {
				time.Sleep(3 * time.Millisecond)
			}
			offset = next
		}
	}))
}
