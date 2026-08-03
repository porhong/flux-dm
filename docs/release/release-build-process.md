# Portable release build process

FluxDM releases only portable Windows artifacts. An application installer, automatic installer updates, and a standalone browser-extension ZIP are not part of the release process.

Every release publishes exactly these application assets:

1. `FluxDM-X.Y.Z-windows-amd64-portable.zip`
2. Its adjacent `.sha256` file, `SHA256SUMS.txt`, and `release-manifest.json`

The portable ZIP contains the desktop executable, matching native host, and unpacked `browser-extension` folder. On startup FluxDM copies the browser files to LocalAppData and registers the current-user native host.

## Build and publish

Push `vX.Y.Z` for a stable release or `vX.Y.Z-rc.N` for a prerelease. Both workflows run on GitHub-hosted `windows-2022`, validate the tag against `wails.json`, run the validation suite, and invoke `scripts/build-portable-release.ps1`.

The generated `build/release/release-manifest.json` must contain exactly one `portable` artifact. It must not contain `installer` or `browser-extension` artifacts.

## Local release build

From a Windows development environment with Go, Node.js, MinGW/GCC, and Wails installed:

```powershell
.scriptsuild-portable-release.ps1 -Version X.Y.Z
```

For a release candidate:

```powershell
.scriptsuild-portable-release.ps1 -Version X.Y.Z -ReleaseVersion X.Y.Z-rc.N
```

Verify the generated files before sharing them:

```powershell
Get-FileHash .\build\release\FluxDM-X.Y.Z-windows-amd64-portable.zip -Algorithm SHA256
Get-Content .\build\release\SHA256SUMS.txt
Get-Content .\build\release\release-manifest.json
```

## User installation

Extract the portable ZIP to a user-writable folder and run `FluxDM.exe`. Its data is stored in the adjacent `data` directory. To enable browser handoff, use **Settings → Browser integration → Set up and open extension folder**, then load the opened folder with the browser's **Load unpacked** option.

To update, close FluxDM and replace the extracted folder with the verified newer package. Open FluxDM once to refresh the browser registration.
