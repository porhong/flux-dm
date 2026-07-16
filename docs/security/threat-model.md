# FluxDM threat model

## Assets and trust boundaries

- User downloads, partial files, destination folders, history, schedules, and configuration.
- URLs, signed query strings, authentication headers, cookies, proxy credentials, and DPAPI ciphertext.
- Browser extension messages, native-host framing, the per-session loopback token, and installer/update artifacts.
- Remote HTTP servers, redirects, filenames, browser messages, frontend calls, proxy servers, and imported settings are untrusted.
- The local filesystem can contain attacker-controlled names and reparse points. Logs and crash reports may be shared and are treated as disclosure surfaces.

## Implemented controls

### Network and download integrity

- Only HTTP/HTTPS URLs with a hostname and no embedded credentials are accepted. Redirect targets are revalidated and limited to ten hops.
- TLS handshake, response header, idle, request, and redirect bounds are configured. Response headers are capped at 1 MiB.
- Cross-host redirects strip every site-profile header, including custom fields. Go also applies its sensitive-header redirect policy.
- Segment ranges are contiguous, non-overlapping, bounded, and checked against `Content-Range`. Resume re-probes size, ETag, and Last-Modified and uses `If-Range`.
- Downloads stream through fixed buffers and bounded workers; retries, queues, connections, native messages, HTTP bodies, and channels have explicit limits.
- Final files appear only after successful close and atomic rename. FluxDM never opens or executes a completed file.

### Filesystem and executable handling

- Filenames are converted to valid UTF-8, stripped of traversal/Windows-invalid characters, protected from device names/trailing dots, and capped at 240 bytes.
- Destination directories must be existing absolute paths and are resolved through symlinks/reparse points before use.
- Volume roots, Windows, Program Files, Program Files (x86), and ProgramData (including descendants and resolved junction targets) are rejected as destinations.
- `.fluxpart` names are atomically reserved with exclusive creation. Existing final/partial paths trigger collision-free selection.
- Executable, installer, script, and other code-bearing extensions require an explicit confirmation in the Add dialog. Browser interception rejects them without confirmation, leaving the browser download intact.

### Credentials, browser integration, and IPC

- Site/profile credentials, cookies, headers, and proxy passwords are encrypted by current-user Windows DPAPI. Browser cookie records are deleted after completion/cancellation.
- Custom headers use token grammar, reject CR/LF, have count/length caps, and cannot override reserved transport/auth/cookie headers.
- The MV3 native protocol has a fixed extension ID/origin, versioned schema, strict unknown-field rejection, and a 64 KiB frame limit.
- The desktop bridge binds only to `127.0.0.1`, requires a random 256-bit per-session token, and applies body/header/time limits. Browser downloads are cancelled only after durable desktop acceptance.
- The native host can launch only the adjacent fixed `FluxDM.exe` path via direct process creation. It never invokes a command shell or interpolates user input.

### Persistence, logging, and recovery

- SQLite statements are parameterized; migrations are embedded, ordered, and transactional; foreign keys and WAL are enabled.
- Corrupt databases are renamed to timestamped `.bak` files and preserved before a clean database is created. No damaged database is silently overwritten.
- Startup reconciliation compares checkpoints with `.fluxpart` state. State transitions use the central state machine.
- Logs redact authorization, cookie, password, secret, token, signature, and API-key fields plus signed-query values. Raw headers/cookies and URLs are not logged; startup no longer logs the user data path.
- **Clear private data** removes encrypted profiles/cookies, terminal download history, schedule history, and logs without deleting downloaded files.

## Validation

- Unit/integration tests cover protected servers, authenticated proxies, redirect header stripping, native framing/authentication, DPAPI opacity, destination/filename behavior, corruption recovery, segmentation hashes, and repeated pause/resume.
- Fuzz targets exercise URL parsing, filename sanitization, and native-message decoding.
- Every milestone runs formatting, vet, unit/integration tests, the Go race detector, frontend lint/typecheck/tests, and a production Wails build.
- Dependency audits use `go mod verify`, `govulncheck`, `npm audit`, and checked-in lockfiles.

## Residual risks and explicit boundaries

- ETag and Last-Modified are server-controlled consistency hints, not cryptographic content proofs. Users should verify publisher hashes/signatures for high-risk files.
- DPAPI protects data at rest from other accounts/offline access; malware already running as the same user can request decryption.
- A local same-user process that can read the per-session bridge file can submit downloads while FluxDM runs. The random token blocks web pages and other accounts, but it is not a boundary against a compromised user session.
- FluxDM does not bypass DRM, paywalls, authentication policy, browser security, or certificate validation, and it has no automatic file execution feature.
