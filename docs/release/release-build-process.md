# Portable release build process

FluxDM releases only portable Windows artifacts. An application installer, automatic installer updates, and a standalone browser-extension ZIP are not part of the release process.

Every release publishes exactly these application assets:

1. `FluxDM-X.Y.Z-windows-amd64-portable.exe`
2. `FluxDM-X.Y.Z-windows-amd64-portable-browser-integration.zip`
3. The adjacent `.sha256` file for each asset, `SHA256SUMS.txt`, and `release-manifest.json`

The browser-integration ZIP contains the matching native host, registration scripts, and unpacked `browser-extension` folder. It must be extracted beside the portable EXE before registering the native host.

## Build and publish

Push `vX.Y.Z` for a stable release or `vX.Y.Z-rc.N` for a prerelease. Both workflows run on GitHub-hosted `windows-2022`, validate the tag against `wails.json`, run the validation suite, and invoke `scripts/build-portable-release.ps1`.

The generated `build/release/release-manifest.json` must contain exactly two artifacts with kinds `portable` and `portable-browser-integration`. It must not contain `installer` or `browser-extension` artifacts.

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
Get-FileHash .\build\release\FluxDM-X.Y.Z-windows-amd64-portable.exe -Algorithm SHA256
Get-FileHash .\build\release\FluxDM-X.Y.Z-windows-amd64-portable-browser-integration.zip -Algorithm SHA256
Get-Content .\build\release\SHA256SUMS.txt
Get-Content .\build\release\release-manifest.json
```

## User installation

Save the portable EXE in a user-writable folder and run it. Its data is stored in the adjacent `data` directory. To enable browser handoff, extract the matching browser-integration ZIP in that same folder, run `scripts\install-browser-integration.ps1`, and load the included `browser-extension` folder with the browser's **Load unpacked** option. Keep exactly one versioned portable EXE in the folder.

To update, close FluxDM and replace the portable EXE with the verified newer version. Rerun the browser-integration install script after replacing it.
