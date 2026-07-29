[CmdletBinding()]
param(
  [Parameter(Mandatory)]
  [string]$Tag,

  [string]$WailsConfig
)

$ErrorActionPreference = 'Stop'
if (-not $WailsConfig) { $WailsConfig = Join-Path (Split-Path -Parent $PSScriptRoot) 'wails.json' }

if ($Tag -notmatch '^v((0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)-rc\.([1-9][0-9]*))$') {
  throw "Release-candidate tag must be vX.Y.Z-rc.N, with strict numeric X.Y.Z and positive N: '$Tag'"
}

$releaseVersion = $Matches[1]
$productVersion = $releaseVersion -replace '-rc\.[1-9][0-9]*$', ''
& "$PSScriptRoot\get-product-version.ps1" -WailsConfig $WailsConfig -ExpectedVersion $productVersion | Out-Null
$releaseVersion
