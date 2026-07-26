[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
$temporaryRoot = Join-Path $env:TEMP ('fluxdm-release-automation-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $temporaryRoot | Out-Null

function Assert-Equal([object]$Actual, [object]$Expected, [string]$Message) {
  if ($Actual -ne $Expected) { throw "$Message Expected '$Expected', got '$Actual'." }
}

function Assert-True([bool]$Value, [string]$Message) {
  if (-not $Value) { throw $Message }
}

function Assert-Throws([scriptblock]$Action, [string]$Message) {
  try { & $Action } catch { return }
  throw $Message
}

try {
  $wailsConfig = Join-Path $temporaryRoot 'wails.json'
  '{"info":{"productVersion":"1.2.3"}}' | Set-Content -Encoding utf8 -LiteralPath $wailsConfig
  $validatedVersion = & "$PSScriptRoot\validate-release-version.ps1" -Tag 'v1.2.3' -WailsConfig $wailsConfig
  Assert-Equal $validatedVersion '1.2.3' 'Valid release tag must resolve to the product version.'
  Assert-Throws { & "$PSScriptRoot\validate-release-version.ps1" -Tag 'v1.2' -WailsConfig $wailsConfig } 'Non-semver release tags must be rejected.'
  Assert-Throws { & "$PSScriptRoot\validate-release-version.ps1" -Tag 'v1.2.4' -WailsConfig $wailsConfig } 'Tag/product version mismatches must be rejected.'

  $installer = Join-Path $temporaryRoot 'installer.exe'
  $privateSigningSentinel = 'do-not-publish-private-signing-data'
  [IO.File]::WriteAllText($installer, $privateSigningSentinel)
  $output = Join-Path $temporaryRoot 'release'
  $repositoryVersion = & "$PSScriptRoot\get-product-version.ps1"
  $staged = & "$PSScriptRoot\stage-release-assets.ps1" -Version $repositoryVersion -InstallerPath $installer -OutputDirectory $output -Signed | ConvertFrom-Json
  $installerName = "FluxDM-$repositoryVersion-windows-amd64-installer.exe"
  Assert-Equal (Split-Path -Leaf $staged.installer) $installerName 'Installer asset name is incorrect.'
  Assert-Equal (Split-Path -Leaf $staged.checksum) "$installerName.sha256" 'Checksum asset name is incorrect.'
  Assert-True (Test-Path -LiteralPath $staged.checksums) 'SHA256SUMS.txt was not created.'
  $sum = Get-Content -Raw -Encoding ascii -LiteralPath $staged.checksum
  Assert-True ($sum -match [regex]::Escape("  $installerName")) 'Checksum file does not identify the versioned installer.'
  $manifest = Get-Content -Raw -Encoding utf8 -LiteralPath $staged.manifest
  Assert-True ($manifest -notmatch [regex]::Escape($privateSigningSentinel)) 'Release manifest must not contain private signing data.'
  Assert-True ($manifest -match [regex]::Escape($installerName)) 'Release manifest does not identify the versioned installer.'

  $workflow = Get-Content -Raw -Encoding utf8 -LiteralPath (Join-Path $root '.github\workflows\release.yml')
  foreach ($required in @('self-hosted', 'windows', 'fluxdm-signing', 'environment: release', 'generate_release_notes: true', 'FluxDM-${{ steps.version.outputs.version }}-windows-amd64-installer.exe', 'SHA256SUMS.txt', 'release-manifest.json')) {
    Assert-True ($workflow.Contains($required)) "Release workflow is missing required contract: $required"
  }

  Write-Host 'Release automation checks passed.'
} finally {
  if (Test-Path -LiteralPath $temporaryRoot) { Remove-Item -LiteralPath $temporaryRoot -Recurse -Force }
}
