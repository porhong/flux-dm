[CmdletBinding()]
param(
  [string]$InstallDir = (Join-Path $env:ProgramFiles 'FluxDM\FluxDM'),
  [string]$ExpectedAppSHA256,
  [string]$ExpectedNativeHostSHA256,
  [switch]$RequireSignatures,
  [switch]$RequireRunning
)
$ErrorActionPreference = 'Stop'
$installed = (Resolve-Path -LiteralPath $InstallDir).Path
$appPath = Join-Path $installed 'FluxDM.exe'
$nativeHostPath = Join-Path $installed 'FluxDM.NativeHost.exe'
$uninstallerPath = Join-Path $installed 'uninstall.exe'
$hostManifestPath = Join-Path $installed 'com.fluxdm.browser.json'
$extensionManifestPath = Join-Path $installed 'browser-extension\manifest.json'
$uninstallKey = 'HKLM:\Software\Microsoft\Windows\CurrentVersion\Uninstall\FluxDMFluxDM'
$nativeKeys = @(
  'HKLM:\Software\Google\Chrome\NativeMessagingHosts\com.fluxdm.browser',
  'HKLM:\Software\Microsoft\Edge\NativeMessagingHosts\com.fluxdm.browser'
)
$shortcuts = @(
  (Join-Path $env:ProgramData 'Microsoft\Windows\Start Menu\Programs\FluxDM.lnk'),
  (Join-Path $env:ProgramData 'Microsoft\Windows\Start Menu\Programs\FluxDM Browser Extension Setup.lnk'),
  (Join-Path ([Environment]::GetFolderPath('CommonDesktopDirectory')) 'FluxDM.lnk')
)

$required = @($appPath,$nativeHostPath,$uninstallerPath,$hostManifestPath,$extensionManifestPath)
foreach ($path in $required) { if (-not (Test-Path -LiteralPath $path)) { throw "Installed layout is missing $path" } }
foreach ($shortcut in $shortcuts) { if (-not (Test-Path -LiteralPath $shortcut)) { throw "Installed shortcut is missing: $shortcut" } }
if (-not (Test-Path -LiteralPath $uninstallKey)) { throw "Installed uninstall registration is missing: $uninstallKey" }

$hostManifest = Get-Content -Raw -Encoding utf8 -LiteralPath $hostManifestPath | ConvertFrom-Json
if ($hostManifest.name -ne 'com.fluxdm.browser') { throw 'Installed native-host manifest name is invalid.' }
if ([IO.Path]::GetFullPath($hostManifest.path) -ne [IO.Path]::GetFullPath($nativeHostPath)) { throw 'Installed native-host manifest path is invalid.' }
if (@($hostManifest.allowed_origins).Count -ne 1 -or $hostManifest.allowed_origins[0] -ne 'chrome-extension://hnemapnmnkccfommbacamppclohhcbfn/') { throw 'Installed native-host allowed origin is invalid.' }
foreach ($key in $nativeKeys) {
  if (-not (Test-Path -LiteralPath $key)) { throw "Installed native-host registration is missing: $key" }
  $value = (Get-Item -LiteralPath $key).GetValue('', $null, [Microsoft.Win32.RegistryValueOptions]::DoNotExpandEnvironmentNames)
  if ([IO.Path]::GetFullPath($value) -ne [IO.Path]::GetFullPath($hostManifestPath)) { throw "Installed native-host registration has the wrong value: $key" }
}

$extensionManifest = Get-Content -Raw -Encoding utf8 -LiteralPath $extensionManifestPath | ConvertFrom-Json
if ($extensionManifest.manifest_version -ne 3 -or $extensionManifest.version -ne '1.0.0' -or -not $extensionManifest.key) { throw 'Installed extension identity/version contract is invalid.' }
$appHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $appPath).Hash
$nativeHostHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $nativeHostPath).Hash
if ($ExpectedAppSHA256 -and $appHash -ne $ExpectedAppSHA256.Trim().ToUpperInvariant()) { throw 'Installed FluxDM.exe hash does not match the expected release input.' }
if ($ExpectedNativeHostSHA256 -and $nativeHostHash -ne $ExpectedNativeHostSHA256.Trim().ToUpperInvariant()) { throw 'Installed native-host hash does not match the expected release input.' }

$signatureEvidence = [ordered]@{}
$artifactPaths = [ordered]@{ app=$appPath; nativeHost=$nativeHostPath; uninstaller=$uninstallerPath }
foreach ($artifact in $artifactPaths.GetEnumerator()) {
  $signature = Get-AuthenticodeSignature -LiteralPath $artifact.Value
  $signatureEvidence[$artifact.Key] = [ordered]@{ status=$signature.Status.ToString(); signer=if($signature.SignerCertificate){$signature.SignerCertificate.Subject}else{$null} }
  if ($RequireSignatures -and $signature.Status -ne 'Valid') { throw "Installed $($artifact.Key) signature is not valid: $($signature.Status)" }
}

$running = @(Get-Process FluxDM -ErrorAction SilentlyContinue | Where-Object { $_.Path -eq $appPath } | ForEach-Object {
  $_.Refresh()
  [ordered]@{ pid=$_.Id; responding=$_.Responding; title=$_.MainWindowTitle; windowHandle=$_.MainWindowHandle }
})
if ($RequireRunning -and ($running.Count -eq 0 -or -not ($running | Where-Object responding))) { throw 'Installed FluxDM process is not running and responding.' }

$uninstall = Get-ItemProperty -LiteralPath $uninstallKey
[ordered]@{
  installDir = $installed
  appSHA256 = $appHash
  nativeHostSHA256 = $nativeHostHash
  extension = [ordered]@{ manifestVersion=$extensionManifest.manifest_version; version=$extensionManifest.version }
  nativeHost = [ordered]@{ manifest=$hostManifestPath; executable=$hostManifest.path; allowedOrigins=$hostManifest.allowed_origins; registrations=$nativeKeys }
  uninstall = [ordered]@{ key=$uninstallKey; displayName=$uninstall.DisplayName; displayVersion=$uninstall.DisplayVersion; uninstallString=$uninstall.UninstallString; quietUninstallString=$uninstall.QuietUninstallString }
  shortcuts = $shortcuts
  signatures = $signatureEvidence
  running = $running
} | ConvertTo-Json -Depth 7
