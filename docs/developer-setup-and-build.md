# FluxDM developer setup and build guide

This guide explains how to prepare a Windows machine for FluxDM development, run the desktop application, validate a change, and produce local or release artifacts. It is written for contributors working from a source checkout.

FluxDM is a Windows desktop application. The backend is Go, the interface is React and TypeScript, Vite builds the web assets, and Wails packages both into a native Windows application.

## 1. What you need

Use a 64-bit Windows 10 or Windows 11 machine. Run the commands in PowerShell from the repository root unless a command explicitly changes directory.

| Tool | Required version or purpose | Needed for |
| --- | --- | --- |
| Git | Current supported version | Cloning and source control |
| Go | **1.26.5 or later** | Backend, Wails CLI, native host, and tests |
| Node.js | **22 or later** | Frontend dependencies, linting, tests, and Vite |
| npm | Bundled with Node.js 22 | Frontend package management |
| Wails CLI | **v2.12.0** | Desktop development and application builds |
| Microsoft Edge WebView2 Runtime | Current Evergreen Runtime | Rendering the Wails desktop window |
| GCC or Clang with CGO enabled | A C compiler available on `PATH` | Go race-detector tests only |

The following tools are only needed when building installers or signed release candidates:

| Tool | Purpose |
| --- | --- |
| NSIS / `makensis.exe` | Builds and checks the Windows installer |
| Windows SDK / `signtool.exe` | Verifies or applies Authenticode signatures |
| 7-Zip / `7z.exe` | Optional independent inspection of an installer payload |
| Code-signing certificate | Required only for a publishable, signed release |

> FluxDM uses the pure-Go SQLite driver for normal builds. A C compiler is still required for `go test -race` because Go's race detector requires CGO on Windows.

## 2. Install and verify the development tools

Install Go, Node.js 22, Git, and the Evergreen WebView2 Runtime using your organization-approved installers. Close and reopen PowerShell after installation so that their `bin` directories are on `PATH`.

Install the repository's Wails version with Go:

```powershell
go install github.com/wailsapp/wails/v2/cmd/wails@v2.12.0
```

Go installs the executable in `$(go env GOPATH)\bin`. If `wails` is not found after installation, add that directory to your user `PATH`, reopen PowerShell, and try again.

Check the tools before continuing:

```powershell
git --version
go version
node --version
npm --version
wails version
```

The root [`go.mod`](../go.mod) declares Go `1.26.0` and pins the `go1.26.5` toolchain. Use Go 1.26.5 or newer; older Go installations are not supported by this project.

### Optional release tools

Install NSIS and ensure `makensis.exe` is on `PATH` before producing an installer. Install the Windows SDK when you need `signtool.exe`. For payload verification, install 7-Zip and pass the complete `7z.exe` path to the release script rather than relying on a machine-specific PATH entry.

For race tests, install a supported GCC or Clang toolchain and make its compiler executable available on `PATH`. The CI environment uses the MSYS2 UCRT64 compiler directory, `C:\msys64\ucrt64\bin`.

## 3. Get the source and install dependencies

Clone the repository, then work from its root directory:

```powershell
git clone <repository-url> flux-dm
Set-Location flux-dm
```

Install the exact dependency versions recorded in the lockfiles:

```powershell
go mod download

Push-Location frontend
npm ci
Pop-Location
```

Use `npm ci` for a reproducible installation. It reads `frontend/package-lock.json` and will fail if the lockfile and `package.json` disagree. Use `npm install` only when you intentionally change frontend dependencies and are also updating the lockfile.

## 4. Run FluxDM in development mode

Start the Wails development server from the repository root:

```powershell
wails dev
```

Wails starts the Go application and the Vite development server, then opens the FluxDM desktop window. Changes to the React/TypeScript interface are served by Vite; Go changes cause the desktop process to rebuild or restart as directed by Wails. Stop the session with `Ctrl+C` in the terminal.

The development workflow is:

