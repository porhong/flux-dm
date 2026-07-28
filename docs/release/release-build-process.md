# Production release build process

This runbook is the authoritative procedure for producing and publishing a FluxDM Windows release. It covers both the unsigned testing channel and the signed production NSIS installer build.

Only the signed installer is a release artifact. Do not publish `FluxDM.exe` or `FluxDM.NativeHost.exe` separately: installation is required to register browser integration and create shortcuts.

## Release contract

| Item | Required value or rule |
| --- | --- |
| Release tag | A protected, immutable `vX.Y.Z` tag with strict numeric semver only. |
| Product version | `wails.json` `info.productVersion` must be exactly `X.Y.Z`. |
| Production publishing workflow | `.github/workflows/release.yml`, triggered only by pushed `vX.Y.Z` tags. |
| Signing boundary | The approved `release` GitHub Environment and the `self-hosted`, `windows`, `fluxdm-signing` runner. |
| Published assets | Versioned installer, its `.sha256`, `SHA256SUMS.txt`, and `release-manifest.json`. |
| Release-candidate workflow | `.github/workflows/rc-release.yml`, triggered by `vX.Y.Z-rc.N` tags and always published as a prerelease. |
| Unsigned artifacts | May be published only through the explicit release-candidate channel; they are never production releases. |

The production workflow validates the tag strictly before it builds. A tag such as `v1.2`, `v01.2.3`, `v1.2.3-rc.1`, or a tag that does not match `wails.json` is rejected.

## Unsigned release candidates while signing is unavailable

Use this channel only to give testers an installer before a trusted signing certificate or signing service is available. It is deliberately separate from the protected production path.

1. Update and merge `wails.json` to the desired numeric product version `X.Y.Z`.
2. Create a new release-candidate tag with a positive sequence number. Do not reuse a release-candidate tag:

   ```powershell
   git tag vX.Y.Z-rc.N <merged-commit-sha>
   git push origin vX.Y.Z-rc.N
   ```

3. The **unsigned Windows release candidate** workflow runs on GitHub-hosted `windows-2022`, installs the build tools, executes the complete validation suite, builds the installer, validates its payload hashes, and publishes a GitHub **prerelease**.
4. Share only the installer, checksum files, and the explicit warning that it is unsigned. Testers must compare `Get-FileHash -Algorithm SHA256` output with `SHA256SUMS.txt` before running it.

The public asset is named `FluxDM-X.Y.Z-rc.N-windows-amd64-installer.exe`. Its `release-manifest.json` records the release-candidate version, the packaged `X.Y.Z` product version, and `signed: false`. Windows SmartScreen or an unknown-publisher warning is expected; a checksum confirms the downloaded bytes but does **not** establish publisher identity. Do not present this as a production, trusted, or signed release.

This release-candidate workflow has no `release` environment, certificate thumbprint, timestamp endpoint, or self-hosted signing runner. It never publishes standalone executables. When signing becomes available, create a new final `vX.Y.Z` tag for the signed production workflow; do not promote or rename an unsigned release-candidate tag.

## One-time production setup

### Protect the GitHub release path

1. Protect the `v*` tag pattern so only authorized release managers can create a release tag.
2. Create the repository GitHub Environment named `release`.
3. Require reviewer approval for that environment.
4. Give the workflow `contents: write` permission and do not grant extra repository permissions.
5. Configure these environment secrets only in `release`:

   - `FLUXDM_CERT_THUMBPRINT`: the thumbprint of the production Authenticode certificate.
   - `FLUXDM_TIMESTAMP_URL`: the RFC 3161 timestamp endpoint.

The certificate's PFX, private key, token, PIN, and signing-service credential must never be stored in the repository, GitHub secrets, release assets, or logs. A thumbprint identifies a certificate; it is not certificate material.

### Prepare the signing runner

Register and protect a dedicated Windows runner with all of these labels:

```text
self-hosted
windows
fluxdm-signing
```

The runner must have:

