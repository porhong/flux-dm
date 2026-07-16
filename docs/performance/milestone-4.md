# Milestone 4 optimization report

## Environment and commands

Windows/amd64, Go 1.25.4, Intel Core i7-9750H (12 logical CPUs). Benchmarks use in-process deterministic HTTP sources, warmed filesystem/network paths, preallocated temporary files, normal sync/close/rename completion, and one measured iteration per case.

```powershell
go test ./internal/download -run '^$' -bench '^BenchmarkBufferSizes64MiB$' -benchtime=1x -benchmem -count=1
go test ./internal/download -run '^$' -bench '^BenchmarkSlowTailAdaptation16MiB$' -benchtime=1x -count=1
```

## Buffer size

Four connections transferred 64 MiB:

| Buffer per worker | Throughput | Go allocation |
|---:|---:|---:|
| 32 KiB | 231.91 MB/s | 0.404 MiB |
| 64 KiB | 246.28 MB/s | 0.466 MiB |
| 128 KiB | 331.99 MB/s | 0.783 MiB |
| 256 KiB | 413.96 MB/s | 1.276 MiB |
| 512 KiB | 302.24 MB/s | 2.322 MiB |

The existing 256 KiB default was fastest on this machine. The 512 KiB buffer increased allocation and reduced throughput, so Milestone 4 keeps 256 KiB rather than assuming larger buffers are better.

## Slow-tail redistribution

A 16 MiB source delayed only the fourth initial range:

| Mode | Wall time | Throughput |
|---|---:|---:|
| Fixed four ranges | 0.456 s | 36.81 MB/s |
| Dynamic tail takeover | 0.240 s | 69.96 MB/s |

Dynamic splitting improved this deliberately imbalanced case by 1.90×. It does not help the low-latency balanced benchmark in the Milestone 3 report, where one connection already saturates the local disk. Adaptation is therefore triggered only after an idle worker and a measurably slow, sufficiently large tail exist.

## Bandwidth accuracy and UI cadence

Integration tests measured a 512 KiB transfer limited to 512 KiB/s at 1.00 seconds, and two concurrent 256 KiB downloads sharing a global 256 KiB/s limit at 2.01 seconds. Both are within 1% of their target duration. The same test verifies nonzero smoothed speed and ETA output. File sync and frontend events remain throttled to 250 ms, while SQLite checkpoints remain capped at once per second.

## Profiles

Captured profiles:

- [`milestone-4-cpu.pprof`](profiles/milestone-4-cpu.pprof)
- [`milestone-4-memory.pprof`](profiles/milestone-4-memory.pprof)

The four-connection 256 MiB profile measured 494.03 MB/s, 0.438 process CPU seconds, and 1.396 MiB of Go allocation. CPU samples were dominated by Windows/runtime calls around network and file I/O. The largest engine-specific allocation site was per-segment copy-buffer setup; its bounded total matches the connection-count memory model documented in Milestone 3.
