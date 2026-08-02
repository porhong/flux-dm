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
  Assert-Equal (& "$PSScriptRoot\validate-release-version.ps1" -Tag 'v1.2.3' -WailsConfig $wailsConfig) '1.2.3' 'Valid release tag must resolve to the product version.'
  Assert-Throws { & "$PSScriptRoot\validate-release-version.ps1" -Tag 'v1.2' -WailsConfig $wailsConfig } 'Non-semver release tags must be rejected.'
  Assert-Equal (& "$PSScriptRoot\validate-rc-release-version.ps1" -Tag 'v1.2.3-rc.4' -WailsConfig $wailsConfig) '1.2.3-rc.4' 'Valid RC tag must resolve to its release version.'

  $repositoryVersion = & "$PSScriptRoot\get-product-version.ps1"
  $portableApp = Join-Path $temporaryRoot 'FluxDM.exe'
  $portableNativeHost = Join-Path $temporaryRoot 'FluxDM.NativeHost.exe'
  $extensionSource = Join-Path $temporaryRoot 'browser-extension'
  New-Item -ItemType Directory -Path (Join-Path $extensionSource 'native-host'), (Join-Path $extensionSource 'icons') -Force | Out-Null
  [ordered]@{ manifest_version = 3; version = $repositoryVersion; key = 'test-public-key'; permissions = @('nativeMessaging') } | ConvertTo-Json | Set-Content -Encoding utf8 -LiteralPath (Join-Path $extensionSource 'manifest.json')
  'console.log("FluxDM test extension")' | Set-Content -Encoding utf8 -LiteralPath (Join-Path $extensionSource 'service-worker.js')
  '{}' | Set-Content -Encoding utf8 -LiteralPath (Join-Path $extensionSource 'native-host\com.fluxdm.browser.template.json')
  [IO.File]::WriteAllText($portableApp, 'portable-app')
  [IO.File]::WriteAllText($portableNativeHost, 'portable-native-host')

  $integrationPath = Join-Path $temporaryRoot 'FluxDM-portable-browser-integration.zip'
  $integration = & "$PSScriptRoot\package-portable-browser-integration.ps1" -Version $repositoryVersion -NativeHostPath $portableNativeHost -ExtensionPath $extensionSource -OutputPath $integrationPath | ConvertFrom-Json
  Add-Type -AssemblyName System.IO.Compression.FileSystem
  $integrationZip = [IO.Compression.ZipFile]::OpenRead($integration.package)
  try {
    $entries = @($integrationZip.Entries.FullName | ForEach-Object { $_.Replace('\', '/') })
    foreach ($required in @('FluxDM.NativeHost.exe', 'scripts/install-browser-integration.ps1', 'browser-extension/manifest.json')) {
      Assert-True ($entries -contains $required) "Portable browser integration package is missing $required."
    }
  } finally { $integrationZip.Dispose() }

  $releaseVersion = "$repositoryVersion-rc.4"
  $output = Join-Path $temporaryRoot 'portable-release'
  $staged = & "$PSScriptRoot\stage-portable-release-assets.ps1" -Version $releaseVersion -ProductVersion $repositoryVersion -PortablePath $portableApp -PortableBrowserIntegrationPackagePath $integration.package -OutputDirectory $output | ConvertFrom-Json
  $portableName = "FluxDM-$releaseVersion-windows-amd64-portable.exe"
  $integrationName = "FluxDM-$releaseVersion-windows-amd64-portable-browser-integration.zip"
  Assert-Equal (Split-Path -Leaf $staged.portable) $portableName 'Portable asset name is incorrect.'
  Assert-Equal (Split-Path -Leaf $staged.portableBrowserIntegration) $integrationName 'Portable browser integration asset name is incorrect.'
  Assert-True (Test-Path -LiteralPath $staged.checksums) 'SHA256SUMS.txt was not created.'
  $manifest = Get-Content -Raw -Encoding utf8 -LiteralPath $staged.manifest | ConvertFrom-Json
  Assert-True $manifest.portable 'Portable release manifest must report portable=true.'
  Assert-True (-not $manifest.signed) 'Portable release manifest must report signed=false.'
  Assert-Equal @($manifest.artifacts).Count 2 'Portable release must contain only the executable and portable browser integration assets.'
  Assert-True (-not ($manifest.artifacts | Where-Object kind -eq 'browser-extension')) 'Portable release must not publish a standalone browser extension.'
  Assert-Throws { & "$PSScriptRoot\stage-portable-release-assets.ps1" -Version $releaseVersion -ProductVersion $repositoryVersion -PortablePath $integration.package -PortableBrowserIntegrationPackagePath $integration.package -OutputDirectory (Join-Path $temporaryRoot 'invalid-portable') } 'Portable release staging must reject a non-executable portable input.'

  foreach ($workflowName in @('release.yml', 'rc-release.yml')) {
    $workflow = Get-Content -Raw -Encoding utf8 -LiteralPath (Join-Path $root ".github\workflows\$workflowName")
    foreach ($required in @('runs-on: windows-2022', 'build-portable-release.ps1', 'portable.exe', 'portable-browser-integration.zip', 'SHA256SUMS.txt', 'release-manifest.json')) {
      Assert-True ($workflow.Contains($required)) "$workflowName is missing portable release contract: $required"
    }
    foreach ($forbidden in @('installer.exe', 'browser-extension.zip', 'update-manifest.json', 'update-manifest.sig', 'fluxdm-signing', 'environment: release', 'FLUXDM_CERT_THUMBPRINT')) {
      Assert-True (-not $workflow.Contains($forbidden)) "$workflowName must not publish or require installer-only contract: $forbidden"
    }
  }

  $portableBuildScript = Get-Content -Raw -Encoding utf8 -LiteralPath (Join-Path $PSScriptRoot 'build-portable-release.ps1')
  foreach ($required in @('PortableMode=true', 'FluxDM.NativeHost.exe', 'package-portable-browser-integration.ps1', 'stage-portable-release-assets.ps1', 'verify-version-metadata.ps1')) {
    Assert-True ($portableBuildScript.Contains($required)) "Portable release build is missing required contract: $required"
  }
  Assert-True (-not $portableBuildScript.Contains('package-browser-extension.ps1')) 'Portable release build must not create a standalone browser-extension release asset.'

  Write-Host 'Release automation checks passed.'
} finally {
  if (Test-Path -LiteralPath $temporaryRoot) { Remove-Item -LiteralPath $temporaryRoot -Recurse -Force }
}
