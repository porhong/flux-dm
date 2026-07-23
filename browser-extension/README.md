# Install the FluxDM browser extension (development build)

1. Run `scripts\install-browser-integration.ps1` from the repository root. This builds and registers the per-user native host for Chrome and Edge.
2. Open `chrome://extensions` or `edge://extensions`.
3. Enable **Developer mode**, choose **Load unpacked**, and select this `browser-extension` directory.
4. Open the extension's **Details → Extension options**, then choose **Test connection**.

The manifest contains a fixed public key, so unpacked Chrome and Edge installations use the stable ID `hnemapnmnkccfommbacamppclohhcbfn`. The native host accepts only that origin.

Pre-click handoff sends explicitly downloadable links and configured file types to FluxDM before Chrome or Edge creates a browser download. This avoids the browser's Save As/File Explorer UI for successful handoffs. Links whose downloadable nature is only revealed after site scripts, a form submission, or a redirect remain in the browser; use the explicit **Download with FluxDM** context-menu action for those links. Cookie transfer is off by default and is used only when the user enables it.
