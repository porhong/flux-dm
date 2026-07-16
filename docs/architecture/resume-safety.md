# Resume safety and failure behavior

FluxDM preserves a partial download only when it can prove where the next byte belongs. Milestone 3 persists one cursor for every planned segment.

## Durable data

The download row and all of its segment rows are committed in one SQLite transaction. A checkpoint contains the original and final URL, expected length, ETag, Last-Modified value, range-support result, temporary path, segment bounds, and each current byte. Progress events may be more frequent, but durable checkpoints occur no more often than once per second and the file is synced before a checkpoint is reported.

Known-size segmented transfers preallocate `.fluxpart` to the final size, so file length cannot represent progress. On startup the file length must equal the expected size and the transactionally persisted, range-validated segment cursors identify valid bytes. A legacy or unknown-size single stream can still reconcile its cursor from actual file length. Missing, inaccessible, incorrectly sized, overlapping, gapped, or out-of-range state is not resumed.

## Safe resume protocol

Before every resume or retry, FluxDM probes the resource again and compares its expected length and available ETag and Last-Modified validators with the stored metadata. A changed validator or length fails safely and requires Restart.

For a non-empty partial file, the engine sends:

```http
Range: bytes=<segment-current>-<segment-end>
If-Range: <stored ETag, otherwise stored Last-Modified>
```

The response must be `206 Partial Content`. Its `Content-Range` must exactly match the requested start and end and report the expected total length. A `200 OK`, malformed range, wrong boundary, inconsistent length, or inconsistent total is rejected without writing that response. A new transfer may safely truncate and fall back to one full stream; a transfer with persisted partial progress requires Restart when ranges become unreliable.

## User actions

- **Pause** changes the state to `pausing`, cancels the active request, syncs and closes the file, reconciles its length, then records `paused`. This intentional cancellation is not a failed download.
- **Resume** continues a paused partial only after the safe resume protocol succeeds.
- **Retry** retains a partial created by a transient failure and applies the same probe and range validation as Resume.
- **Restart** truncates the partial file, clears remote validators and progress, and probes/downloads again from byte zero. It is the recovery path when the resource changed, range support disappeared, or local recovery cannot trust the partial.
- **Cancel** removes the partial file and resets progress. It is distinct from Pause.

## Crash and shutdown recovery

Downloads found in `probing`, `preparing`, `downloading`, `pausing`, or `retrying` are reconciled during application startup:

- no partial bytes: return to `queued`;
- partial bytes plus previously verified range support: become `paused` and wait for the user;
- partial bytes without range support, or an inconsistent partial: become `failed` and require Restart.

FluxDM never silently joins a full `200 OK` response onto an existing partial and never publishes the final filename until the expected stream has completed and the atomic rename succeeds.

## Known boundary

HTTP validators are server assertions, not content hashes. A server that changes bytes while preserving all validators can defeat validator-based resume. A later integrity-verification milestone can add trusted checksums or signatures when a trustworthy expected value is available.
