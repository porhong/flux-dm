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
  $validatedRCVersion = & "$PSScriptRoot\validate-rc-release-version.ps1" -Tag 'v1.2.3-rc.4' -WailsConfig $wailsConfig
  Assert-Equal $validatedRCVersion '1.2.3-rc.4' 'Valid release-candidate tag must resolve to its public release version.'
  Assert-Throws { & "$PSScriptRoot\validate-rc-release-version.ps1" -Tag 'v1.2.3-rc.0' -WailsConfig $wailsConfig } 'Release-candidate sequence zero must be rejected.'
  Assert-Throws { & "$PSScriptRoot\validate-rc-release-version.ps1" -Tag 'v1.2.4-rc.1' -WailsConfig $wailsConfig } 'Release-candidate product-version mismatches must be rejected.'

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

  $rcOutput = Join-Path $temporaryRoot 'release-candidate'
  $rcReleaseVersion = "$repositoryVersion-rc.4"
  $rcStaged = & "$PSScriptRoot\stage-release-assets.ps1" -Version $rcReleaseVersion -ProductVersion $repositoryVersion -InstallerPath $installer -OutputDirectory $rcOutput | ConvertFrom-Json
  Assert-Equal (Split-Path -Leaf $rcStaged.installer) "FluxDM-$rcReleaseVersion-windows-amd64-installer.exe" 'Release-candidate installer asset name is incorrect.'
  $rcManifest = Get-Content -Raw -Encoding utf8 -LiteralPath $rcStaged.manifest | ConvertFrom-Json
  Assert-Equal $rcManifest.version $rcReleaseVersion 'Release-candidate manifest must identify the public release version.'
  Assert-Equal $rcManifest.productVersion $repositoryVersion 'Release-candidate manifest must identify the packaged product version.'
  Assert-True (-not $rcManifest.signed) 'Unsigned release-candidate manifest must report signed=false.'
  Assert-Throws { & "$PSScriptRoot\stage-release-assets.ps1" -Version '1.2.4-rc.1' -ProductVersion '1.2.3' -InstallerPath $installer -OutputDirectory (Join-Path $temporaryRoot 'invalid-release-candidate') } 'Release-candidate asset version/product mismatches must be rejected.'

  $workflow = Get-Content -Raw -Encoding utf8 -LiteralPath (Join-Path $root '.github\workflows\release.yml')
  foreach ($required in @('self-hosted', 'windows', 'fluxdm-signing', 'environment: release', 'fail_on_unmatched_files: true', 'generate_release_notes: true', 'FluxDM-${{ steps.version.outputs.version }}-windows-amd64-installer.exe', 'SHA256SUMS.txt', 'release-manifest.json')) {
    Assert-True ($workflow.Contains($required)) "Release workflow is missing required contract: $required"
  }

  $rcWorkflow = Get-Content -Raw -Encoding utf8 -LiteralPath (Join-Path $root '.github\workflows\rc-release.yml')
  foreach ($required in @("'v*-rc.*'", 'runs-on: windows-2022', 'prerelease: true', 'not Authenticode-signed', 'fail_on_unmatched_files: true', 'FluxDM-${{ steps.version.outputs.release_version }}-windows-amd64-installer.exe', 'SHA256SUMS.txt', 'release-manifest.json')) {
    Assert-True ($rcWorkflow.Contains($required)) "Release-candidate workflow is missing required contract: $required"
  }
  foreach ($forbidden in @('FLUXDM_CERT_THUMBPRINT', 'FLUXDM_TIMESTAMP_URL', 'environment: release', 'fluxdm-signing')) {
    Assert-True (-not $rcWorkflow.Contains($forbidden)) "Release-candidate workflow must not contain production signing configuration: $forbidden"
  }

  $ciWorkflow = Get-Content -Raw -Encoding utf8 -LiteralPath (Join-Path $root '.github\workflows\ci.yml')
  foreach ($workflowDefinition in @($ciWorkflow, $rcWorkflow)) {
    Assert-True ($workflowDefinition.Contains('choco install nsis.install --yes')) 'Windows packaging workflow must install NSIS when the compiler is unavailable.'
    Assert-True (-not $workflowDefinition.Contains('nsis.install --version=')) 'Windows packaging workflow must not pin an NSIS version that can conflict with the hosted runner image.'
  }

  $buildScript = Get-Content -Raw -Encoding utf8 -LiteralPath (Join-Path $PSScriptRoot 'build-release.ps1')
  Assert-True ($buildScript.Contains('$productVersion=&')) 'Release build must keep the product version separate from the release-version parameter.'
  Assert-True (-not $buildScript.Contains('$releaseVersion=&')) 'Release build must not overwrite the release-version parameter with the product version.'

  Write-Host 'Release automation checks passed.'
} finally {
  if (Test-Path -LiteralPath $temporaryRoot) { Remove-Item -LiteralPath $temporaryRoot -Recurse -Force }
}
