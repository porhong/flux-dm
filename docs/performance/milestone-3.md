# Milestone 3 performance report

## Method

Measured on Windows/amd64 with Go 1.25.4 and an Intel Core i7-9750H (12 logical CPUs). The benchmark transfers a warmed-up 256 MiB deterministic stream from an in-process HTTP server into a preallocated temporary file, runs exactly one measured transfer for each supported connection count, and performs the normal sync, size verification, close, and atomic rename.

Command:

```powershell
go test ./internal/download -run '^$' -bench '^BenchmarkSegmented256MiB$' -benchtime=1x -benchmem -count=1
```

## Results

| Connections | Wall time | Throughput | Process CPU | CPU / one core | Go allocation | Allocations | Peak requests | Peak process goroutines |
|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| 1 | 0.334 s | 803.83 MB/s | 0.359 s | 107.6% | 0.337 MiB | 156 | 1 | 9 |
| 2 | 0.454 s | 590.90 MB/s | 0.391 s | 86.0% | 0.686 MiB | 357 | 2 | 14 |
| 4 | 0.472 s | 569.20 MB/s | 0.531 s | 112.6% | 1.356 MiB | 669 | 4 | 24 |
| 8 | 0.535 s | 501.41 MB/s | 0.438 s | 81.7% | 2.789 MiB | 1,725 | 8 | 44 |
| 16 | 0.546 s | 491.36 MB/s | 0.672 s | 123.0% | 5.567 MiB | 3,059 | 16 | 84 |

`CPU / one core` is process CPU seconds divided by wall seconds and can exceed 100% when work runs on multiple cores. Allocation is total Go heap allocation during the operation, not peak working set. Goroutine counts include the benchmark HTTP server and transport as well as segment workers.

## Interpretation

The local source has effectively zero latency and the destination disk is the bottleneck, so one connection is fastest on this machine. More connections add buffers, HTTP/server goroutines, synchronization, and allocations without creating more source bandwidth. Real remote servers with latency or per-connection throttling may benefit from multiple ranges, which is why FluxDM exposes the count instead of assuming 16 is always better. Four connections remains the UI default as a conservative network-oriented tradeoff; one connection is the appropriate choice for fast local or already-saturated sources.

Memory growth is bounded and approximately linear with connection count because every active segment owns a 256 KiB copy buffer plus HTTP/runtime overhead. No dynamic splitting or unbounded worker creation occurs.
