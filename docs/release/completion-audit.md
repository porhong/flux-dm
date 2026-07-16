# Milestones 4–11 completion audit

Audit date: 2026-07-16. The milestone plan attached to the project is the authority for this ledger. “Proven” means the requirement has direct source, automated-test, measured-report, build, or runtime evidence. A clean-machine or signing item is not marked proven from static inspection.

## Milestone 4 — dynamic segmentation and optimization

Status: **proven locally**.

- Dynamic tail splitting, idle-worker reuse, bounded adaptation, retry control, speed smoothing, ETA, and global/per-download limiting live in `internal/download`.
- Integration tests prove slow-tail splitting without corruption, shared/global and per-download rate bounds, server-overload adaptation, checksum correctness, and fallback behavior.
- `docs/performance/milestone-4.md` records buffer benchmarks, throughput, allocation, CPU/goroutine observations, when segmentation helps, and when it does not. CPU and heap profiles are committed under `docs/performance/profiles`.

## Milestone 5 — main desktop interface

Status: **proven locally; installed-machine interaction is included in the Milestone 11 smoke run**.

- React implements navigation, add/properties dialogs, filtering, context actions, bulk selection, error/empty states, tray actions, and Windows notifications.
- A 10,000-row test proves a bounded rendered window. Per-download external progress signals and memoized rows isolate high-frequency updates.
- UI tests cover sidebar navigation and keyboard commands including Space, Enter, `P`, Delete, Ctrl+A, and Ctrl+N. CSS includes visible focus and reduced-motion handling.
- A user-initiated installation of the unsigned candidate on Windows 11 build 26200 was inspected in place. The running installed WebView2 window successfully navigated to Downloads, Categories, Scheduler, and Settings via its actual sidebar, directly covering the previously reported installed-EXE navigation failure. See `docs/release/installed-state-evidence.md`.

## Milestone 6 — categories and queues

Status: **proven locally**.

- Numbered migrations and repositories persist categories, queues, assignments, priority, sequential mode, concurrency, connection caps, and queue speed limits.
- Deterministic category tests prove normalized extension matching and stable priority resolution. Dispatcher tests prove priority and capacity behavior.
- Queue bandwidth now uses one shared engine limiter, and a concurrent integration test proves the configured rate is an aggregate queue budget rather than a per-download multiplier.

## Milestone 7 — scheduler and automation

Status: **proven locally, except real sleep/hibernate/shutdown effects are intentionally reserved for the clean-machine manual run**.

- Daily/weekly evaluation, queue start/stop, speed profiles, retries, missed-run policy, history, post-completion exit/power actions, and durable claims are implemented and persisted.
- Repository and service tests prove restart persistence primitives and duplicate prevention. Scheduler tests prove missed-run policy.
- The Wails/backend boundary rejects sleep, hibernate, and shutdown schedules unless the request carries explicit confirmation; UI confirmation alone is not trusted.

## Milestone 8 — Chrome and Edge integration

Status: **protocol, packaging, and isolated real-browser behavior proven locally; installed clean-machine behavior remains in the release acceptance run**.

- The version 1.0.0 Manifest V3 extension has a fixed ID, context-menu handoff, automatic interception, hostname and extension exclusions, connection status, and opt-in cookies.
- Browser cancellation occurs only after an accepted native response. Policy tests cover host/subdomain and filename/URL extension exclusions.
- Native framing, exact origin, message bounds, unknown fields, authenticated loopback bridge, and accept-after-submit behavior have Go tests and fuzz coverage.
- The installer packages the extension and creates a Start-menu setup guide. Strict NSIS compilation proves the fixed-origin native-host manifest and Chrome/Edge registrations compile without warnings.
- The isolated real-browser harness passed in Microsoft Edge 150.0.4078.65 and official Chrome for Testing 150.0.7871.114. In both engines, the extension connected through native messaging, completed a direct 262,144-byte FluxDM transfer with the expected SHA-256, intercepted a genuine browser download, completed the second FluxDM transfer with the expected SHA-256, and observed the browser item as `interrupted` / `USER_CANCELED` only after acceptance. See `docs/release/browser-smoke-evidence.md`.
- The same two engines passed the unavailable-desktop case with a host-only installation: the extension reported `Not connected`, did not cancel the browser item, and Chromium completed a 262,144-byte browser-owned file with the expected SHA-256. This provides real-browser evidence for both the accepted takeover and rejection fallback branches.

## Milestone 9 — authentication, cookies, and proxy

Status: **proven locally**.

