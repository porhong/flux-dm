# Milestone 1 performance observations

## 1 GiB single-stream fixture

Command:

```powershell
go test ./internal/download -run '^$' -bench '^BenchmarkSingleStream1GiB$' -benchtime=1x -benchmem -count=1
```

Observed on 2026-07-15:

```text
Windows amd64
Intel Core i7-9750H @ 2.60 GHz
1,073,741,824 bytes
2.166819400 seconds/op
495.54 MB/s
385,032 B/op
262 allocations/op
0.3672 MiB total allocation delta reported by the benchmark
```

The source fixture generates a logical 1 GiB zero-filled stream without holding it in memory. The engine writes the full response to disk using one reusable 256 KiB buffer. The measurement is loopback plus local-disk throughput, not an internet-speed claim. Allocation results demonstrate bounded streaming memory; they are not a peak working-set measurement. Actual throughput depends on the server, network, disk, proxy, and security software.
