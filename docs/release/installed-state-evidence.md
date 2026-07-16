# Installed-state evidence

Observation date: 2026-07-16.

An interactive installation of the unsigned 1.0.0 development candidate was launched by the user through Windows Explorer on Windows 11 Pro build 26200. This was not a clean release VM and does not replace the Windows 10/11 production acceptance matrix. The validation session did not stop or uninstall the user-created installation.

## Layout and registration

- Installed path: `C:\Program Files\FluxDM\FluxDM`.
- `FluxDM.exe` SHA-256: `06960DAD21814F8EBF281914F040F7AB8B2BAC7D11CC2484BD9A3A2B458648B4` (matches the release input).
- `FluxDM.NativeHost.exe` SHA-256: `F73B4E7CAAE3D31ACCBD81746BB9512EF7E58971DD969DC5ACC4AFF48CEB888D` (matches the release input).
- The native manifest points to the installed native host and allows only `chrome-extension://hnemapnmnkccfommbacamppclohhcbfn/`.
- Chrome and Edge HKLM native-messaging registrations both point to the installed manifest.
- The actual Wails uninstall key is `HKLM\Software\Microsoft\Windows\CurrentVersion\Uninstall\FluxDMFluxDM`, with display version 1.0.0 and interactive/silent uninstall commands.
- Application, browser-extension-setup, and all-users Desktop shortcuts are present.
- The installed development executables are intentionally unsigned; this observation is not production-signing evidence.

`scripts/verify-installed-layout.ps1` reproduced these checks read-only with expected hashes and a required responsive running process.

## Installed sidebar navigation

The existing Wails window was restored from its tray-hidden state without starting another process. Window-relative clicks selected each sidebar item and captured the resulting installed WebView2 page:

| Sidebar item | Observed page heading | Result |
| --- | --- | --- |
| Downloads | Downloads | Passed |
| Categories | Categories | Passed |
| Scheduler | Scheduler | Passed |
| Settings | Settings | Passed |

The running process remained responsive and the Settings/Categories views reported `Backend Healthy`. No download, category, schedule, or setting was created or changed during the navigation check.

## Uninstall defect discovered after observation

After the read-only checks, the user independently initiated uninstall while FluxDM was still running. The old candidate removed its native-host files, registrations, uninstall key, and shortcuts, but the locked running `FluxDM.exe` remained. The process was not stopped by the validation session.

The installer source was corrected to terminate the FluxDM process tree before removing Program Files. The elevated smoke test now deliberately invokes uninstall while its test-launched app is still running and requires both process termination and complete install-directory removal. A complete unsigned release rebuild passed with corrected installer SHA-256 `E0C36374F16C61B6FE5752FAB5FE74D748CE6187618E999FCDC2B423E51AD7F6`. The corrected uninstall behavior still requires execution on an elevated clean machine.

## Follow-up corrected-candidate verification

After the corrected candidate was installed, `scripts/verify-installed-layout.ps1` was rerun read-only with the corrected release-input hashes and `-RequireRunning`. It passed: the installed application and native-host hashes match, the installed process is responsive, the fixed-origin Chrome and Edge registrations point to the installed manifest, both Start-menu shortcuts and the all-users Desktop shortcut are present, and the Wails uninstall registration is present. The candidate remains intentionally unsigned and this is still not a clean-machine or production-signing result.
