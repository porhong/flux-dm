package download_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fluxdm/fluxdm/internal/download"
	"golang.org/x/sys/windows"
)

const benchmarkSize = int64(256 << 20)

func BenchmarkSlowTailAdaptation16MiB(b *testing.B) {
	payload := deterministicPayload(16 * 1024 * 1024)
	server := newPayloadServer(payload, 12*1024*1024)
	defer server.Close()
	for _, mode := range []struct {
		name    string
		options download.EngineOptions
	}{
		{name: "fixed", options: download.EngineOptions{DynamicSplitMinBytes: int64(len(payload))}},
		{name: "dynamic", options: download.EngineOptions{DynamicSplitMinBytes: 256 * 1024, SlowSegmentThreshold: 20 * time.Millisecond}},
	} {
		b.Run(mode.name, func(b *testing.B) {
			engine := download.NewEngineWithOptions(server.Client(), mode.options)
			directory := b.TempDir()
			b.SetBytes(int64(len(payload)))
			b.ResetTimer()
			for index := 0; index < b.N; index++ {
				id := fmt.Sprintf("slow-%s-%d", mode.name, index)
				finalPath := filepath.Join(directory, id+".bin")
				segments, err := download.PlanSegments(id, finalPath+".fluxpart", int64(len(payload)), 4)
				if err != nil {
					b.Fatal(err)
				}
				if err := engine.Download(context.Background(), download.Download{
					ID: id, URL: server.URL, FinalURL: server.URL, DestinationPath: finalPath,
					TempPath: finalPath + ".fluxpart", TotalBytes: int64(len(payload)), RangeSupported: true,
					Connections: 4, Segments: segments,
				}, nil); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkBufferSizes64MiB(b *testing.B) {
	const total = int64(64 << 20)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start, end := int64(0), total-1
		if value := r.Header.Get("Range"); value != "" {
			parts := strings.Split(strings.TrimPrefix(value, "bytes="), "-")
			start, _ = strconv.ParseInt(parts[0], 10, 64)
			end, _ = strconv.ParseInt(parts[1], 10, 64)
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, total))
			w.Header().Set("Content-Length", strconv.FormatInt(end-start+1, 10))
			w.WriteHeader(http.StatusPartialContent)
		}
		_, _ = io.CopyN(w, zeroReader{}, end-start+1)
	}))
	defer server.Close()
	warmDirectory := b.TempDir()
	warmPath := filepath.Join(warmDirectory, "buffer-warmup.bin")
	warmSegments, err := download.PlanSegments("buffer-warmup", warmPath+".fluxpart", total, 4)
	if err != nil {
		b.Fatal(err)
	}
	if err := download.NewEngineWithOptions(server.Client(), download.EngineOptions{BufferSize: 128 * 1024}).Download(context.Background(), download.Download{
		ID: "buffer-warmup", URL: server.URL, FinalURL: server.URL, DestinationPath: warmPath,
		TempPath: warmPath + ".fluxpart", TotalBytes: total, RangeSupported: true, Connections: 4, Segments: warmSegments,
	}, nil); err != nil {
		b.Fatal(err)
	}
	for _, bufferSize := range []int{32, 64, 128, 256, 512} {
		b.Run(fmt.Sprintf("buffer_%d_KiB", bufferSize), func(b *testing.B) {
			engine := download.NewEngineWithOptions(server.Client(), download.EngineOptions{BufferSize: bufferSize * 1024})
			directory := b.TempDir()
			b.SetBytes(total)
			b.ReportAllocs()
			b.ResetTimer()
			for index := 0; index < b.N; index++ {
				id := fmt.Sprintf("buffer-%d-%d", bufferSize, index)
				finalPath := filepath.Join(directory, id+".bin")
				segments, err := download.PlanSegments(id, finalPath+".fluxpart", total, 4)
				if err != nil {
					b.Fatal(err)
				}
				if err := engine.Download(context.Background(), download.Download{
					ID: id, URL: server.URL, FinalURL: server.URL, DestinationPath: finalPath,
					TempPath: finalPath + ".fluxpart", TotalBytes: total, RangeSupported: true,
					Connections: 4, Segments: segments,
				}, nil); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkSegmented256MiB(b *testing.B) {
	var active atomic.Int64
	var peak atomic.Int64
	var peakGoroutines atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("Content-Length", strconv.FormatInt(benchmarkSize, 10))
		if r.Method == http.MethodHead {
			return
		}
		start, end := int64(0), benchmarkSize-1
		if value := r.Header.Get("Range"); value != "" {
			parts := strings.Split(strings.TrimPrefix(value, "bytes="), "-")
			start, _ = strconv.ParseInt(parts[0], 10, 64)
			if parts[1] != "" {
				end, _ = strconv.ParseInt(parts[1], 10, 64)
			}
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, benchmarkSize))
			w.Header().Set("Content-Length", strconv.FormatInt(end-start+1, 10))
			w.WriteHeader(http.StatusPartialContent)
		}
		current := active.Add(1)
		for current > peak.Load() && !peak.CompareAndSwap(peak.Load(), current) {
		}
		goroutines := int64(runtime.NumGoroutine())
		for goroutines > peakGoroutines.Load() && !peakGoroutines.CompareAndSwap(peakGoroutines.Load(), goroutines) {
		}
		defer active.Add(-1)
		_, _ = io.CopyN(w, zeroReader{}, end-start+1)
	}))
	defer server.Close()
	warmDirectory := b.TempDir()
	warmPath := filepath.Join(warmDirectory, "warmup.bin")
	warmSegments, err := download.PlanSegments("benchmark-warmup", warmPath+".fluxpart", benchmarkSize, 1)
	if err != nil {
		b.Fatal(err)
	}
	if err := download.NewEngine(server.Client()).Download(context.Background(), download.Download{
		ID: "benchmark-warmup", URL: server.URL, FinalURL: server.URL,
		DestinationPath: warmPath, TempPath: warmPath + ".fluxpart", TotalBytes: benchmarkSize,
		RangeSupported: true, Connections: 1, Segments: warmSegments,
	}, nil); err != nil {
		b.Fatal(err)
	}

	for _, connections := range []int{1, 2, 4, 8, 16} {
		b.Run(fmt.Sprintf("connections_%d", connections), func(b *testing.B) {
			peak.Store(0)
			peakGoroutines.Store(0)
			engine := download.NewEngine(server.Client())
			directory := b.TempDir()
			b.SetBytes(benchmarkSize)
			b.ReportAllocs()
			beforeGoroutines := runtime.NumGoroutine()
			var before, after runtime.MemStats
			runtime.GC()
			runtime.ReadMemStats(&before)
			beforeCPU := processCPUSeconds()
			b.ResetTimer()
			for index := 0; index < b.N; index++ {
				finalPath := filepath.Join(directory, fmt.Sprintf("segmented-%d.bin", index))
				id := fmt.Sprintf("benchmark-%d-%d", connections, index)
				segments, err := download.PlanSegments(id, finalPath+".fluxpart", benchmarkSize, connections)
				if err != nil {
					b.Fatal(err)
				}
				task := download.Download{ID: id, URL: server.URL, FinalURL: server.URL,
					DestinationPath: finalPath, TempPath: finalPath + ".fluxpart", TotalBytes: benchmarkSize,
					RangeSupported: true, Connections: connections, Segments: segments}
				if err := engine.Download(context.Background(), task, nil); err != nil {
					b.Fatal(err)
				}
			}
			b.StopTimer()
			runtime.ReadMemStats(&after)
			afterCPU := processCPUSeconds()
			b.ReportMetric(float64(after.TotalAlloc-before.TotalAlloc)/(1<<20)/float64(b.N), "alloc_MiB/op")
			b.ReportMetric((afterCPU-beforeCPU)/float64(b.N), "cpu_s/op")
			b.ReportMetric(float64(runtime.NumGoroutine()-beforeGoroutines), "goroutine_delta")
			b.ReportMetric(float64(peakGoroutines.Load()), "peak_goroutines")
			b.ReportMetric(float64(peak.Load()), "peak_requests")
		})
	}
}

type zeroReader struct{}

func (zeroReader) Read(buffer []byte) (int, error) { clear(buffer); return len(buffer), nil }

func processCPUSeconds() float64 {
	var creation, exit, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(windows.CurrentProcess(), &creation, &exit, &kernel, &user); err != nil {
		return 0
	}
	return float64(kernel.Nanoseconds()+user.Nanoseconds()) / float64(1e9)
}
