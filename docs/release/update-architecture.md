# Update architecture

FluxDM checks the pinned `porhong/flux-dm` GitHub Releases endpoint after startup and then once every 24 hours while the desktop process is running, including when its window is hidden in the tray. Discovery data is untrusted: the application only accepts a release after verifying its detached Ed25519 `update-manifest.sig` before parsing update metadata.

Portable builds intentionally do not initialize this installer updater. They are updated by downloading a verified newer portable EXE and replacing the previous executable after it exits; this avoids self-replacing an executable in use and does not execute any downloaded artifact.

## Trust and channels

- **Stable** is the default. It accepts only non-prerelease releases signed with the compiled stable metadata public key, then verifies the installer SHA-256, byte count, and Windows Authenticode trust chain before enabling installation.
- **Preview** is opt-in. It may discover a prerelease signed by the compiled preview metadata public key. Preview installers remain unsigned, are never background-downloaded, and need an explicit unsigned-installer confirmation immediately before launch.
- Production and preview use independent Ed25519 key pairs. The public keys are injected into release builds; private keys remain GitHub Actions secrets. Compromise of the preview key cannot authorize an update for Stable users.

`update-manifest.json` contains a strict release version, numeric product version, channel, signed flag, minimum supported version, release notes URL, and the installer filename/SHA-256/size. The release workflow signs the exact UTF-8 bytes and publishes the manifest and Base64 detached signature as release assets.

## Installation flow

Installers stream to `%AppData%\FluxDM\updates`, never the user Downloads folder. The service limits metadata and installer sizes, follows HTTPS redirects only, hashes the streamed file, atomically promotes a verified file, and stores only its local path. It never executes a URL or remote-provided path.

After the user chooses **Restart and install**, FluxDM copies its packaged launcher into the private update cache, exits, and that helper waits for the parent before directly starting the verified NSIS installer. The helper restarts the fixed installed FluxDM executable only after a successful installer exit. No shell is used.

## Release setup and response

Configure `FLUXDM_UPDATE_STABLE_PRIVATE_KEY` in the protected `release` environment and inject the corresponding stable public key into the signed installer build. Portable RC builds do not receive update keys or publish updater metadata. Private values are Base64 Ed25519 seeds or private keys and must never be logged, committed, or published.

If a key is suspected compromised, withdraw affected releases, rotate the relevant key pair, ship the new public key through an already trusted release, and issue a new immutable release tag. Do not move an existing tag or replace an existing release asset.
