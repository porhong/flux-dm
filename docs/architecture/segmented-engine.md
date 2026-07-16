# Segmented download engine

## Planning

The planner accepts 1, 2, 4, 8, or 16 connections. For a known positive size it divides bytes into deterministic, contiguous, inclusive ranges. Earlier ranges receive one additional byte when division has a remainder. Files smaller than the selected count use one non-empty segment per byte; empty ranges are never created.

Every plan is validated before network or file work. Segment indexes must be ordered, the first range must start at zero, adjacent ranges must meet without gaps or overlap, cursors must remain inside their assigned range, and the last range must end at `totalBytes - 1`.

## Execution and writes

The application queue remains bounded at three active downloads. Each transfer creates at most 16 segment workers, and the shared transport caps per-host connections at 16 while reusing healthy idle connections.

Known-size temporary files are preallocated once. Segment workers share the open file and call `WriteAt` through a writer that rejects any write beyond the segment's exclusive end offset. Workers never seek a shared file cursor and never buffer the resource in memory. Each worker owns one planned segment at a time, preventing overlapping writers.

Each ranged response must be `206 Partial Content` with an exact `Content-Range` start, end, and total plus the expected response length. Invalid or ignored ranges cancel the other workers. If no persisted partial existed, FluxDM truncates the temporary file and safely falls back to one full-stream connection. Existing resumable data is never discarded implicitly.

## Dynamic takeover and adaptation

When a worker becomes idle, the scheduler watches the final active segment. If that tail remains active past the slow threshold and has at least two minimum split units remaining, FluxDM cancels only that request, snapshots its exact written cursor, shortens the original range, appends a new contiguous range for the other half, and assigns both halves to bounded workers. Splits are capped at four times the configured worker count. The append-only tail rule keeps persisted indexes stable and makes gap/overlap validation straightforward.

HTTP 429 and 503 responses reduce the per-download connection controller by half, down to one. Successful segments gradually add capacity back toward the configured count. This controller gates requests independently from the fixed worker bound, so server pressure cannot create more goroutines.

Global and per-download rate limiters reserve write time from shared monotonic schedules before `WriteAt`. Multiple segments and downloads therefore consume one aggregate budget instead of each receiving the full configured rate. Progress snapshots compute an exponentially smoothed speed and ETA without increasing the 250 ms frontend event cadence.

## Retry policy

Every segment has at most four attempts. Connection failures and HTTP 429, 500, and 503 responses use exponential backoff with bounded deterministic jitter. Invalid range metadata and non-retryable HTTP statuses fail immediately. An in-process retry begins at that segment's latest written cursor; crash recovery begins at the latest synced transactional checkpoint.

## Progress and completion

The reporter snapshots every segment under one mutex, syncs the file before publishing, and emits at most once per 250 milliseconds plus a final event. The application saves the aggregate byte count and all segment cursors in one SQLite transaction at most once per second. The final file is published only after all ranges are complete, aggregate bytes and file size equal the expected total, the file is synced and closed, and the atomic rename succeeds.

Tests compare SHA-256 output at every supported connection count and after connection resets, slow segments, retryable HTTP statuses, malformed range fallback, pause/resume stress, and simulated crash recovery.
