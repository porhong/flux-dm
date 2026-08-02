# Release integrity

The active FluxDM release pipeline is portable-only. It does not build or publish an installer and therefore has no installer-signing or installer-update requirements.

Each release publishes SHA-256 checksum files and `release-manifest.json` for the portable EXE and matching portable browser-integration ZIP. Verify the downloaded files against these checksums before use. See [release-build-process.md](release-build-process.md) for the current process.
