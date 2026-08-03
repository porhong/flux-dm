# Portable update policy

FluxDM releases are portable-only. Portable builds do not initialize or use the legacy installer updater, and releases do not publish installer update manifests or signatures.

To update FluxDM, download the newer portable ZIP, verify its SHA-256 against the matching release checksum, close FluxDM, and replace the extracted folder. Open FluxDM once afterward to refresh its current-user browser registration. No downloaded artifact is automatically executed.
