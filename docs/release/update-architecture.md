# Future update architecture

FluxDM 1.0.0 does not auto-update. A future updater must keep download execution separate from installation and use this trust chain:

1. Fetch a small version manifest over HTTPS from a pinned project-owned origin.
2. Verify an offline-root-signed Ed25519 manifest before parsing artifact URLs or versions.
3. Download into a newly created user-only temporary directory with strict size/time/redirect limits.
4. Verify the SHA-256 digest from the signed manifest and a valid Microsoft Authenticode chain/timestamp on the installer.
5. Ask for explicit user confirmation, checkpoint active downloads, and launch the fixed verified installer path directly without a shell.
6. Support staged rollout, rollback metadata, and a minimum-supported-version/security revocation field.

The updater must never accept unsigned manifests, downgrade across a security floor, execute a path supplied by a remote manifest, or reuse the normal Downloads directory.

