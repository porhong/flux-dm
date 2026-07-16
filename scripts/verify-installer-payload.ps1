[CmdletBinding()]
param(
  [string]$InstallerPath,
  [Parameter(Mandatory)][string]$SevenZipPath,
  [string]$AppPath,
  [string]$NativeHostPath,
  [string]$ExtensionPath,
  [switch]$RequireSignatures
)
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
if (-not $InstallerPath) { $InstallerPath = Join-Path $root 'build\bin\FluxDM-amd64-installer.exe' }
if (-not $AppPath) { $AppPath = Join-Path $root 'build\bin\FluxDM.exe' }
if (-not $NativeHostPath) { $NativeHostPath = Join-Path $root 'build\bin\FluxDM.NativeHost.exe' }
if (-not $ExtensionPath) { $ExtensionPath = Join-Path $root 'browser-extension' }

$installer = (Resolve-Path -LiteralPath $InstallerPath).Path
$sevenZip = (Resolve-Path -LiteralPath $SevenZipPath).Path
$app = (Resolve-Path -LiteralPath $AppPath).Path
$nativeHost = (Resolve-Path -LiteralPath $NativeHostPath).Path
$extension = (Resolve-Path -LiteralPath $ExtensionPath).Path
$temporaryRoot = Join-Path $env:TEMP ('fluxdm-installer-payload-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $temporaryRoot | Out-Null

function Assert-ValidSignature([string]$Path, [string]$Label) {
  $signature = Get-AuthenticodeSignature -LiteralPath $Path
  if ($signature.Status -ne 'Valid') { throw "$Label signature is not valid: $($signature.Status)" }
  return $signature
}

try {
  $null = & $sevenZip x $installer "-o$temporaryRoot" -y
  if ($LASTEXITCODE -ne 0) { throw "7-Zip could not extract the NSIS installer (exit code $LASTEXITCODE)." }

  $required = @(
    'FluxDM.exe',
    'FluxDM.NativeHost.exe',
    'uninstall.exe',
    'browser-extension\manifest.json',
    'browser-extension\service-worker.js',
    'browser-extension\options.html',
    '$PLUGINSDIR\webview2bootstrapper\MicrosoftEdgeWebview2Setup.exe'
  )
  foreach ($relative in $required) {
    if (-not (Test-Path -LiteralPath (Join-Path $temporaryRoot $relative))) { throw "Installer payload is missing $relative" }
  }

  $comparisons = [Collections.Generic.List[object]]::new()
  foreach ($pair in @(@('FluxDM.exe',$app),@('FluxDM.NativeHost.exe',$nativeHost))) {
    $packagedPath = Join-Path $temporaryRoot $pair[0]
    $packagedHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $packagedPath).Hash
    $sourceHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $pair[1]).Hash
    if ($packagedHash -ne $sourceHash) { throw "Installer payload hash mismatch: $($pair[0])" }
    $comparisons.Add([ordered]@{ file = $pair[0]; sha256 = $packagedHash; match = $true })
  }

  $packagedExtension = Join-Path $temporaryRoot 'browser-extension'
  foreach ($sourceFile in Get-ChildItem -LiteralPath $extension -Recurse -File) {
    $relative = $sourceFile.FullName.Substring($extension.Length + 1)
    $packagedPath = Join-Path $packagedExtension $relative
    if (-not (Test-Path -LiteralPath $packagedPath)) { throw "Installer extension payload is missing $relative" }
    $packagedHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $packagedPath).Hash
    $sourceHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $sourceFile.FullName).Hash
    if ($packagedHash -ne $sourceHash) { throw "Installer extension payload hash mismatch: $relative" }
    $comparisons.Add([ordered]@{ file = "browser-extension\$relative"; sha256 = $packagedHash; match = $true })
  }

  $manifest = Get-Content -Raw -Encoding utf8 -LiteralPath (Join-Path $packagedExtension 'manifest.json') | ConvertFrom-Json
  if ($manifest.manifest_version -ne 3 -or $manifest.version -ne '1.0.0' -or -not $manifest.key) { throw 'Packaged extension identity/version contract is invalid.' }

  $bootstrapPath = Join-Path $temporaryRoot '$PLUGINSDIR\webview2bootstrapper\MicrosoftEdgeWebview2Setup.exe'
  $bootstrapSignature = Assert-ValidSignature $bootstrapPath 'WebView2 bootstrapper'
  if ($bootstrapSignature.SignerCertificate.Subject -notmatch 'O=Microsoft Corporation') { throw "Unexpected WebView2 bootstrapper signer: $($bootstrapSignature.SignerCertificate.Subject)" }

  $signatureReport = [ordered]@{
    installer = (Get-AuthenticodeSignature -LiteralPath $installer).Status.ToString()
    app = (Get-AuthenticodeSignature -LiteralPath (Join-Path $temporaryRoot 'FluxDM.exe')).Status.ToString()
    nativeHost = (Get-AuthenticodeSignature -LiteralPath (Join-Path $temporaryRoot 'FluxDM.NativeHost.exe')).Status.ToString()
    uninstaller = (Get-AuthenticodeSignature -LiteralPath (Join-Path $temporaryRoot 'uninstall.exe')).Status.ToString()
  }
  if ($RequireSignatures) {
    $null = Assert-ValidSignature $installer 'Installer'
    $null = Assert-ValidSignature (Join-Path $temporaryRoot 'FluxDM.exe') 'Packaged desktop executable'
    $null = Assert-ValidSignature (Join-Path $temporaryRoot 'FluxDM.NativeHost.exe') 'Packaged native host'
    $null = Assert-ValidSignature (Join-Path $temporaryRoot 'uninstall.exe') 'Embedded uninstaller'
  }

  [ordered]@{
    installer = $installer
    installerSHA256 = (Get-FileHash -Algorithm SHA256 -LiteralPath $installer).Hash
    nsisPayloadFiles = @(Get-ChildItem -LiteralPath $temporaryRoot -Recurse -File).Count
    requiredFiles = $required
    matchingInputs = $comparisons
    extension = [ordered]@{ manifestVersion = $manifest.manifest_version; version = $manifest.version }
    webViewBootstrapper = [ordered]@{
      sha256 = (Get-FileHash -Algorithm SHA256 -LiteralPath $bootstrapPath).Hash
      signature = $bootstrapSignature.Status.ToString()
      signer = $bootstrapSignature.SignerCertificate.Subject
    }
    signatures = $signatureReport
    productionSignaturesRequired = [bool]$RequireSignatures
  } | ConvertTo-Json -Depth 6
} finally {
  $resolvedTemporaryRoot = [IO.Path]::GetFullPath($temporaryRoot)
  $temporaryPrefix = [IO.Path]::GetFullPath($env:TEMP).TrimEnd('\') + '\'
  if (-not $resolvedTemporaryRoot.StartsWith($temporaryPrefix, [StringComparison]::OrdinalIgnoreCase)) { throw "Refusing cleanup outside TEMP: $resolvedTemporaryRoot" }
  if (Test-Path -LiteralPath $resolvedTemporaryRoot) { Remove-Item -LiteralPath $resolvedTemporaryRoot -Recurse -Force }
}
