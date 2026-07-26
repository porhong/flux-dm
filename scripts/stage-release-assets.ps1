[CmdletBinding()]
param(
  [Parameter(Mandatory)]
  [string]$Version,

  [Parameter(Mandatory)]
  [string]$InstallerPath,

  [string]$OutputDirectory = (Join-Path (Split-Path -Parent $PSScriptRoot) 'build\release'),

  [switch]$Signed
)

$ErrorActionPreference = 'Stop'
& "$PSScriptRoot\get-product-version.ps1" -ExpectedVersion $Version | Out-Null
$installer = (Resolve-Path -LiteralPath $InstallerPath).Path
$output = [IO.Path]::GetFullPath($OutputDirectory)
New-Item -ItemType Directory -Force -Path $output | Out-Null

$installerName = "FluxDM-$Version-windows-amd64-installer.exe"
$releaseInstaller = Join-Path $output $installerName
$checksumPath = "$releaseInstaller.sha256"
$sumsPath = Join-Path $output 'SHA256SUMS.txt'
$manifestPath = Join-Path $output 'release-manifest.json'

Copy-Item -LiteralPath $installer -Destination $releaseInstaller -Force
$sha256 = (Get-FileHash -Algorithm SHA256 -LiteralPath $releaseInstaller).Hash.ToLowerInvariant()
"$sha256  $installerName" | Set-Content -Encoding ascii -NoNewline -LiteralPath $checksumPath
"$sha256  $installerName" | Set-Content -Encoding ascii -NoNewline -LiteralPath $sumsPath

[ordered]@{
  version = $Version
  signed = [bool]$Signed
  generatedAt = (Get-Date).ToUniversalTime().ToString('o')
  artifacts = @(
    [ordered]@{
      file = $installerName
      sha256 = $sha256
      bytes = (Get-Item -LiteralPath $releaseInstaller).Length
    }
  )
} | ConvertTo-Json -Depth 4 | Set-Content -Encoding utf8 -LiteralPath $manifestPath

[ordered]@{
  installer = $releaseInstaller
  checksum = $checksumPath
  checksums = $sumsPath
  manifest = $manifestPath
} | ConvertTo-Json -Depth 2
