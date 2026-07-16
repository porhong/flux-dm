# Code-signing strategy

Production releases require an organization-validated Authenticode code-signing certificate available in the Windows certificate store. The private key must remain in a hardware-backed provider or CI secret-backed signing service; it is never stored in this repository.

`scripts/build-release.ps1 -Sign` signs `FluxDM.exe` and `FluxDM.NativeHost.exe` before rebuilding the installer. NSIS then signs the embedded uninstaller and final installer through its finalize hooks. Every signature uses SHA-256 and an RFC 3161 timestamp. `scripts/verify-release.ps1` checks both PowerShell Authenticode status and WinVerifyTrust through `signtool verify /pa /all`.

Unsigned development installers may be built for local testing, but must not be published as releases. Release publication is blocked unless all three top-level artifacts and the embedded uninstaller validate on clean Windows 10 and Windows 11 machines.

`-AllowUntimestampedTestSignature` exists only to exercise the complete signing/NSIS verification path with an ephemeral local test certificate when a timestamp authority is unavailable. It requires an empty `-TimestampUrl`, is never acceptable for publication, and does not relax the default production requirement for an RFC 3161 timestamp.

Pass a full 7-Zip executable through `-SevenZipPath` to make `scripts/build-release.ps1` independently extract the final NSIS artifact and compare every packaged application/extension file with its release input. For signed builds, this also requires valid Authenticode status on the installer, packaged desktop executable, packaged native host, and embedded uninstaller. The packaged WebView2 bootstrapper is always required to have a valid Microsoft Corporation signature.
