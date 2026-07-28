# Production release build process

This runbook is the authoritative procedure for producing and publishing a FluxDM Windows release. It covers both the unsigned testing channel and the signed production NSIS installer build.

The signed installer is the only executable release artifact. Do not publish `FluxDM.exe` or `FluxDM.NativeHost.exe` separately: installation is required to register browser integration and create shortcuts. The matching browser-extension ZIP is published separately for store submission or portable unpacked installation.

## Release contract

| Item | Required value or rule |
| --- | --- |
| Release tag | A protected, immutable `vX.Y.Z` tag with strict numeric semver only. |
| Product version | `wails.json` `info.productVersion` must be exactly `X.Y.Z`. |
| Production publishing workflow | `.github/workflows/release.yml`, triggered only by pushed `vX.Y.Z` tags. |
| Signing boundary | The approved `release` GitHub Environment and the `self-hosted`, `windows`, `fluxdm-signing` runner. |
| Published assets | Versioned installer and browser-extension ZIP, each with an adjacent `.sha256`, plus `SHA256SUMS.txt`, `release-manifest.json`, signed `update-manifest.json`, and `update-manifest.sig`. |
| Release-candidate workflow | `.github/workflows/rc-release.yml`, triggered by `vX.Y.Z-rc.N` tags and always published as a prerelease. |
| Unsigned artifacts | May be published only through the explicit release-candidate channel; they are never production releases. |

The production workflow validates the tag strictly before it builds. A tag such as `v1.2`, `v01.2.3`, `v1.2.3-rc.1`, or a tag that does not match `wails.json` is rejected.

## Choose the release channel

Use the channel that matches the artifact's trust level. A green workflow is not itself evidence that an artifact is safe to distribute; verify the expected assets and metadata after every run.

| Topic | Unsigned release candidate | Signed production release |
| --- | --- | --- |
| Purpose | Tester feedback and installation validation before trusted signing is available. | Public, trusted distribution after approval and clean-machine acceptance. |
| Tag | `vX.Y.Z-rc.N`, where `N` is a new positive integer. | `vX.Y.Z`, matching the product version exactly. |
| Workflow | **unsigned Windows release candidate** (`rc-release.yml`). | **signed Windows release** (`release.yml`). |
| Runner | GitHub-hosted `windows-2022`. | Dedicated `self-hosted`, `windows`, `fluxdm-signing` runner. |
| Signing | Never Authenticode-signed; a preview-only metadata key signs its update manifest. | Authenticode signing with SHA-256 and an RFC 3161 timestamp, approved through the protected `release` environment. |
| GitHub release type | Prerelease. | Normal release. |
| Manifest | `version: X.Y.Z-rc.N`, `productVersion: X.Y.Z`, `signed: false`. | `version: X.Y.Z`, `productVersion: X.Y.Z`, `signed: true`. |
| Tester/user message | Windows SmartScreen or an unknown-publisher warning is expected. Do not call it trusted or production-ready. | Verify the checksum and valid Authenticode signature before announcement. |

Both channels publish eight custom assets: a versioned installer and browser-extension ZIP, each with an adjacent `.sha256` file, `SHA256SUMS.txt`, `release-manifest.json`, `update-manifest.json`, and `update-manifest.sig`. GitHub also supplies source archives separately. Missing either installer, checksum, or signed update metadata file is a failed release, even if a GitHub Release page was created.

## Unsigned release candidates while signing is unavailable

Use this channel only to give testers an installer before a trusted signing certificate or signing service is available. It is deliberately separate from the protected production path.

### 1. Prepare the exact candidate commit

1. Update and merge the intended numeric product version `X.Y.Z`.
2. Keep every packaged/user-visible version aligned before tagging:

   | Location | Required value |
   | --- | --- |
   | `wails.json` → `info.productVersion` | `X.Y.Z` |
   | `internal/application/health.go` → `Version` | `X.Y.Z` |
   | `browser-extension/manifest.json` → `version` | `X.Y.Z` |

3. Verify the merged commit locally. Replace the placeholders with the product version and the intended RC tag:

   ```powershell
   .\scripts\validate-rc-release-version.ps1 -Tag vX.Y.Z-rc.N
   .\scripts\test-release-automation.ps1
   git status --short
   git rev-parse HEAD
   ```

   `git status --short` must produce no output. Record the printed commit SHA: Actions builds the commit the tag points to, not a later commit on `main`.

