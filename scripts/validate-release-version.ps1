[CmdletBinding()]
param(
  [Parameter(Mandatory)]
  [string]$Tag,

  [string]$WailsConfig = (Join-Path (Split-Path -Parent $PSScriptRoot) 'wails.json')
)

$ErrorActionPreference = 'Stop'
if ($Tag -notmatch '^v((0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*))$') {
  throw "Release tag must be strict vX.Y.Z semver: '$Tag'"
}

$tagVersion = $Matches[1]
& "$PSScriptRoot\get-product-version.ps1" -WailsConfig $WailsConfig -ExpectedVersion $tagVersion
