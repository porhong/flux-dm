[CmdletBinding()]
param(
  [Parameter(Mandatory)][ValidateSet('Chrome','Edge')][string]$Browser,
  [Parameter(Mandatory)][string]$BrowserPath,
  [string]$ExtensionPath = (Join-Path (Split-Path -Parent $PSScriptRoot) 'browser-extension'),
  [string]$NativeHostPath = (Join-Path (Split-Path -Parent $PSScriptRoot) 'build\bin\FluxDM.NativeHost.exe'),
  [string]$FluxDMPath = (Join-Path (Split-Path -Parent $PSScriptRoot) 'build\bin\FluxDM.exe'),
  [switch]$ExpectDesktopUnavailable,
  [ValidateRange(5,120)][int]$TimeoutSeconds = 30
)
$ErrorActionPreference = 'Stop'

$extensionID = 'hnemapnmnkccfommbacamppclohhcbfn'
$browserExecutable = (Resolve-Path -LiteralPath $BrowserPath).Path
$extensionDirectory = (Resolve-Path -LiteralPath $ExtensionPath).Path
$nativeHostExecutable = (Resolve-Path -LiteralPath $NativeHostPath).Path
$desktopExecutable = (Resolve-Path -LiteralPath $FluxDMPath).Path
$node = (Get-Command node -ErrorAction Stop).Source
$driver = Join-Path $PSScriptRoot 'browser-extension-smoke-driver.mjs'
$registryPath = if ($Browser -eq 'Chrome') { 'HKCU:\Software\Google\Chrome\NativeMessagingHosts\com.fluxdm.browser' } else { 'HKCU:\Software\Microsoft\Edge\NativeMessagingHosts\com.fluxdm.browser' }

$temporaryRoot = Join-Path $env:TEMP ('fluxdm-browser-smoke-' + [guid]::NewGuid().ToString('N'))
$profileDirectory = Join-Path $temporaryRoot 'browser-profile'
$isolatedAppData = Join-Path $temporaryRoot 'appdata'
$isolatedUserProfile = Join-Path $temporaryRoot 'user-profile'
$isolatedDownloads = Join-Path $isolatedUserProfile 'Downloads'
$manifestPath = Join-Path $temporaryRoot 'com.fluxdm.browser.json'
$browserLog = Join-Path $temporaryRoot 'browser-stderr.log'
$browserProcess = $null
$desktopProcess = $null
$registryExisted = Test-Path -LiteralPath $registryPath
$previousRegistryValue = if ($registryExisted) { (Get-Item -LiteralPath $registryPath).GetValue('', $null, [Microsoft.Win32.RegistryValueOptions]::DoNotExpandEnvironmentNames) } else { $null }
$originalAppData = $env:APPDATA
$originalUserProfile = $env:USERPROFILE

function Stop-FluxDMProcessTree([int]$RootProcessID) {
  $processes = @(Get-CimInstance Win32_Process -ErrorAction SilentlyContinue | Select-Object ProcessId,ParentProcessId)
  $ordered = [Collections.Generic.List[int]]::new()
  function Add-Descendants([int]$ParentID) {
    foreach ($child in $processes | Where-Object ParentProcessId -eq $ParentID) {
      Add-Descendants $child.ProcessId
      $ordered.Add([int]$child.ProcessId)
    }
  }
  Add-Descendants $RootProcessID
  $ordered.Add($RootProcessID)
  foreach ($processID in $ordered) { Stop-Process -Id $processID -Force -ErrorAction SilentlyContinue }
}

