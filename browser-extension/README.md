# Install the FluxDM browser extension (development build)

1. Run `scripts\install-browser-integration.ps1` from the repository root. This builds and registers the per-user native host for Chrome and Edge.
2. Open `chrome://extensions` or `edge://extensions`.
3. Enable **Developer mode**, choose **Load unpacked**, and select this `browser-extension` directory.
4. Open the extension's **Details → Extension options**, then choose **Test connection**.

The manifest contains a fixed public key, so unpacked Chrome and Edge installations use the stable ID `hnemapnmnkccfommbacamppclohhcbfn`. The native host accepts only that origin.

Automatic interception can be disabled globally or excluded by hostname and file extension. Explicit **Download with FluxDM** context-menu actions remain available for HTTP and HTTPS links. Cookie transfer is off by default and is used only when the user enables it.
