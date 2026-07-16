# Windows 10/11 clean-machine smoke checklist

Run on fresh, fully patched Windows 10 22H2 and Windows 11 virtual machines with no development toolchain installed.

From a normal PowerShell session, the following command requests elevation, refuses any machine/profile that already contains FluxDM state, verifies the expected installer hash, and writes a JSON evidence report:

```powershell
.\scripts\start-elevated-installer-smoke.ps1 -InstallerPath .\build\bin\FluxDM-amd64-installer.exe -ExpectedInstallerSHA256 E0C36374F16C61B6FE5752FAB5FE74D748CE6187618E999FCDC2B423E51AD7F6 -ReportPath .\build\bin\installer-smoke-report.json
```

Add `-RequireSignatures` for production candidates. The production run must use the signed candidate's hash rather than the unsigned development hash shown above.

For a non-destructive check of an installation that must remain in place, run `scripts\verify-installed-layout.ps1` with the expected application/native-host hashes. Add `-RequireRunning` to require a responsive process and `-RequireSignatures` for production installations.

- Verify installer Authenticode signature in Properties and with `signtool verify /pa /all`.
- Install interactively and silently; confirm WebView2 is detected or installed, FluxDM launches, sidebar navigation works, and Start/Desktop shortcuts open the app. Select the optional **Start FluxDM when I sign in** component once, verify the current-user Run value, and confirm uninstall removes it.
- Add a local test-server download, then exercise pause, exit, restart recovery, resume, completion hash, tray hide/show, and notification.
- Create category/queue/schedule/profile records, restart, and confirm persistence. Verify an executable download requires acknowledgement.
- Load the packaged extension in Chrome and Edge, test native connectivity, context-menu handoff, eligible interception, exclusion fallback, malformed-message rejection, and opt-in cookie transfer.
- Verify native-host manifest paths/registrations point into the install directory and contain only the fixed extension origin.
- Run the silent uninstaller. Confirm shortcuts and Chrome/Edge registry keys are removed, while completed downloads and `%APPDATA%\FluxDM` data remain. Reinstall, uninstall interactively with **Remove FluxDM settings…** selected, and confirm known database/log/bridge files are removed while an unknown sentinel file and completed downloads survive.
- Reinstall and confirm preserved data migrates/opens. Corrupt a copied test database and confirm FluxDM preserves a timestamped backup before starting clean.
- Exercise the desktop UI at 100%, 125%, 150%, and 200% display scaling in both Windows light and dark modes. Confirm dialogs remain reachable, text and icons do not clip, keyboard focus stays visible, and the virtualized download table remains usable.
- Move the running window between monitors with different DPI settings (where available), disconnect/reconnect the secondary display, and confirm the window returns on-screen with usable sizing.
- Complete downloads whose destination path is long and whose filename contains non-ASCII text, including Khmer characters. Confirm the final file name, size, and SHA-256 are exact and that restart recovery retains the path.
- Exercise constrained free-space handling with a disposable volume or dynamically sized test disk. Confirm the download fails safely with an actionable error, retains resumable state when possible, and does not report a corrupt partial file as complete.
- On disposable VM snapshots, test WebView2 absent and an outdated WebView2 runtime. Confirm the installer offers/executes the bootstrap path, handles bootstrap failure visibly, and launches successfully after the supported runtime is present.
- Record OS build, WebView2 version, browser versions, installer hash, signer chain, and results in the release ticket.

`scripts/smoke-test-installer.ps1` automates the install/file/exact-registry/desktop-window/uninstall/non-deletion core and records OS build, hashes, and signature state. `scripts/smoke-test-browser-extension.ps1` automates isolated native connectivity, direct handoff, automatic interception, final-file hashes, browser cancellation after acceptance, and browser-owned completion when `-ExpectDesktopUnavailable` is used. Installed Chrome Stable, interactive extension loading, context-menu UX, and the remaining interactive desktop checks stay explicit checklist items.
