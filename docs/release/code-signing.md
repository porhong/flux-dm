# Release integrity

The active FluxDM release pipeline is portable-only. It does not build or publish an installer and therefore has no installer-signing or installer-update requirements.

Each release publishes a SHA-256 checksum file and `release-manifest.json` for the self-contained portable ZIP. Verify the downloaded package against these checksums before extracting. See [release-build-process.md](release-build-process.md) for the current process.