- Go at the version declared in `go.mod` (the release script also requires Go 1.26.5 or newer).
- Node.js 22 or newer.
- Wails CLI v2.12.0. The workflow installs the pinned CLI, but the runner must allow Go tools on `PATH`.
- MinGW/GCC for CGO race tests.
- NSIS, with `makensis.exe` resolvable on `PATH`.
- Windows SDK signing tools, with `signtool.exe` resolvable on `PATH`.
- 7-Zip at `C:\Program Files\7-Zip\7z.exe`.
- Microsoft Edge WebView2 build prerequisites.
- Access to the production certificate through a hardware-backed token, HSM, or OS-backed certificate-store provider. The runner service identity must be allowed to use the private key without exporting it.

Keep this runner dedicated to signing. Limit interactive access, keep it patched, and do not install untrusted software or browser extensions on it. Before the first production release, use a test certificate in a non-production environment to prove that `signtool`, NSIS finalization, and 7-Zip payload extraction work together. Never use an untimestamped test signature for publication.

## Prepare a version change

1. Start from the intended release commit and confirm the working tree contains no unrelated changes.
2. Update `wails.json` `info.productVersion` to the target `X.Y.Z` version.
3. Keep all user-visible and packaged version metadata synchronized. In particular, update `internal/application/health.go` and `browser-extension/manifest.json` when their versions change. The payload verifier requires the extension version to match the product version.
4. Add or update release notes for the target version.
5. Commit and merge the version change before creating its tag. Do not modify version metadata after the release tag exists.

Confirm the release contract locally:

```powershell
.\scripts\validate-release-version.ps1 -Tag vX.Y.Z
.\scripts\test-release-automation.ps1
```

The first command prints `X.Y.Z` only when the tag and `wails.json` match. The second checks the version/tag rules, release asset naming, checksum generation, manifest content, and required workflow contracts.

## Run the required validation suite

Run these commands from the repository root before requesting a production tag. Run the npm commands from `frontend`.