New-Item -ItemType Directory -Path $profileDirectory,$isolatedAppData,$isolatedDownloads -Force | Out-Null
try {
  $effectiveNativeHost = $nativeHostExecutable
  if ($ExpectDesktopUnavailable) {
    $hostOnlyDirectory = Join-Path $temporaryRoot 'host-only'
    New-Item -ItemType Directory -Path $hostOnlyDirectory -Force | Out-Null
    $effectiveNativeHost = Join-Path $hostOnlyDirectory 'FluxDM.NativeHost.exe'
    Copy-Item -LiteralPath $nativeHostExecutable -Destination $effectiveNativeHost
  }
  $nativeManifest = [ordered]@{
    name = 'com.fluxdm.browser'
    description = 'FluxDM native messaging host smoke test'
    path = $effectiveNativeHost
    type = 'stdio'
    allowed_origins = @("chrome-extension://$extensionID/")
  } | ConvertTo-Json -Depth 3
  [IO.File]::WriteAllText($manifestPath, $nativeManifest, [Text.UTF8Encoding]::new($false))
  New-Item -Path $registryPath -Force | Out-Null
  Set-Item -LiteralPath $registryPath -Value $manifestPath

  $env:APPDATA = $isolatedAppData
  $env:USERPROFILE = $isolatedUserProfile
  if (-not $ExpectDesktopUnavailable) {
    $desktopProcess = Start-Process -FilePath $desktopExecutable -PassThru
    $connectionPath = Join-Path $isolatedAppData 'FluxDM\browser-bridge.json'
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while (-not (Test-Path -LiteralPath $connectionPath) -and (Get-Date) -lt $deadline) {
      if ($desktopProcess.HasExited) { throw "FluxDM exited before creating the browser bridge (exit code $($desktopProcess.ExitCode))." }
      Start-Sleep -Milliseconds 100
    }
    if (-not (Test-Path -LiteralPath $connectionPath)) { throw 'FluxDM did not create the isolated browser bridge before timeout.' }
  }

  $arguments = @(
    '--headless=new',
    '--no-first-run',
    '--no-default-browser-check',
    '--disable-background-networking',
    '--disable-component-update',
    '--disable-sync',
    '--remote-debugging-port=0',
    "--user-data-dir=`"$profileDirectory`"",
    "--disable-extensions-except=`"$extensionDirectory`"",
    "--load-extension=`"$extensionDirectory`"",
    'about:blank'
  )
  $browserProcess = Start-Process -FilePath $browserExecutable -ArgumentList $arguments -RedirectStandardError $browserLog -PassThru -WindowStyle Hidden
  $mode = if ($ExpectDesktopUnavailable) { 'unavailable' } else { 'connected' }
  $driverOutput = & $node $driver $profileDirectory $extensionID $isolatedDownloads $mode ($TimeoutSeconds * 1000)
  if ($LASTEXITCODE -ne 0) {
    $logTail = if (Test-Path -LiteralPath $browserLog) { (Get-Content -LiteralPath $browserLog -Tail 30) -join [Environment]::NewLine } else { '' }
    throw "Browser extension driver failed for $Browser.`n$logTail"
  }
  $driverResult = $driverOutput | ConvertFrom-Json
  [ordered]@{
    browser = $Browser
    browserVersion = (Get-Item -LiteralPath $browserExecutable).VersionInfo.ProductVersion
    isolatedProfile = $true
    isolatedAppData = $true
    isolatedDownloads = $true
    extensionID = $driverResult.extensionID
    nativeConnection = $driverResult.nativeConnection
    nativeTransfer = $driverResult.nativeTransfer
    transferBytes = $driverResult.transferBytes
    transferSHA256 = $driverResult.transferSHA256
    transferRequests = $driverResult.transferRequests
    automaticInterception = $driverResult.automaticInterception
    automaticTransferBytes = $driverResult.automaticTransferBytes
    automaticTransferSHA256 = $driverResult.automaticTransferSHA256
    automaticTransferRequests = $driverResult.automaticTransferRequests
    browserDownloadState = $driverResult.browserDownloadState
    browserDownloadError = $driverResult.browserDownloadError
    unavailableFallback = $driverResult.unavailableFallback
    fallbackBytes = $driverResult.fallbackBytes
    fallbackSHA256 = $driverResult.fallbackSHA256
    fallbackRequests = $driverResult.fallbackRequests
  } | ConvertTo-Json
} finally {
  $env:APPDATA = $originalAppData
  $env:USERPROFILE = $originalUserProfile
  if ($browserProcess) { Stop-FluxDMProcessTree $browserProcess.Id }
  if ($desktopProcess) { Stop-FluxDMProcessTree $desktopProcess.Id }
  if ($registryExisted) {
    New-Item -Path $registryPath -Force | Out-Null
    Set-Item -LiteralPath $registryPath -Value $previousRegistryValue
  } elseif (Test-Path -LiteralPath $registryPath) {
    Remove-Item -LiteralPath $registryPath -Force
  }
  Start-Sleep -Milliseconds 250
  $resolvedTemporaryRoot = [IO.Path]::GetFullPath($temporaryRoot)
  $temporaryPrefix = [IO.Path]::GetFullPath($env:TEMP).TrimEnd('\') + '\'
  if (-not $resolvedTemporaryRoot.StartsWith($temporaryPrefix, [StringComparison]::OrdinalIgnoreCase)) { throw "Refusing cleanup outside TEMP: $resolvedTemporaryRoot" }
  if (Test-Path -LiteralPath $resolvedTemporaryRoot) { Remove-Item -LiteralPath $resolvedTemporaryRoot -Recurse -Force }
}