1. Keep `wails dev` running in one terminal.
2. Edit backend code under `internal/`, `cmd/`, or the root Go files, and edit UI code under `frontend/src/features/`.
3. Test the changed feature in the desktop window.
4. Run the validation commands in [Section 6](#6-validate-a-change) before handing off the change.

Do not edit `frontend/wailsjs` by hand. Wails generates these bindings from the public methods exposed by the Go application. Run `wails dev` or a Wails build after changing bindings so generated output is refreshed.

### Run only the frontend

For interface-focused work, Vite can run independently:

```powershell
Set-Location frontend
npm run dev
```

This is useful for layout work, but Wails bindings and desktop-only behavior require `wails dev` for end-to-end testing. Vite's production output goes to `dist/` at the repository root, where the Go application embeds it.

## 5. Local state, logs, and safe cleanup

On Windows, FluxDM stores its local application data beneath:

```text
%APPDATA%\FluxDM
```

Important files include `fluxdm.db` (SQLite database) and `fluxdm.log` (structured application log). The browser bridge also uses application data while FluxDM runs.

Do not commit this directory, database files, logs, build output, or `frontend/node_modules`; they are ignored by the repository. If you need to investigate a development issue, copy the database and log before changing or removing any local state. Close FluxDM first so its database and bridge resources are released.

The test suite uses its own temporary data and a local test server for deterministic download behavior. It should not require manually deleting your normal application data.

## 6. Validate a change

The project requires the following full validation set before a task is considered complete. Run Go commands from the repository root and npm commands from `frontend/`:

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

`go fmt ./...` modifies Go source files. Check `git status` afterward; a formatting-only diff means the source should be committed in formatted form. The race test needs a working compiler and `CGO_ENABLED=1`; set it for the current shell if necessary:

```powershell
$env:CGO_ENABLED = '1'
go test -race ./...
```

For broader release-oriented checks, use the release script described in [Section 8](#8-build-an-installer-or-release-candidate). It additionally verifies Go modules, scans Go dependencies, audits npm dependencies, and checks browser-extension JavaScript.

## 7. Build a local desktop executable

Create an unsigned local desktop build with:

```powershell
wails build
```

The build performs the following work:

1. Runs the Wails frontend build command, `npm run build`, from `frontend/`.
2. Type-checks the frontend as part of that command.
3. Writes optimized frontend files to `dist/`.
4. Embeds `dist/` into the Go executable through `main.go`.
5. Produces the Windows application executable in `build\bin\`, normally `build\bin\FluxDM.exe`.

For a clean, smaller local candidate, use the same flags as the release workflow:

```powershell
wails build -clean -trimpath -nocolour -ldflags '-s -w'
```

The result is unsigned and is suitable for development or local QA only. Do not publish it as a production release.

### Build the browser native host

The browser extension communicates with a small companion executable. Build it independently when testing browser integration from source:

```powershell
go build -trimpath -o build\bin\FluxDM.NativeHost.exe .\cmd\fluxdm-native-host
```

To register a source-tree development host for the current Windows user and prepare Chrome/Edge native-messaging registration, run:

```powershell
.\scripts\install-browser-integration.ps1
```

Then open `chrome://extensions` or `edge://extensions`, enable **Developer mode**, choose **Load unpacked**, and select the repository's `browser-extension` directory. In the extension options, use **Test connection**. See [`browser-extension/README.md`](../browser-extension/README.md) for the concise browser-specific steps.

## 8. Build an installer or release candidate

Use the repository script for a complete, repeatable installer build:

```powershell
.\scripts\build-release.ps1
```

This command requires Go, Node.js, Wails, a C compiler, and NSIS. It performs the following in order:

1. Checks the Go version and locates `makensis.exe` and the compiler.
2. Formats, vets, tests, race-tests, verifies modules, and runs `govulncheck`.
3. Reinstalls frontend dependencies with `npm ci`; then lints, type-checks, tests, and audits them.
4. Syntax-checks and tests the browser extension.
5. Builds `FluxDM.exe` and `FluxDM.NativeHost.exe`.
6. Creates an NSIS installer that includes WebView2 download handling.
7. Rebuilds the installer with NSIS warnings treated as errors.
8. Verifies executable version metadata and writes `build\bin\release-manifest.json` with SHA-256 hashes.

Expected unsigned artifacts are:

```text
build\bin\FluxDM.exe
build\bin\FluxDM.NativeHost.exe
build\bin\FluxDM-amd64-installer.exe
build\bin\release-manifest.json
```

To also independently inspect the contents of the installer, provide a full 7-Zip path:

```powershell
.\scripts\build-release.ps1 -SevenZipPath 'C:\Program Files\7-Zip\7z.exe'
```

### Signed production candidates

Production releases require an organization-validated Authenticode certificate installed through a protected key provider, plus an RFC 3161 timestamp service. Never put a certificate private key or signing secret in this repository, source files, scripts, or logs.

Example signing invocation:

```powershell
.\scripts\build-release.ps1 `
  -Sign `
  -CertificateThumbprint '<certificate-thumbprint>' `
  -SignToolPath 'C:\Program Files (x86)\Windows Kits\10\bin\x64\signtool.exe' `
  -MakeNSISPath 'C:\Program Files (x86)\NSIS\makensis.exe' `
  -GCCPath 'C:\msys64\ucrt64\bin\gcc.exe' `
  -SevenZipPath 'C:\Program Files\7-Zip\7z.exe'
```

The script signs both executables, rebuilds the installer so its embedded uninstaller and final installer are signed, and verifies signatures. `-AllowUntimestampedTestSignature` is only for an ephemeral local test certificate when no timestamp URL is supplied; it is never valid for a published release. See [`docs/release/code-signing.md`](release/code-signing.md) and [`docs/release/windows-smoke-checklist.md`](release/windows-smoke-checklist.md) before releasing anything.

## 9. CI and the local workflow

GitHub Actions runs on Windows and is the authoritative automated baseline. It has three relevant jobs:

| Job | What it confirms |
| --- | --- |
| `backend` | Formatting, vet, unit/integration tests, race tests, module verification, and Go vulnerability scan |
| `frontend` | Locked dependency install, lint, TypeScript checks, Vitest, npm high-severity audit, production web build, and browser-extension checks |
| `wails-build` | Desktop executable, native host, NSIS installer, and Windows version metadata |

Local validation should match this behavior as closely as practical. CI does not replace a manual desktop check for changes involving the Wails window, tray behavior, downloads, browser handoff, installer UX, or display scaling.

## 10. Common problems

| Symptom | Likely cause and fix |
| --- | --- |
| `wails` is not recognized | Run the Wails installation command, add `$(go env GOPATH)\bin` to `PATH`, and reopen PowerShell. |
| `wails dev` opens no usable window | Install or update the Microsoft Edge WebView2 Evergreen Runtime, then restart the command. |
| `go test -race ./...` fails before tests run | Install GCC or Clang, expose it on `PATH`, and set `CGO_ENABLED=1` for the shell. |
| `npm ci` fails | Use Node 22 or newer and do not modify `package.json` without updating `frontend/package-lock.json`. |
| Installer build cannot find `makensis.exe` | Install NSIS, add it to `PATH`, or pass `-MakeNSISPath` to `build-release.ps1`. |
| Extension cannot connect | Run `install-browser-integration.ps1`, load the repository `browser-extension` directory, start FluxDM, and use the extension's Test connection action. |
| A local state issue persists across runs | Exit FluxDM, preserve a copy of `%APPDATA%\FluxDM\fluxdm.db` and `fluxdm.log` for diagnosis, then investigate the copied data rather than deleting it blindly. |

## 11. Contribution boundaries

Keep changes focused on one milestone or feature. The download engine must remain independent of Wails; UI DTOs must not become database entities. Public backend methods validate inputs, database schema changes include migrations, and download-state changes go through the centralized state machine.

Frontend feature code belongs under `frontend/src/features`, TypeScript remains strict, and high-frequency download progress stays out of one large global object. Do not log credentials, authorization data, cookies, or signed URL query values. See [`AGENTS.md`](../AGENTS.md) for the complete engineering, testing, frontend, and security rules that apply to every contribution.
