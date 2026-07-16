# Browser integration

The Chrome/Edge Manifest V3 extension uses native messaging host `com.fluxdm.browser`. Its embedded public key fixes the extension ID at `hnemapnmnkccfommbacamppclohhcbfn`, and the native-host manifest allows only that origin.

Automatic interception policy is stored in browser sync settings and supports both hostname and file-extension exclusions. Explicit context-menu handoff remains available even when an automatic rule excludes the URL. Cookie sharing is separately opt-in.

Native messages are little-endian length-prefixed JSON capped at 64 KiB. The decoder rejects unknown fields, invalid versions, malformed IDs, non-HTTP(S) URLs, embedded credentials, and oversized filenames. The host never logs URLs, referrers, headers, or session tokens.

The native process forwards requests to `127.0.0.1` only. FluxDM generates a random 256-bit token on each desktop start and publishes the port/token file with user-only permissions. Every bridge request needs the token and the HTTP server has strict header/body/time limits. If FluxDM is closed, the installed native host launches the adjacent signed desktop executable and retries briefly.

For an intercepted browser download, the extension keeps the browser transfer alive while the request is checked. It cancels the browser transfer only after the desktop service has validated the request, reserved a destination, committed the download to SQLite, and admitted it to the queue. Rejections and outages leave the browser download intact.

Automatic interception supports HTTP and HTTPS only and respects the enabled toggle plus exact/subdomain exclusions. The context menu provides an explicit **Download with FluxDM** action. Extension options include live connection testing and exclusion management.

`scripts/smoke-test-browser-extension.ps1` performs an isolated end-to-end browser check without changing the user's normal browser profile or FluxDM data. It installs a temporary current-user native-host registration, launches the desktop and browser with temporary application/profile/download directories, drives the extension options page through the Chromium DevTools protocol, submits a deterministic localhost transfer, and triggers a genuine browser download. Success requires both FluxDM files to match their expected SHA-256 hashes and the intercepted browser item to finish as `interrupted` / `USER_CANCELED`. With `-ExpectDesktopUnavailable`, the harness places the native host alone in a temporary directory, requires the options page to report the outage, and proves the browser retains and completes its own hash-verified download. The script restores any previous registration and removes all temporary state in `finally` cleanup.
