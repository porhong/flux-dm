[CmdletBinding()]
param(
  [Parameter(Mandatory)]
  [string]$Version,

  [string]$ProductVersion,

  [Parameter(Mandatory)]
  [string]$InstallerPath,

  [Parameter(Mandatory)]
  [string]$ExtensionPackagePath,

  [string]$OutputDirectory = (Join-Path (Split-Path -Parent $PSScriptRoot) 'build\release'),

  [switch]$Signed
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
$extensionPackage = (Resolve-Path -LiteralPath $ExtensionPackagePath).Path
$output = [IO.Path]::GetFullPath($OutputDirectory)
New-Item -ItemType Directory -Force -Path $output | Out-Null

$installerName = "FluxDM-$Version-windows-amd64-installer.exe"
$releaseInstaller = Join-Path $output $installerName
$checksumPath = "$releaseInstaller.sha256"
$extensionName = "FluxDM-$Version-browser-extension.zip"
$releaseExtension = Join-Path $output $extensionName
$extensionChecksumPath = "$releaseExtension.sha256"
$sumsPath = Join-Path $output 'SHA256SUMS.txt'
$manifestPath = Join-Path $output 'release-manifest.json'

Copy-Item -LiteralPath $installer -Destination $releaseInstaller -Force
Copy-Item -LiteralPath $extensionPackage -Destination $releaseExtension -Force
$sha256 = (Get-FileHash -Algorithm SHA256 -LiteralPath $releaseInstaller).Hash.ToLowerInvariant()
$extensionSHA256 = (Get-FileHash -Algorithm SHA256 -LiteralPath $releaseExtension).Hash.ToLowerInvariant()
"$sha256  $installerName" | Set-Content -Encoding ascii -NoNewline -LiteralPath $checksumPath
"$extensionSHA256  $extensionName" | Set-Content -Encoding ascii -NoNewline -LiteralPath $extensionChecksumPath
[IO.File]::WriteAllText($sumsPath, "$sha256  $installerName`n$extensionSHA256  $extensionName`n", [Text.ASCIIEncoding]::new())

[ordered]@{
  version = $Version
  productVersion = $ProductVersion
  signed = [bool]$Signed
  generatedAt = (Get-Date).ToUniversalTime().ToString('o')
  artifacts = @(
    [ordered]@{
      file = $installerName
      sha256 = $sha256
      bytes = (Get-Item -LiteralPath $releaseInstaller).Length
    },
    [ordered]@{
      file = $extensionName
      sha256 = $extensionSHA256
      bytes = (Get-Item -LiteralPath $releaseExtension).Length
    }
  )
} | ConvertTo-Json -Depth 4 | Set-Content -Encoding utf8 -LiteralPath $manifestPath

[ordered]@{
  installer = $releaseInstaller
  checksum = $checksumPath
  extension = $releaseExtension
  extensionChecksum = $extensionChecksumPath
  checksums = $sumsPath
  manifest = $manifestPath
} | ConvertTo-Json -Depth 2
