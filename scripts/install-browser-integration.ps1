[CmdletBinding()]
param()
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
$installDir = Join-Path $env:LOCALAPPDATA 'FluxDM\BrowserIntegration'
New-Item -ItemType Directory -Force -Path $installDir | Out-Null
go build -trimpath -o (Join-Path $installDir 'FluxDM.NativeHost.exe') "$root\cmd\fluxdm-native-host"
$desktopSource = Join-Path $root 'build\bin\FluxDM.exe'
if (Test-Path -LiteralPath $desktopSource) { Copy-Item -LiteralPath $desktopSource -Destination (Join-Path $installDir 'FluxDM.exe') -Force }
$hostPath = (Join-Path $installDir 'FluxDM.NativeHost.exe').Replace('\','\\')
$template = Get-Content -Raw -LiteralPath (Join-Path $root 'browser-extension\native-host\com.fluxdm.browser.template.json')
$manifestPath = Join-Path $installDir 'com.fluxdm.browser.json'
[IO.File]::WriteAllText($manifestPath, $template.Replace('@@NATIVE_HOST_PATH@@',$hostPath), [Text.UTF8Encoding]::new($false))
foreach ($key in @('HKCU:\Software\Google\Chrome\NativeMessagingHosts\com.fluxdm.browser','HKCU:\Software\Microsoft\Edge\NativeMessagingHosts\com.fluxdm.browser')) {
  New-Item -Force -Path $key | Out-Null
  Set-Item -Path $key -Value $manifestPath
}
Write-Host "FluxDM native host installed. Load unpacked: $root\browser-extension"

