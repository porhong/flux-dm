# Install the FluxDM browser extension (development build)

For an installed FluxDM release, use the **FluxDM Browser Extension Setup** shortcut. It opens the packaged setup page and the extension folder is already available at `C:\Program Files\FluxDM\FluxDM\browser-extension`.

Every release also includes `FluxDM-X.Y.Z-browser-extension.zip`. This is the browser-store submission and portable import package; install FluxDM first so its native messaging host is registered. Chrome, Edge, and Brave do not install a local ZIP directly from their extensions pages: extract the ZIP, enable **Developer mode**, then select **Load unpacked** and choose the extracted folder. A store-published extension should be installed from that browser's official store instead.

1. Run `scripts\install-browser-integration.ps1` from the repository root. This builds and registers the per-user native host for Chrome, Edge, and Brave.
2. Open `chrome://extensions`, `edge://extensions`, or `brave://extensions`.
3. Enable **Developer mode**, choose **Load unpacked**, and select this `browser-extension` directory.
4. Open the extension's **Details → Extension options**, then choose **Test connection**.

The manifest contains a fixed public key, so unpacked Chrome, Edge, and Brave installations use the stable ID `hnemapnmnkccfommbacamppclohhcbfn`. The native host accepts only that origin.

Pre-click handoff sends explicitly downloadable links and configured file types to FluxDM before Chrome, Edge, or Brave creates a browser download. This avoids the browser's Save As/File Explorer UI for successful handoffs. Links whose downloadable nature is only revealed after site scripts, a form submission, or a redirect remain in the browser; use the explicit **Download with FluxDM** context-menu action for those links. Cookie transfer is off by default and is used only when the user enables it.
