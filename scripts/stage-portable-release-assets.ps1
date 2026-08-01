[CmdletBinding()]
param(
  [Parameter(Mandatory)][string]$Version,
  [Parameter(Mandatory)][string]$ProductVersion,
  [Parameter(Mandatory)][string]$PortablePath,
  [Parameter(Mandatory)][string]$ExtensionPackagePath,
  [string]$OutputDirectory = (Join-Path (Split-Path -Parent $PSScriptRoot) 'build\release')
)

$ErrorActionPreference = 'Stop'
if ($Version -notmatch '^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-rc\.[1-9][0-9]*)?$') { throw "Version must be X.Y.Z or X.Y.Z-rc.N: '$Version'" }
if (($Version -replace '-rc\.[1-9][0-9]*$', '') -ne $ProductVersion) { throw "Version '$Version' does not match product version '$ProductVersion'." }
$portable = (Resolve-Path -LiteralPath $PortablePath -ErrorAction Stop).Path
$extensionPackage = (Resolve-Path -LiteralPath $ExtensionPackagePath -ErrorAction Stop).Path
if ([IO.Path]::GetExtension($portable) -ne '.exe') { throw "PortablePath must be an .exe file: '$PortablePath'" }
$output = [IO.Path]::GetFullPath($OutputDirectory)
New-Item -ItemType Directory -Force -Path $output | Out-Null

$portableName = "FluxDM-$Version-windows-amd64-portable.exe"
$extensionName = "FluxDM-$Version-browser-extension.zip"
$portableRelease = Join-Path $output $portableName
$extensionRelease = Join-Path $output $extensionName
Copy-Item -LiteralPath $portable -Destination $portableRelease -Force
Copy-Item -LiteralPath $extensionPackage -Destination $extensionRelease -Force
$portableHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $portableRelease).Hash.ToLowerInvariant()
$extensionHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $extensionRelease).Hash.ToLowerInvariant()
"$portableHash  $portableName" | Set-Content -Encoding ascii -NoNewline -LiteralPath "$portableRelease.sha256"
"$extensionHash  $extensionName" | Set-Content -Encoding ascii -NoNewline -LiteralPath "$extensionRelease.sha256"
[IO.File]::WriteAllText((Join-Path $output 'SHA256SUMS.txt'), "$portableHash  $portableName`n$extensionHash  $extensionName`n", [Text.ASCIIEncoding]::new())

[ordered]@{
  version = $Version
  productVersion = $ProductVersion
  portable = $true
  signed = $false
  generatedAt = (Get-Date).ToUniversalTime().ToString('o')
  artifacts = @(
    [ordered]@{ kind = 'portable'; file = $portableName; sha256 = $portableHash; bytes = (Get-Item -LiteralPath $portableRelease).Length },
    [ordered]@{ kind = 'browser-extension'; file = $extensionName; sha256 = $extensionHash; bytes = (Get-Item -LiteralPath $extensionRelease).Length }
  )
} | ConvertTo-Json -Depth 4 | Set-Content -Encoding utf8 -LiteralPath (Join-Path $output 'release-manifest.json')

[ordered]@{
  portable = $portableRelease
  portableChecksum = "$portableRelease.sha256"
  extension = $extensionRelease
  extensionChecksum = "$extensionRelease.sha256"
  checksums = Join-Path $output 'SHA256SUMS.txt'
  manifest = Join-Path $output 'release-manifest.json'
} | ConvertTo-Json -Depth 2
