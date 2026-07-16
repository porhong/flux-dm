# FluxDM

FluxDM is an original Windows download manager built with Wails, Go, React, and TypeScript. The repository implements **Milestones 0–11**, including the Windows installer and release-engineering workflow.

## Foundation

- Thin Wails desktop adapter with an independent application layer
- React 19, TypeScript strict mode, Vite, and Tailwind CSS v4
- shadcn/ui primitives for Button, Dialog, Table, Tooltip, DropdownMenu, Progress, Badge, Input, Select, Tabs, and ScrollArea
- Zustand, React Hook Form, and Zod
- SQLite through the pure-Go `modernc.org/sqlite` driver
- Embedded, ordered, transactional database migrations
- Structured JSON logging with sensitive-field and signed-query redaction
- Typed application errors and an in-process event bus
- Backend `HealthCheck` binding and `app:ready` frontend event
- URL probing with redirect, metadata, filename, and verified range-support detection
- Deterministic 1, 2, 4, 8, or 16-range planning with bounded workers and shared HTTP transports
- Preallocated `.fluxpart` files with protected concurrent `WriteAt` segment writes
- Per-segment exponential retry with jitter and safe single-stream fallback
- Slow-tail detection, idle-worker takeover, and server-overload connection adaptation
- Smoothed speed/ETA plus global and per-download bandwidth limits
- `.fluxpart` temporary files followed by atomic completion rename
- Transactional download/segment checkpoints with remote validators and startup reconciliation
- Persistent download history, throttled progress events, and reactive Pause/Resume/Retry/Restart UI
- Virtualized 10,000-row history with search, filters, bulk selection, keyboard commands, properties, and context actions
- Close-to-tray behavior, tray Show/Add/Exit commands, and native Windows completion notifications
- Deterministic extension categories, destination rules, persisted queue priority, sequential mode, aggregate queue bandwidth, and queue resource limits
- Daily/weekly queue actions, speed profiles, retry policies, missed-run handling, durable history, and explicit post-completion power actions
- Chrome/Edge MV3 interception, host/extension exclusions, and context actions through a bounded, authenticated native-messaging and loopback protocol
- Per-site Basic/Bearer authentication, opt-in browser cookies, validated custom headers, and authenticated proxy profiles protected by Windows DPAPI
- Executable warnings, sensitive-destination blocking, corruption-preserving database recovery, fuzz targets, cross-host header stripping, and privacy reset controls
- Windows 10/11 NSIS packaging with WebView2 handling, shortcuts, browser-host registration/unregistration, signing hooks, release manifests, and smoke-test automation
- Go and frontend test/lint/typecheck validation in GitHub Actions

## Prerequisites

- Windows 10 or 11
- Go 1.26.5 or newer (security baseline)
- Node.js 22 or newer
- Wails CLI v2
- Microsoft Edge WebView2 Runtime
- GCC or Clang plus `CGO_ENABLED=1` when running Go's race detector (not required for the production build)

## Development

```powershell
go mod download
Push-Location frontend
npm ci
Pop-Location
wails dev
```

The SQLite database and structured log are stored below the current user's configuration directory in `FluxDM`.

## Browser extension

An installed release registers the native host automatically. Open **FluxDM Browser Extension Setup** from the Windows Start menu, enable Developer mode at `chrome://extensions` or `edge://extensions`, choose **Load unpacked**, and select `C:\Program Files\FluxDM\FluxDM\browser-extension`. Open the extension options and choose **Test connection**.

For a source-tree development build, run `scripts\install-browser-integration.ps1` first and load this repository's `browser-extension` directory.

The isolated real-browser smoke harness temporarily registers the native host under the current user, launches FluxDM with temporary application/user data, exercises connection, direct handoff, and automatic interception, then restores the registry and removes all test state:

```powershell
.\scripts\smoke-test-browser-extension.ps1 -Browser Edge -BrowserPath 'C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe'
.\scripts\smoke-test-browser-extension.ps1 -Browser Edge -BrowserPath 'C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe' -ExpectDesktopUnavailable
```

Chrome Stable intentionally restricts command-line loading of unpacked extensions. Run the same harness with an official Chrome for Testing executable for automated Chrome coverage, and use the clean-machine checklist for the final installed Chrome manual check.

## Validation

```powershell
go fmt ./...
go vet ./...
go test ./...
go test -race ./...
Push-Location frontend
npm run lint
npm run typecheck
npm run test
Pop-Location
wails build
```

To independently inspect the final NSIS payload, pass a full 7-Zip executable:

```powershell
.\scripts\verify-installer-payload.ps1 -InstallerPath .\build\bin\FluxDM-amd64-installer.exe -SevenZipPath 'C:\Program Files\7-Zip\7z.exe'
```

Use `scripts\verify-installed-layout.ps1` to validate installed files, exact native-host/uninstall registrations, all-users shortcuts, hashes, signatures, and optionally a responsive running process without changing or uninstalling the installation.

Architecture and security notes are in [`docs/architecture`](docs/architecture) and [`docs/security`](docs/security).
Measured Milestone 1 performance is documented in [`docs/performance/milestone-1.md`](docs/performance/milestone-1.md).
Resume and crash-recovery guarantees are documented in [`docs/architecture/resume-safety.md`](docs/architecture/resume-safety.md).
Milestone 3 engine design and measured performance are documented in [`docs/architecture/segmented-engine.md`](docs/architecture/segmented-engine.md) and [`docs/performance/milestone-3.md`](docs/performance/milestone-3.md).
Milestone 4 optimization results and profiles are documented in [`docs/performance/milestone-4.md`](docs/performance/milestone-4.md).
Desktop rendering and shell behavior are documented in [`docs/architecture/desktop-interface.md`](docs/architecture/desktop-interface.md).
Category matching and queue admission are documented in [`docs/architecture/categories-and-queues.md`](docs/architecture/categories-and-queues.md).
Scheduler execution and duplicate prevention are documented in [`docs/architecture/scheduler.md`](docs/architecture/scheduler.md).
Browser extension and native-host security are documented in [`docs/architecture/browser-integration.md`](docs/architecture/browser-integration.md).
Credential storage and proxy handling are documented in [`docs/security/credentials-and-proxies.md`](docs/security/credentials-and-proxies.md).
The consolidated threat model and privacy review are in [`docs/security/threat-model.md`](docs/security/threat-model.md) and [`docs/security/privacy-review.md`](docs/security/privacy-review.md).
Release notes, signing, update architecture, diagnostics, and clean-machine validation are under [`docs/release`](docs/release).
The requirement-by-requirement status and remaining external release evidence are recorded in [`docs/release/completion-audit.md`](docs/release/completion-audit.md).

## Scope boundary

FluxDM uses an original interface and architecture. It must not bypass DRM, paywalls, authentication, browser security, or other access controls, and it must never automatically execute downloaded files.
