[CmdletBinding()]
param(
  [Parameter(Mandatory)][string]$Version,
  [Parameter(Mandatory)][string]$ProductVersion,
  [Parameter(Mandatory)][string]$PortablePath,
  [Parameter(Mandatory)][string]$PortableBrowserIntegrationPackagePath,
  [string]$OutputDirectory = (Join-Path (Split-Path -Parent $PSScriptRoot) 'build\release')
)

$ErrorActionPreference = 'Stop'
if ($Version -notmatch '^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-rc\.[1-9][0-9]*)?$') { throw "Version must be X.Y.Z or X.Y.Z-rc.N: '$Version'" }
if (($Version -replace '-rc\.[1-9][0-9]*$', '') -ne $ProductVersion) { throw "Version '$Version' does not match product version '$ProductVersion'." }
$portable = (Resolve-Path -LiteralPath $PortablePath -ErrorAction Stop).Path
$portableBrowserIntegrationPackage = (Resolve-Path -LiteralPath $PortableBrowserIntegrationPackagePath -ErrorAction Stop).Path
if ([IO.Path]::GetExtension($portable) -ne '.exe') { throw "PortablePath must be an .exe file: '$PortablePath'" }
$output = [IO.Path]::GetFullPath($OutputDirectory)
New-Item -ItemType Directory -Force -Path $output | Out-Null

$portableName = "FluxDM-$Version-windows-amd64-portable.exe"
$portableBrowserIntegrationName = "FluxDM-$Version-windows-amd64-portable-browser-integration.zip"
$portableRelease = Join-Path $output $portableName
$portableBrowserIntegrationRelease = Join-Path $output $portableBrowserIntegrationName
Copy-Item -LiteralPath $portable -Destination $portableRelease -Force
Copy-Item -LiteralPath $portableBrowserIntegrationPackage -Destination $portableBrowserIntegrationRelease -Force
$portableHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $portableRelease).Hash.ToLowerInvariant()
$portableBrowserIntegrationHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $portableBrowserIntegrationRelease).Hash.ToLowerInvariant()
"$portableHash  $portableName" | Set-Content -Encoding ascii -NoNewline -LiteralPath "$portableRelease.sha256"
"$portableBrowserIntegrationHash  $portableBrowserIntegrationName" | Set-Content -Encoding ascii -NoNewline -LiteralPath "$portableBrowserIntegrationRelease.sha256"
[IO.File]::WriteAllText((Join-Path $output 'SHA256SUMS.txt'), "$portableHash  $portableName`n$portableBrowserIntegrationHash  $portableBrowserIntegrationName`n", [Text.ASCIIEncoding]::new())

[ordered]@{
  version = $Version
  productVersion = $ProductVersion
  portable = $true
  signed = $false
  generatedAt = (Get-Date).ToUniversalTime().ToString('o')
  artifacts = @(
    [ordered]@{ kind = 'portable'; file = $portableName; sha256 = $portableHash; bytes = (Get-Item -LiteralPath $portableRelease).Length },
    [ordered]@{ kind = 'portable-browser-integration'; file = $portableBrowserIntegrationName; sha256 = $portableBrowserIntegrationHash; bytes = (Get-Item -LiteralPath $portableBrowserIntegrationRelease).Length }
  )
} | ConvertTo-Json -Depth 4 | Set-Content -Encoding utf8 -LiteralPath (Join-Path $output 'release-manifest.json')

[ordered]@{
  portable = $portableRelease
  portableChecksum = "$portableRelease.sha256"
  portableBrowserIntegration = $portableBrowserIntegrationRelease
  portableBrowserIntegrationChecksum = "$portableBrowserIntegrationRelease.sha256"
  checksums = Join-Path $output 'SHA256SUMS.txt'
  manifest = Join-Path $output 'release-manifest.json'
} | ConvertTo-Json -Depth 2
