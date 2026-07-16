[CmdletBinding()]
param()
$ErrorActionPreference = 'Stop'
foreach ($key in @('HKCU:\Software\Google\Chrome\NativeMessagingHosts\com.fluxdm.browser','HKCU:\Software\Microsoft\Edge\NativeMessagingHosts\com.fluxdm.browser')) { if (Test-Path $key) { Remove-Item -LiteralPath $key -Recurse -Force } }
$installDir = Join-Path $env:LOCALAPPDATA 'FluxDM\BrowserIntegration'
if (Test-Path -LiteralPath $installDir) { Remove-Item -LiteralPath $installDir -Recurse -Force }
Write-Host 'FluxDM browser integration removed.'
