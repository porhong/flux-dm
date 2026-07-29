[CmdletBinding()]
param(
  [Parameter(Mandatory)]
  [string]$Tag,

  [string]$WailsConfig
)

$ErrorActionPreference = 'Stop'
if (-not $WailsConfig) { $WailsConfig = Join-Path (Split-Path -Parent $PSScriptRoot) 'wails.json' }
if ($Tag -notmatch '^v((0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*))$') {
  throw "Release tag must be strict vX.Y.Z semver: '$Tag'"
}

$tagVersion = $Matches[1]
& "$PSScriptRoot\get-product-version.ps1" -WailsConfig $WailsConfig -ExpectedVersion $tagVersion
