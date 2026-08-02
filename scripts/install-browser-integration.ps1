[CmdletBinding()]
param(
  [string]$ApplicationRoot
)
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
$portableApps = @(Get-ChildItem -LiteralPath $root -File -Filter 'FluxDM-*-windows-amd64-portable.exe' -ErrorAction SilentlyContinue)
$defaultApplicationRoot = if (Test-Path -LiteralPath (Join-Path $root 'FluxDM.exe')) { $root } elseif ($portableApps.Count -eq 1) { $root } else { Join-Path $root 'build\bin' }
if (-not $ApplicationRoot) { $ApplicationRoot = $defaultApplicationRoot }
$applicationDirectory = (Resolve-Path -LiteralPath $ApplicationRoot -ErrorAction Stop).Path
$desktopPath = Join-Path $applicationDirectory 'FluxDM.exe'
$portableApps = @(Get-ChildItem -LiteralPath $applicationDirectory -File -Filter 'FluxDM-*-windows-amd64-portable.exe' -ErrorAction SilentlyContinue)
if (-not (Test-Path -LiteralPath $desktopPath) -and $portableApps.Count -eq 1) { $desktopPath = $portableApps[0].FullName }
$nativeHostPath = Join-Path $applicationDirectory 'FluxDM.NativeHost.exe'
if (-not (Test-Path -LiteralPath $desktopPath) -or -not (Test-Path -LiteralPath $nativeHostPath)) {
	throw "ApplicationRoot must contain FluxDM.exe or one versioned portable EXE, plus FluxDM.NativeHost.exe: $applicationDirectory"
}
$integrationDirectory = Join-Path $applicationDirectory 'data\browser-integration'
New-Item -ItemType Directory -Force -Path $integrationDirectory | Out-Null
$hostPath = $nativeHostPath.Replace('\','\\')
$templatePath = Join-Path $root 'browser-integration\com.fluxdm.browser.template.json'
if (-not (Test-Path -LiteralPath $templatePath)) { $templatePath = Join-Path $root 'browser-extension\native-host\com.fluxdm.browser.template.json' }
$template = Get-Content -Raw -LiteralPath $templatePath
$manifestPath = Join-Path $integrationDirectory 'com.fluxdm.browser.json'
[IO.File]::WriteAllText($manifestPath, $template.Replace('@@NATIVE_HOST_PATH@@',$hostPath), [Text.UTF8Encoding]::new($false))
foreach ($key in @('HKCU:\Software\Google\Chrome\NativeMessagingHosts\com.fluxdm.browser','HKCU:\Software\Microsoft\Edge\NativeMessagingHosts\com.fluxdm.browser','HKCU:\Software\BraveSoftware\Brave-Browser\NativeMessagingHosts\com.fluxdm.browser')) {
  New-Item -Force -Path $key | Out-Null
  Set-Item -Path $key -Value $manifestPath
}
Write-Host "FluxDM browser integration enabled for the current user. Load unpacked: $root\browser-extension"

