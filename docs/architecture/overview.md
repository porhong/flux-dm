# Architecture overview

## Scope

Milestone 3 adds deterministic multi-connection segmented downloading while retaining validator-guarded resume, retry, restart, and startup recovery. It intentionally contains no dynamic segment splitting, browser integration, authentication, scheduler, or updater.

## Layers

```text
React / TypeScript UI
        |
        | typed Wails bindings and events
        v
Wails adapter (package main)
        |
        +--> internal/application  DTOs, errors, paths, orchestration
        +--> internal/events       synchronous typed in-process events
        +--> internal/persistence  SQLite connection and migrations
        +--> internal/logging      structured, redacted JSON logs
        +--> internal/download     independent engine boundary
        |                          probe, state machine, segment metadata, streaming engine
        +--> internal/filesystem   Windows-safe names and destinations
        +--> internal/transport    shared tuned HTTP client
        +--> internal/security     validation and secret-store boundary
        +--> internal/platform     narrowly scoped OS integrations
```

The Wails adapter owns desktop lifecycle callbacks and translates application events to Wails runtime events. Internal packages do not import Wails, keeping the download engine usable in deterministic tests and future command-line tools.

## Download flow

1. `CreateDownload` validates the HTTP/HTTPS URL and existing absolute destination directory, chooses a collision-free Windows-safe filename, and persists a `queued` record.
2. `StartDownload` adds the identifier to a bounded queue serviced by three fixed workers.
3. The worker transitions through `probing` and sends `HEAD`, followed by `GET` with `Range: bytes=0-0` to verify partial-request support.
4. Metadata and the final redirected URL are persisted before entering `preparing` and `downloading`.
5. For a known-size range-capable resource, the engine deterministically plans up to the selected 1, 2, 4, 8, or 16 inclusive ranges. Otherwise it uses one full-stream segment.
6. The engine preallocates `<filename>.fluxpart`, runs a bounded segment pool, and writes each response directly to its assigned offsets with a fixed 256 KiB buffer and `WriteAt`.
7. Progress events are throttled, while durable per-segment checkpoints are written no more often than once per second. Download and all segment rows are saved in one SQLite transaction.
8. A successful stream is size-verified, flushed, closed, and atomically renamed to its final path. Interrupted or truncated streams never create the final file.

## Pause and resume flow

1. `PauseDownload` persists `pausing` before cancelling the active request. That cancellation is interpreted as a user pause, not a failure.
2. The engine syncs and closes the `.fluxpart` file. The application reconciles the actual file size and transactionally persists `paused` plus segment progress.
3. `ResumeDownload` probes the URL again before writing. It rejects changed size, ETag, or Last-Modified validators and rejects a server that no longer supports ranges.
4. Each incomplete segment sends `Range: bytes=<current>-<segment-end>` and `If-Range` using the stored ETag, or Last-Modified when no ETag exists. Only an exact matching `206 Partial Content` response is accepted.
5. Retry preserves a safe partial file and follows the same validation path. Restart truncates the partial file, clears stale validators and progress, and starts from byte zero.

## Startup and shutdown

1. Resolve the per-user configuration directory and create it with user-only permissions.
2. Open the structured log.
3. Start Wails and receive its lifecycle context.
4. Open SQLite, configure foreign keys/WAL/busy timeout, and apply embedded migrations transactionally.
5. Reconcile downloads left in transient states. For a preallocated multi-segment file, the file must match the expected total length and persisted segment cursors identify written bytes. Legacy/non-preallocated single streams reconcile their cursor from actual file length.
6. Register the application-event bridge and publish `app.ready`.
7. Close workers, SQLite, and the log during orderly shutdown.

If persistence initialization fails, the application logs a sanitized error, presents a generic desktop error, and returns an unavailable health status.

## Data ownership

SQLite entities stay in `internal/persistence`. DTOs exposed to Wails stay in `internal/application`; the frontend validates unknown binding output with Zod. Database schema changes require a numbered migration embedded into the binary.

## Frontend

The frontend uses React strict mode, Tailwind CSS v4, shadcn/ui primitives, and feature-scoped state. Zustand holds low-frequency shell state. React Hook Form and Zod validate Add Download input and Wails responses. Progress uses per-download external signals so an update rerenders only the affected row. Generated Wails bindings are excluded from linting and regenerated by Wails builds.
