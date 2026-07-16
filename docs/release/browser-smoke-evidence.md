# Browser extension smoke evidence

Validation date: 2026-07-16.

The automated browser harness was run with isolated browser profile, `%APPDATA%`, `%USERPROFILE%`, and Downloads directories. It temporarily installed only the selected browser's `HKCU` native-messaging registration, used extension ID `hnemapnmnkccfommbacamppclohhcbfn`, and restored the registry and removed every temporary directory/process after each run.

## Results

| Browser | Version | Native connection | Direct FluxDM transfer | Automatic interception | Browser result |
| --- | --- | --- | --- | --- | --- |
| Microsoft Edge | 150.0.4078.65 | Connected | 262,144 bytes, SHA-256 `31a1f9dea0169551092d05e8bf4a446228c8c3eb4c9b713c66adcb7fd53c89be` | 262,144 bytes, SHA-256 `48511837ebc5e84cc7ff34d73794b59f205aff0bb7eb0349cb006540a67fa6d1` | `interrupted` / `USER_CANCELED` |
| Chrome for Testing | 150.0.7871.114 | Connected | 262,144 bytes, SHA-256 `31a1f9dea0169551092d05e8bf4a446228c8c3eb4c9b713c66adcb7fd53c89be` | 262,144 bytes, SHA-256 `48511837ebc5e84cc7ff34d73794b59f205aff0bb7eb0349cb006540a67fa6d1` | `interrupted` / `USER_CANCELED` |

Both engines were also run with `-ExpectDesktopUnavailable`, which copied only the native host into a temporary host directory so it could not launch an adjacent desktop executable. Each extension options page reported `Not connected: Open FluxDM and try again.` The genuine browser download remained in browser ownership, reached state `complete` with no error, and produced 262,144 bytes with SHA-256 `48511837ebc5e84cc7ff34d73794b59f205aff0bb7eb0349cb006540a67fa6d1`.

The transfer server observed six HTTP requests for the direct handoff and seven for automatic interception in each browser, covering FluxDM probing and segmented range requests. The harness checked the final file lengths and hashes rather than treating request receipt as completion evidence.

## Commands

```powershell
.\scripts\smoke-test-browser-extension.ps1 -Browser Edge -BrowserPath 'C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe' -TimeoutSeconds 30
.\scripts\smoke-test-browser-extension.ps1 -Browser Edge -BrowserPath 'C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe' -ExpectDesktopUnavailable -TimeoutSeconds 30
.\scripts\smoke-test-browser-extension.ps1 -Browser Chrome -BrowserPath '<Chrome for Testing>\chrome.exe' -TimeoutSeconds 30
.\scripts\smoke-test-browser-extension.ps1 -Browser Chrome -BrowserPath '<Chrome for Testing>\chrome.exe' -ExpectDesktopUnavailable -TimeoutSeconds 30
```

Google documents Chrome for Testing as its reproducible automation build and documents new-headless extension testing. Chrome Stable 150 on the validation host blocked command-line loading of the unpacked extension with `ERR_BLOCKED_BY_CLIENT`; the harness did not bypass that product policy. The installed Chrome Stable flow therefore remains an explicit manual item on each clean release machine.

This evidence proves the source-tree extension/native-host/desktop path in real Chrome and Edge engines. It does not substitute for verifying installer-created files and registry entries on clean Windows 10 and Windows 11 machines.