4. Run the required validation suite described in [Run the required validation suite](#run-the-required-validation-suite). Do not tag a commit that has not passed it.

### 2. Create and verify the RC tag

Create an **annotated**, never-reused tag at the already-merged and validated commit. Do not omit the commit SHA; an explicit target prevents accidentally tagging an older local checkout.

   ```powershell
   $commit = '<merged-commit-sha>'
   git tag -a vX.Y.Z-rc.N $commit -m "FluxDM X.Y.Z release candidate N"
   git push origin vX.Y.Z-rc.N
   ```

Immediately confirm the remote tag resolves to the expected commit:

```powershell
git ls-remote --tags origin refs/tags/vX.Y.Z-rc.N
```

### 3. Review the workflow and published prerelease

The **unsigned Windows release candidate** workflow runs on GitHub-hosted `windows-2022`, installs the packaging tools, executes the complete validation suite, builds the installer, validates its payload hashes, and publishes a GitHub **prerelease**.

Before sharing it, verify all of the following in the workflow and GitHub Release page:

1. The workflow ref is exactly `vX.Y.Z-rc.N` and its commit SHA is the commit recorded above.
2. The workflow's staging output names the installer `FluxDM-X.Y.Z-rc.N-windows-amd64-installer.exe`.
3. The GitHub prerelease contains all eight custom assets:

   ```text
   FluxDM-X.Y.Z-rc.N-windows-amd64-installer.exe
   FluxDM-X.Y.Z-rc.N-windows-amd64-installer.exe.sha256
   FluxDM-X.Y.Z-rc.N-browser-extension.zip
   FluxDM-X.Y.Z-rc.N-browser-extension.zip.sha256
   SHA256SUMS.txt
   release-manifest.json
   update-manifest.json
   update-manifest.sig
   ```

4. `release-manifest.json` identifies the RC version, the numeric product version, `signed: false`, and both release artifacts.
5. `SHA256SUMS.txt` identifies the RC installer and browser-extension ZIP filenames exactly.

Share the installer, browser-extension ZIP, checksum files, and the explicit warning that the installer is unsigned. Testers must compare `Get-FileHash -Algorithm SHA256` output with `SHA256SUMS.txt` before running or extracting either artifact:

```powershell
$installer = '.\FluxDM-X.Y.Z-rc.N-windows-amd64-installer.exe'
Get-FileHash -Algorithm SHA256 -LiteralPath $installer
Get-FileHash -Algorithm SHA256 -LiteralPath .\FluxDM-X.Y.Z-rc.N-browser-extension.zip
Get-Content .\SHA256SUMS.txt
```

The public assets are named `FluxDM-X.Y.Z-rc.N-windows-amd64-installer.exe` and `FluxDM-X.Y.Z-rc.N-browser-extension.zip`. Its `release-manifest.json` records the release-candidate version, the packaged `X.Y.Z` product version, and `signed: false`. Windows SmartScreen or an unknown-publisher warning is expected for the installer; a checksum confirms the downloaded bytes but does **not** establish publisher identity. Do not present this as a production, trusted, or signed release.

This release-candidate workflow has no `release` environment, certificate thumbprint, timestamp endpoint, or self-hosted signing runner. It never publishes standalone executables. When signing becomes available, create a new final `vX.Y.Z` tag for the signed production workflow; do not promote or rename an unsigned release-candidate tag.

### RC failure and recovery

- If validation, packaging, or asset review fails, do not distribute the candidate. Mark the incomplete prerelease as draft or delete it according to the incident process.
- If the source must change, merge the fix, rerun validation, and create `vX.Y.Z-rc.(N+1)` at the **new** commit. Re-running an old tag always builds the old source.
- Never move, force-push, or reuse an RC tag. Preserve the failed workflow and release page as evidence.
- If a workflow creates a prerelease with only checksums/manifest and no installer, treat it as incomplete. The workflows are configured to fail on unmatched upload files; investigate the staging and upload names before creating the next RC tag.

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

### 1. Preflight the signing boundary

1. Verify that the version-change commit is merged into the target branch, the working tree is clean, and all required checks have passed.
2. Confirm the `release` environment still requires approval and exposes only `FLUXDM_CERT_THUMBPRINT` and `FLUXDM_TIMESTAMP_URL` to the workflow.
3. Confirm the dedicated runner has the expected `self-hosted`, `windows`, and `fluxdm-signing` labels, access to the certificate without exporting it, `signtool.exe`, NSIS, 7-Zip, Go, Node, Wails, and MinGW/GCC.
4. Confirm a clean Windows 10 and Windows 11 acceptance plan is ready. A signed release is not ready for announcement until the [Clean-machine acceptance](#clean-machine-acceptance) evidence is complete.

### 2. Create the final protected tag

Production tags are final numeric versions only. An RC tag never becomes the production tag, even if its installer passed testing.

```powershell
.\scripts\validate-release-version.ps1 -Tag vX.Y.Z
.\scripts\test-release-automation.ps1

$commit = '<merged-commit-sha>'
git tag -a vX.Y.Z $commit -m "FluxDM X.Y.Z"
git push origin vX.Y.Z
```

Do not create the final tag until the version metadata and source are final. Never move or replace it after publication.

### 3. Approve and monitor the production workflow

1. Open the **signed Windows release** GitHub Actions run. Confirm its ref is exactly `vX.Y.Z`, its commit SHA is the approved commit, and it is using the expected signing runner.
2. An authorized reviewer approves the `release` environment only after checking the tag, commit, release scope, runner identity, and certificate availability.
3. After approval, the workflow performs the following gates in order:

   1. Validates the tag against `wails.json` and runs the release-automation contract tests.
   2. Runs Go formatting, vet, tests, race tests, module verification, and vulnerability scanning.
   3. Runs frontend dependency installation, lint, typecheck, tests, and audit.
   4. Checks browser-extension scripts and policy tests.
   5. Builds the Wails desktop application, native host, browser-extension ZIP, and NSIS installer.
   6. Verifies Windows file version metadata against the validated version.
   7. Signs `FluxDM.exe` and `FluxDM.NativeHost.exe`, then rebuilds NSIS so the embedded uninstaller and final installer are signed and RFC 3161 timestamped.
   8. Verifies Authenticode/WinVerifyTrust signatures and extracts the NSIS payload with 7-Zip to compare hashes and signatures.
   9. Creates the versioned release assets and a non-draft GitHub Release with GitHub-generated release notes.

The signing thumbprint and timestamp URL are read from the approved environment at runtime. Do not place them on a command line, echo them, upload environment dumps, or add diagnostic tracing around the signing step. Stop immediately if the runner requests a PIN interactively, the certificate chain is unexpected, the timestamp operation fails, or any signature verification fails.

## Release assets and verification

For version `X.Y.Z`, the workflow stages exactly these files in `build\release` and uploads exactly these files to the GitHub Release:

```text
FluxDM-X.Y.Z-windows-amd64-installer.exe
FluxDM-X.Y.Z-windows-amd64-installer.exe.sha256
FluxDM-X.Y.Z-browser-extension.zip
FluxDM-X.Y.Z-browser-extension.zip.sha256
SHA256SUMS.txt
release-manifest.json
```

The adjacent checksum files and `SHA256SUMS.txt` contain the SHA-256 values of the versioned installer and browser-extension ZIP. `release-manifest.json` identifies the version, whether the build was signed, both artifact filenames, SHA-256 values, and byte counts. It must not include certificate material, credentials, or private signing information.

On a clean verification machine, download the installer and checksum file from the GitHub Release and run:

```powershell
$installer = '.\FluxDM-X.Y.Z-windows-amd64-installer.exe'
Get-FileHash -Algorithm SHA256 -LiteralPath $installer
Get-FileHash -Algorithm SHA256 -LiteralPath .\FluxDM-X.Y.Z-browser-extension.zip
Get-Content .\SHA256SUMS.txt
Get-AuthenticodeSignature -LiteralPath $installer | Format-List Status, StatusMessage, SignerCertificate, TimeStamperCertificate
signtool verify /pa /all $installer
```

The computed SHA-256 must match the entry in `SHA256SUMS.txt`, `Get-AuthenticodeSignature` must report `Valid`, and `signtool` must succeed. Do not treat an untrusted, expired, missing, or untimestamped signature as a successful release.

On the signing runner, the release build additionally verifies the installer payload, including the packaged desktop executable, native host, embedded uninstaller, and Microsoft-signed WebView2 bootstrapper. This verifies evidence that cannot be obtained from the installer hash alone.

## Clean-machine acceptance

Before announcing a new production release, execute [the Windows smoke checklist](windows-smoke-checklist.md) on fresh, fully patched Windows 10 and Windows 11 machines. Record the OS build, browser and WebView2 versions, installer SHA-256, signature chain, and outcomes in the release ticket.

At minimum, cover interactive and silent installation, desktop startup, browser integration in Chrome, Edge, and Brave, a completed download with its expected hash, uninstall with both data-retention choices, reinstall, and verification that downloaded files are never automatically executed. Include the compatibility, scaling, path, disk-space, and WebView2 cases specified by the checklist.

## Publish review and rollback

After the workflow succeeds:

1. Confirm the GitHub Release is non-draft, targets the expected tag, and has generated release notes.
2. Confirm it exposes only the eight approved assets listed above.
3. Perform the public download/hash/signature verification from a clean machine. The browser-extension ZIP is verified by hash; the installer is additionally verified by Authenticode signature.
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
