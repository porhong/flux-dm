[CmdletBinding()]
param(
  [Parameter(Mandatory)]
  [string]$Version,

  [string]$ProductVersion,

  [Parameter(Mandatory)]
  [string]$InstallerPath,

  [Parameter(Mandatory)]
  [string]$PortablePath,

  [Parameter(Mandatory)]
  [string]$ExtensionPackagePath,

  [string]$OutputDirectory = (Join-Path (Split-Path -Parent $PSScriptRoot) 'build\release'),

  [switch]$Signed,

  [string]$UpdateManifestPrivateKey
)

$ErrorActionPreference = 'Stop'
if ($Version -notmatch '^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-rc\.[1-9][0-9]*)?$') {
  throw "Version must be X.Y.Z or X.Y.Z-rc.N: '$Version'"
}
if (-not $ProductVersion) { $ProductVersion = $Version -replace '-rc\.[1-9][0-9]*$', '' }
if (($Version -replace '-rc\.[1-9][0-9]*$', '') -ne $ProductVersion) {
  throw "Version '$Version' does not match product version '$ProductVersion'."
}
& "$PSScriptRoot\get-product-version.ps1" -ExpectedVersion $ProductVersion | Out-Null
$installer = (Resolve-Path -LiteralPath $InstallerPath).Path
$portable = (Resolve-Path -LiteralPath $PortablePath).Path
$extensionPackage = (Resolve-Path -LiteralPath $ExtensionPackagePath).Path
if ([IO.Path]::GetExtension($portable) -ne '.exe') { throw "PortablePath must be an .exe file: '$PortablePath'" }
$output = [IO.Path]::GetFullPath($OutputDirectory)
New-Item -ItemType Directory -Force -Path $output | Out-Null

$installerName = "FluxDM-$Version-windows-amd64-installer.exe"
$releaseInstaller = Join-Path $output $installerName
$checksumPath = "$releaseInstaller.sha256"
$portableName = "FluxDM-$Version-windows-amd64-portable.exe"
$releasePortable = Join-Path $output $portableName
$portableChecksumPath = "$releasePortable.sha256"
$extensionName = "FluxDM-$Version-browser-extension.zip"
$releaseExtension = Join-Path $output $extensionName
$extensionChecksumPath = "$releaseExtension.sha256"
$sumsPath = Join-Path $output 'SHA256SUMS.txt'
$manifestPath = Join-Path $output 'release-manifest.json'
$updateManifestPath = Join-Path $output 'update-manifest.json'
$updateSignaturePath = Join-Path $output 'update-manifest.sig'

Copy-Item -LiteralPath $installer -Destination $releaseInstaller -Force
Copy-Item -LiteralPath $portable -Destination $releasePortable -Force
Copy-Item -LiteralPath $extensionPackage -Destination $releaseExtension -Force
$sha256 = (Get-FileHash -Algorithm SHA256 -LiteralPath $releaseInstaller).Hash.ToLowerInvariant()
$portableSHA256 = (Get-FileHash -Algorithm SHA256 -LiteralPath $releasePortable).Hash.ToLowerInvariant()
$extensionSHA256 = (Get-FileHash -Algorithm SHA256 -LiteralPath $releaseExtension).Hash.ToLowerInvariant()
"$sha256  $installerName" | Set-Content -Encoding ascii -NoNewline -LiteralPath $checksumPath
"$portableSHA256  $portableName" | Set-Content -Encoding ascii -NoNewline -LiteralPath $portableChecksumPath
"$extensionSHA256  $extensionName" | Set-Content -Encoding ascii -NoNewline -LiteralPath $extensionChecksumPath
[IO.File]::WriteAllText($sumsPath, "$sha256  $installerName`n$portableSHA256  $portableName`n$extensionSHA256  $extensionName`n", [Text.ASCIIEncoding]::new())

[ordered]@{
  version = $Version
  productVersion = $ProductVersion
  signed = [bool]$Signed
  generatedAt = (Get-Date).ToUniversalTime().ToString('o')
  artifacts = @(
    [ordered]@{
      kind = 'installer'
      file = $installerName
      sha256 = $sha256
      bytes = (Get-Item -LiteralPath $releaseInstaller).Length
    },
    [ordered]@{
      kind = 'portable'
      file = $portableName
      sha256 = $portableSHA256
      bytes = (Get-Item -LiteralPath $releasePortable).Length
    },
    [ordered]@{
      kind = 'browser-extension'
      file = $extensionName
      sha256 = $extensionSHA256
      bytes = (Get-Item -LiteralPath $releaseExtension).Length
    }
  )
} | ConvertTo-Json -Depth 4 | Set-Content -Encoding utf8 -LiteralPath $manifestPath

$signingKey = if ($UpdateManifestPrivateKey) { $UpdateManifestPrivateKey } else { $env:FLUXDM_UPDATE_MANIFEST_PRIVATE_KEY }
if ($signingKey) {
  $channel = if ($Version -match '-rc\.') { 'preview' } else { 'stable' }
  [ordered]@{
    version = $Version
    productVersion = $ProductVersion
    channel = $channel
    signed = [bool]$Signed
    minimumVersion = ''
    releaseNotesUrl = "https://github.com/porhong/flux-dm/releases/tag/v$Version"
    installer = [ordered]@{ file = $installerName; sha256 = $sha256; bytes = (Get-Item -LiteralPath $releaseInstaller).Length }
  } | ConvertTo-Json -Depth 4 -Compress | Set-Content -Encoding utf8 -NoNewline -LiteralPath $updateManifestPath
  $previousKey = $env:FLUXDM_UPDATE_MANIFEST_PRIVATE_KEY
  try {
    $env:FLUXDM_UPDATE_MANIFEST_PRIVATE_KEY = $signingKey
    go run .\cmd\fluxdm-update-manifest-signer -input $updateManifestPath -output $updateSignaturePath
    if ($LASTEXITCODE) { throw 'Update manifest signing failed.' }
  } finally { $env:FLUXDM_UPDATE_MANIFEST_PRIVATE_KEY = $previousKey }
}

[ordered]@{
  installer = $releaseInstaller
  checksum = $checksumPath
  portable = $releasePortable
  portableChecksum = $portableChecksumPath
  extension = $releaseExtension
  extensionChecksum = $extensionChecksumPath
  checksums = $sumsPath
  manifest = $manifestPath
  updateManifest = if (Test-Path -LiteralPath $updateManifestPath) { $updateManifestPath } else { '' }
  updateSignature = if (Test-Path -LiteralPath $updateSignaturePath) { $updateSignaturePath } else { '' }
} | ConvertTo-Json -Depth 2
