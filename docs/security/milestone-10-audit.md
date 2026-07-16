# Milestone 10 security audit

Date: 2026-07-16 (Asia/Bangkok)

## Results

- `go test -race ./...`: passed on Windows/amd64 with Go 1.26.5 and CGO enabled.
- URL fuzz target: 164,494 executions in the recorded 5-second campaign; passed.
- Filename fuzz target: 114,119 executions; passed.
- Native-message fuzz target: 250,588 executions; passed.
- Segment-range fuzz target: 632,982 executions in the recorded 10-second campaign; passed.
- `npm audit --audit-level=high`: 0 vulnerabilities.
- `go mod verify`: all modules verified.
- `govulncheck ./...`: no reachable vulnerabilities after remediation.
- Production Wails build and native-host build: passed using Go 1.26.5.

## Findings and remediation

The first vulnerability scan found reachable standard-library advisories because the workstation was using Go 1.25.4. The machine-wide upgrade stalled in Windows Installer, so the build was moved to the official Go 1.26.5 portable distribution and `go.mod` now declares `toolchain go1.26.5`.

The follow-up scan found an imported-but-unreachable `golang.org/x/sys/windows` advisory fixed in v0.44.0. The module was upgraded from v0.36.0 to v0.44.0. The final scanner result is **No vulnerabilities found**.

Code review also found that custom profile headers could follow a redirect to a different port on the same hostname. Redirect comparison now uses the complete authority (`host:port`) and strips all profile headers on authority changes; an integration test locks this behavior.

No user-provided value is executed through a command shell. The only direct child process is the native host launching an adjacent fixed `FluxDM.exe` path when the authenticated bridge is unavailable. Installer PowerShell scripts use structured cmdlets and never use `Invoke-Expression` or construct executable command strings from browser/user input.
