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
  $extensionSource = Join-Path $temporaryRoot 'browser-extension'
  New-Item -ItemType Directory -Path (Join-Path $extensionSource 'native-host'),(Join-Path $extensionSource 'icons') -Force | Out-Null
  $repositoryVersion = & "$PSScriptRoot\get-product-version.ps1"
  [ordered]@{ manifest_version=3; version=$repositoryVersion; key='test-public-key'; permissions=@('nativeMessaging') } | ConvertTo-Json | Set-Content -Encoding utf8 -LiteralPath (Join-Path $extensionSource 'manifest.json')
  'console.log("FluxDM test extension")' | Set-Content -Encoding utf8 -LiteralPath (Join-Path $extensionSource 'service-worker.js')
  'development-only documentation' | Set-Content -Encoding utf8 -LiteralPath (Join-Path $extensionSource 'README.md')
  '{}' | Set-Content -Encoding utf8 -LiteralPath (Join-Path $extensionSource 'native-host\com.fluxdm.browser.template.json')
  $extensionPackage = Join-Path $temporaryRoot 'FluxDM-browser-extension.zip'
  $packagedExtension = & "$PSScriptRoot\package-browser-extension.ps1" -ExtensionPath $extensionSource -ExpectedVersion $repositoryVersion -OutputPath $extensionPackage | ConvertFrom-Json
  Assert-True (Test-Path -LiteralPath $packagedExtension.package) 'Browser extension package was not created.'
  Add-Type -AssemblyName System.IO.Compression.FileSystem
  $zip = [IO.Compression.ZipFile]::OpenRead($packagedExtension.package)
  try {
    Assert-True ($zip.Entries.FullName -contains 'manifest.json') 'Browser extension package is missing manifest.json.'
    Assert-True ($zip.Entries.FullName -notcontains 'README.md') 'Browser extension package must exclude development documentation.'
    Assert-True (-not ($zip.Entries.FullName | Where-Object { $_.StartsWith('native-host/', [StringComparison]::OrdinalIgnoreCase) })) 'Browser extension package must exclude the native-host template.'
  } finally { $zip.Dispose() }
  $privateSigningSentinel = 'do-not-publish-private-signing-data'
  [IO.File]::WriteAllText($installer, $privateSigningSentinel)
	$keyBytes = New-Object byte[] 32
	$keyGenerator = [Security.Cryptography.RandomNumberGenerator]::Create()
	try { $keyGenerator.GetBytes($keyBytes) } finally { $keyGenerator.Dispose() }
	$updateSigningKey = [Convert]::ToBase64String($keyBytes)
  $output = Join-Path $temporaryRoot 'release'
  $staged = & "$PSScriptRoot\stage-release-assets.ps1" -Version $repositoryVersion -InstallerPath $installer -ExtensionPackagePath $extensionPackage -OutputDirectory $output -Signed -UpdateManifestPrivateKey $updateSigningKey | ConvertFrom-Json
  $installerName = "FluxDM-$repositoryVersion-windows-amd64-installer.exe"
  $extensionName = "FluxDM-$repositoryVersion-browser-extension.zip"
  Assert-Equal (Split-Path -Leaf $staged.installer) $installerName 'Installer asset name is incorrect.'
  Assert-Equal (Split-Path -Leaf $staged.checksum) "$installerName.sha256" 'Checksum asset name is incorrect.'
  Assert-Equal (Split-Path -Leaf $staged.extension) $extensionName 'Browser extension asset name is incorrect.'
  Assert-Equal (Split-Path -Leaf $staged.extensionChecksum) "$extensionName.sha256" 'Browser extension checksum asset name is incorrect.'
  Assert-True (Test-Path -LiteralPath $staged.checksums) 'SHA256SUMS.txt was not created.'
  $sum = Get-Content -Raw -Encoding ascii -LiteralPath $staged.checksum
  Assert-True ($sum -match [regex]::Escape("  $installerName")) 'Checksum file does not identify the versioned installer.'
  $extensionSum = Get-Content -Raw -Encoding ascii -LiteralPath $staged.extensionChecksum
  Assert-True ($extensionSum -match [regex]::Escape("  $extensionName")) 'Browser extension checksum file does not identify the versioned extension.'
  $manifest = Get-Content -Raw -Encoding utf8 -LiteralPath $staged.manifest
  Assert-True ($manifest -notmatch [regex]::Escape($privateSigningSentinel)) 'Release manifest must not contain private signing data.'
  Assert-True ($manifest -match [regex]::Escape($installerName)) 'Release manifest does not identify the versioned installer.'
  Assert-True ($manifest -match [regex]::Escape($extensionName)) 'Release manifest does not identify the versioned browser extension.'
	Assert-True (Test-Path -LiteralPath $staged.updateManifest) 'Signed update manifest was not created.'
	Assert-True (Test-Path -LiteralPath $staged.updateSignature) 'Signed update manifest signature was not created.'
	$updateManifest = Get-Content -Raw -Encoding utf8 -LiteralPath $staged.updateManifest | ConvertFrom-Json
	Assert-Equal $updateManifest.channel 'stable' 'Production update manifest must identify the stable channel.'
	Assert-Equal $updateManifest.installer.file $installerName 'Update manifest must identify the versioned installer.'

  $rcOutput = Join-Path $temporaryRoot 'release-candidate'
  $rcReleaseVersion = "$repositoryVersion-rc.4"
  $rcStaged = & "$PSScriptRoot\stage-release-assets.ps1" -Version $rcReleaseVersion -ProductVersion $repositoryVersion -InstallerPath $installer -ExtensionPackagePath $extensionPackage -OutputDirectory $rcOutput -UpdateManifestPrivateKey $updateSigningKey | ConvertFrom-Json
  Assert-Equal (Split-Path -Leaf $rcStaged.installer) "FluxDM-$rcReleaseVersion-windows-amd64-installer.exe" 'Release-candidate installer asset name is incorrect.'
  $rcManifest = Get-Content -Raw -Encoding utf8 -LiteralPath $rcStaged.manifest | ConvertFrom-Json
  Assert-Equal $rcManifest.version $rcReleaseVersion 'Release-candidate manifest must identify the public release version.'
  Assert-Equal $rcManifest.productVersion $repositoryVersion 'Release-candidate manifest must identify the packaged product version.'
  Assert-True (-not $rcManifest.signed) 'Unsigned release-candidate manifest must report signed=false.'
	$rcUpdateManifest = Get-Content -Raw -Encoding utf8 -LiteralPath $rcStaged.updateManifest | ConvertFrom-Json
	Assert-Equal $rcUpdateManifest.channel 'preview' 'Release-candidate update manifest must identify the preview channel.'
  Assert-Throws { & "$PSScriptRoot\stage-release-assets.ps1" -Version '1.2.4-rc.1' -ProductVersion '1.2.3' -InstallerPath $installer -ExtensionPackagePath $extensionPackage -OutputDirectory (Join-Path $temporaryRoot 'invalid-release-candidate') } 'Release-candidate asset version/product mismatches must be rejected.'

  $workflow = Get-Content -Raw -Encoding utf8 -LiteralPath (Join-Path $root '.github\workflows\release.yml')
  foreach ($required in @('self-hosted', 'windows', 'fluxdm-signing', 'environment: release', 'fail_on_unmatched_files: true', 'generate_release_notes: true', 'FluxDM-${{ steps.version.outputs.version }}-windows-amd64-installer.exe', 'FluxDM-${{ steps.version.outputs.version }}-browser-extension.zip', 'SHA256SUMS.txt', 'release-manifest.json', 'update-manifest.json', 'update-manifest.sig', 'FLUXDM_UPDATE_STABLE_PRIVATE_KEY')) {
    Assert-True ($workflow.Contains($required)) "Release workflow is missing required contract: $required"
  }

  $rcWorkflow = Get-Content -Raw -Encoding utf8 -LiteralPath (Join-Path $root '.github\workflows\rc-release.yml')
  foreach ($required in @("'v*-rc.*'", 'runs-on: windows-2022', 'prerelease: true', 'not Authenticode-signed', 'fail_on_unmatched_files: true', 'FluxDM-${{ steps.version.outputs.release_version }}-windows-amd64-installer.exe', 'FluxDM-${{ steps.version.outputs.release_version }}-browser-extension.zip', 'SHA256SUMS.txt', 'release-manifest.json', 'update-manifest.json', 'update-manifest.sig', 'FLUXDM_UPDATE_PREVIEW_PRIVATE_KEY')) {
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
	Assert-True ($buildScript.Contains('FluxDM.UpdateLauncher.exe')) 'Release build must package the update launcher.'

  $installerScript = Get-Content -Raw -Encoding utf8 -LiteralPath (Join-Path $root 'build\windows\installer\project.nsi')
  foreach ($required in @('Icon "..\icon.ico"', 'UninstallIcon "..\icon.ico"')) {
    Assert-True ($installerScript.Contains($required)) "Installer must embed the FluxDM icon in its executable: $required"
  }
  foreach ($required in @('WriteRegStr HKLM "Software\BraveSoftware\Brave-Browser\NativeMessagingHosts\com.fluxdm.browser"', 'WriteRegStr HKCU "Software\Google\Chrome\NativeMessagingHosts\com.fluxdm.browser"', 'WriteRegStr HKCU "Software\Microsoft\Edge\NativeMessagingHosts\com.fluxdm.browser"', 'WriteRegStr HKCU "Software\BraveSoftware\Brave-Browser\NativeMessagingHosts\com.fluxdm.browser"', 'DeleteRegKey HKLM "Software\BraveSoftware\Brave-Browser\NativeMessagingHosts\com.fluxdm.browser"', 'DeleteRegKey HKCU "Software\Google\Chrome\NativeMessagingHosts\com.fluxdm.browser"', 'DeleteRegKey HKCU "Software\Microsoft\Edge\NativeMessagingHosts\com.fluxdm.browser"', 'DeleteRegKey HKCU "Software\BraveSoftware\Brave-Browser\NativeMessagingHosts\com.fluxdm.browser"')) {
    Assert-True ($installerScript.Contains($required)) "Installer must repair and remove the current-user native-host registration: $required"
  }

  Write-Host 'Release automation checks passed.'
} finally {
  if (Test-Path -LiteralPath $temporaryRoot) { Remove-Item -LiteralPath $temporaryRoot -Recurse -Force }
}
