# Portable update policy

FluxDM releases are portable-only. Portable builds do not initialize or use the legacy installer updater, and releases do not publish installer update manifests or signatures.

To update FluxDM, download the newer portable EXE, verify its SHA-256 against the matching release checksum, close FluxDM, and replace the old EXE. If browser handoff is enabled, rerun the browser-integration registration script after replacing the executable. No downloaded artifact is automatically executed.