- Site profiles support Basic/Bearer authentication, validated custom headers, cookies, HTTP(S) proxy settings, and proxy authentication.
- Integration tests complete protected downloads using Basic auth and using Bearer auth plus cookies/custom headers; a separate authenticated-proxy integration test proves proxy credentials are applied.
- Windows DPAPI round-trip tests prove opaque ciphertext. DTOs expose only secret-presence metadata, and clear-secret/private-data operations are explicit.

## Milestone 10 — reliability and security hardening

Status: **proven locally**.

- Threat model, privacy review, executable confirmation, URL/path/redirect controls, database corruption backup/recovery, log redaction, and dependency review are implemented and documented.
- Race tests pass with Go 1.26.5, CGO enabled, and MinGW GCC. The release script now makes that compiler gate explicit.
- Filename, URL, native-message, and segment-range fuzzers cover the named unsafe-input boundaries. The segment planner fuzzer checks full contiguous coverage, ordering, checkpoints, gaps, and overlaps for arbitrary sizes and supported connection counts.
- `go mod verify`, `govulncheck`, and `npm audit` are release gates. No user-controlled string is executed through a shell.

## Milestone 11 — installer and release

Status: **implementation and unsigned release-candidate pipeline proven; production acceptance not yet proven**.

- NSIS packages FluxDM, downloads/detects WebView2, registers/unregisters the native host, creates application and extension-setup shortcuts, and provides an uninstaller.
- Uninstall preserves data by default. Interactive cleanup removes only known FluxDM database/log/bridge/recovery files; unknown files and downloads remain outside recursive deletion.
- Crash logging, release notes, update-ready architecture, signing/verification scripts, checksum manifest, and clean Windows checklist are present.
- The one-command release pipeline passes formatting, vet, unit/integration tests, race tests, module verification, vulnerability audits, frontend validation, extension policy tests, Wails production build, warning-as-error NSIS compilation, PE version checks, and manifest generation.
- The final NSIS artifact was independently extracted with 7-Zip 26.02. All packaged application/native-host and extension files match their release-input SHA-256 hashes, the embedded uninstaller is present, and the packaged WebView2 bootstrapper has a valid Microsoft Corporation Authenticode signature. `scripts/verify-installer-payload.ps1` makes this reproducible and can require valid production signatures for the installer plus all three embedded executables.
- The user-initiated Windows 11 installation has the expected application/native-host hashes, fixed native-host manifest/origin, Chrome and Edge HKLM registrations, Wails uninstall registration, all-users shortcuts, responsive desktop window, and working sidebar. `scripts/verify-installed-layout.ps1` provides a non-destructive repeatable check. Uninstall behavior was deliberately not exercised against user-created installed state.
- A subsequent user-initiated uninstall of the old candidate while FluxDM was running exposed a locked-executable defect. The corrected uninstaller terminates the FluxDM/WebView2 process tree before Program Files removal, and the elevated smoke runner now makes uninstall-while-running a required regression check. The complete unsigned release pipeline and extracted-payload verifier pass for the corrected installer; elevated execution remains part of the external clean-machine matrix.
- The corrected unsigned candidate was subsequently installed and passed the read-only installed-layout verifier with its expected hashes and `-RequireRunning`: the installed process responded, native-host manifest/registrations and shortcuts were exact, and the Wails uninstall registration was present. This verifies corrected-candidate installation layout but does not exercise its elevated uninstaller.
- The complete signing path was exercised with a temporary locally trusted code-signing certificate in the explicitly test-only untimestamped mode. Both application executables, the NSIS embedded uninstaller, and the final installer were signed; all top-level artifacts passed Authenticode and WinVerifyTrust verification, and the signed manifest was generated. The temporary certificate/private key and signed test outputs were removed afterward, and the original unsigned candidates were restored byte-for-byte.

The following acceptance evidence is still external and therefore **not complete**:

1. Sign `FluxDM.exe`, `FluxDM.NativeHost.exe`, the embedded uninstaller, and the installer with a trusted Authenticode certificate and RFC 3161 timestamp, then pass `scripts/verify-release.ps1`.
2. Run `docs/release/windows-smoke-checklist.md` on fresh, fully patched Windows 10 22H2 and Windows 11 machines, including interactive/silent install, sidebar, transfers/recovery, Chrome and Edge, both uninstall data choices, reinstall, display scaling/light/dark/mixed-DPI behavior, long and non-ASCII/Khmer paths, constrained disk space, absent/outdated WebView2, and signature-chain recording.

Unsigned artifacts are development candidates only and must not be published as the 1.0.0 production release.

The local signing-path test does not satisfy production acceptance: its certificate was self-signed and its signatures intentionally had no RFC 3161 timestamp. A production certificate and clean Windows 10/11 machines are external release inputs, not artifacts that can be generated from this repository.