```powershell
Push-Location frontend
npm ci
npm run build
Pop-Location

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

All commands must pass. Do not waive a failing race, integration, lint, typecheck, test, or Wails build check for a production release. Record the commit SHA and validation results in the release ticket.

For a candidate installer with the additional release-only checks, run:

```powershell
.\scripts\build-release.ps1 -Version X.Y.Z -SevenZipPath 'C:\Program Files\7-Zip\7z.exe'
```

This unsigned command is useful for local QA only. It must not be uploaded as a GitHub Release asset.

## Signed build and workflow execution

The production workflow is the publishing mechanism. It is intentionally not manually dispatchable.

1. Verify that the version-change commit is merged into the target branch and all required checks have passed.
2. Create and push the matching protected tag:

   ```powershell
   git tag vX.Y.Z <merged-commit-sha>
   git push origin vX.Y.Z
   ```

3. Open the **signed Windows release** GitHub Actions run. Confirm its ref is exactly `vX.Y.Z` and it is using the expected signing runner.
4. An authorized reviewer approves the `release` environment only after checking the tag, commit, release scope, and runner identity.
5. After approval, the workflow performs the following gates in order:

   1. Validates the tag against `wails.json` and runs the release-automation contract tests.
   2. Runs Go formatting, vet, tests, race tests, module verification, and vulnerability scanning.
   3. Runs frontend dependency installation, lint, typecheck, tests, and audit.
   4. Checks browser-extension scripts and policy tests.
   5. Builds the Wails desktop application, native host, and NSIS installer.
   6. Verifies Windows file version metadata against the validated version.
   7. Signs `FluxDM.exe` and `FluxDM.NativeHost.exe`, then rebuilds NSIS so the embedded uninstaller and final installer are signed and RFC 3161 timestamped.
   8. Verifies Authenticode/WinVerifyTrust signatures and extracts the NSIS payload with 7-Zip to compare hashes and signatures.
   9. Creates the versioned release assets and a non-draft GitHub Release with GitHub-generated release notes.

The signing thumbprint and timestamp URL are read from the approved environment at runtime. Do not place them on a command line, echo them, upload environment dumps, or add diagnostic tracing around the signing step.

## Release assets and verification

For version `X.Y.Z`, the workflow stages exactly these files in `build\release` and uploads exactly these files to the GitHub Release:

```text
FluxDM-X.Y.Z-windows-amd64-installer.exe
FluxDM-X.Y.Z-windows-amd64-installer.exe.sha256
SHA256SUMS.txt
release-manifest.json
```

The checksum file and `SHA256SUMS.txt` contain the SHA-256 of the versioned installer. `release-manifest.json` identifies the version, whether the build was signed, the installer filename, SHA-256, and byte count. It must not include certificate material, credentials, or private signing information.

On a clean verification machine, download the installer and checksum file from the GitHub Release and run:

```powershell
$installer = '.\FluxDM-X.Y.Z-windows-amd64-installer.exe'
Get-FileHash -Algorithm SHA256 -LiteralPath $installer
Get-Content .\SHA256SUMS.txt
Get-AuthenticodeSignature -LiteralPath $installer | Format-List Status, StatusMessage, SignerCertificate, TimeStamperCertificate
signtool verify /pa /all $installer
```

The computed SHA-256 must match the entry in `SHA256SUMS.txt`, `Get-AuthenticodeSignature` must report `Valid`, and `signtool` must succeed. Do not treat an untrusted, expired, missing, or untimestamped signature as a successful release.

On the signing runner, the release build additionally verifies the installer payload, including the packaged desktop executable, native host, embedded uninstaller, and Microsoft-signed WebView2 bootstrapper. This verifies evidence that cannot be obtained from the installer hash alone.

## Clean-machine acceptance

Before announcing a new production release, execute [the Windows smoke checklist](windows-smoke-checklist.md) on fresh, fully patched Windows 10 and Windows 11 machines. Record the OS build, browser and WebView2 versions, installer SHA-256, signature chain, and outcomes in the release ticket.

At minimum, cover interactive and silent installation, desktop startup, browser integration in Chrome and Edge, a completed download with its expected hash, uninstall with both data-retention choices, reinstall, and verification that downloaded files are never automatically executed. Include the compatibility, scaling, path, disk-space, and WebView2 cases specified by the checklist.

## Publish review and rollback

After the workflow succeeds:

1. Confirm the GitHub Release is non-draft, targets the expected tag, and has generated release notes.
2. Confirm it exposes only the four approved assets listed above.
3. Perform the public download/hash/signature verification from a clean machine.
4. Link the workflow run, smoke-test evidence, checksums, and release notes in the release ticket.
5. Announce the release only after those checks are complete.

If a release must be withdrawn, mark the GitHub Release as draft or remove it according to the incident process, communicate the affected version and reason, and publish a corrected version under a new tag. Do not replace or move a protected release tag after publication; preserve the original evidence and use a subsequent patch version instead.

## Failure handling

| Failure | Required response |
| --- | --- |
| Tag/version mismatch | Stop. Correct and merge version metadata, then create a new matching tag. |
| Environment approval unavailable | Do not bypass it or copy secrets to another environment. Restore the configured approval path. |
| Certificate unavailable or signature invalid | Stop publication. Investigate the certificate-store access, chain, timestamp service, and runner identity without exporting the private key. |
| NSIS, 7-Zip, or payload verification failure | Do not publish. Repair the runner/tooling or packaging defect and rerun from a new approved release tag if the tagged source changes. |
| Validation suite failure | Fix the defect and repeat all required validation before tagging the corrected commit. |
| Public checksum or signature mismatch | Immediately withdraw the release, preserve logs and artifacts for investigation, and publish a new version only after the trust chain is verified. |

For implementation details of the signing and payload checks, see [code-signing.md](code-signing.md). For local developer setup and unsigned candidates, see [developer setup and build](../developer-setup-and-build.md).
