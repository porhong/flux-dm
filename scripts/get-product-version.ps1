[CmdletBinding()]
param(
  [string]$WailsConfig = (Join-Path (Split-Path -Parent $PSScriptRoot) 'wails.json'),
  [string]$ExpectedVersion
)

$ErrorActionPreference = 'Stop'
$configPath = (Resolve-Path -LiteralPath $WailsConfig).Path
$config = Get-Content -Raw -Encoding utf8 -LiteralPath $configPath | ConvertFrom-Json
$version = [string]$config.info.productVersion

if ($version -notmatch '^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$') {
  throw "wails.json productVersion must be strict X.Y.Z semver: '$version'"
}

if ($ExpectedVersion) {
  if ($ExpectedVersion -notmatch '^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$') {
    throw "ExpectedVersion must be strict X.Y.Z semver: '$ExpectedVersion'"
  }
  if ($ExpectedVersion -ne $version) {
    throw "ExpectedVersion '$ExpectedVersion' does not match wails.json productVersion '$version'."
  }
}

$version
