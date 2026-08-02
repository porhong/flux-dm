[CmdletBinding()]
param(
  [string]$ApplicationRoot
)
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
$portableApps = @(Get-ChildItem -LiteralPath $root -File -Filter 'FluxDM-*-windows-amd64-portable.exe' -ErrorAction SilentlyContinue)
if (-not $ApplicationRoot) { $ApplicationRoot = if (Test-Path -LiteralPath (Join-Path $root 'FluxDM.exe')) { $root } elseif ($portableApps.Count -eq 1) { $root } else { Join-Path $root 'build\bin' } }
$applicationDirectory = (Resolve-Path -LiteralPath $ApplicationRoot -ErrorAction Stop).Path
$manifestPath = Join-Path $applicationDirectory 'data\browser-integration\com.fluxdm.browser.json'
foreach ($key in @('HKCU:\Software\Google\Chrome\NativeMessagingHosts\com.fluxdm.browser','HKCU:\Software\Microsoft\Edge\NativeMessagingHosts\com.fluxdm.browser','HKCU:\Software\BraveSoftware\Brave-Browser\NativeMessagingHosts\com.fluxdm.browser')) {
  if (-not (Test-Path -LiteralPath $key)) { continue }
  $registeredManifest = (Get-Item -LiteralPath $key).GetValue('', $null, [Microsoft.Win32.RegistryValueOptions]::DoNotExpandEnvironmentNames)
  if ($registeredManifest -eq $manifestPath) { Remove-Item -LiteralPath $key -Recurse -Force }
}
Write-Host 'FluxDM browser integration was removed for this portable copy.'
