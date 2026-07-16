[CmdletBinding()]
param(
  [Parameter(Mandatory)][string]$InstallerPath,
  [string]$ReportPath,
  [string]$ExpectedInstallerSHA256,
  [switch]$RequireSignatures
)
$ErrorActionPreference = 'Stop'
$installer = (Resolve-Path -LiteralPath $InstallerPath).Path
$installDir = Join-Path $env:ProgramFiles 'FluxDM\FluxDM'
$uninstallKey = 'HKLM:\Software\Microsoft\Windows\CurrentVersion\Uninstall\FluxDMFluxDM'
$nativeKeys = @(
  'HKLM:\Software\Google\Chrome\NativeMessagingHosts\com.fluxdm.browser',
  'HKLM:\Software\Microsoft\Edge\NativeMessagingHosts\com.fluxdm.browser'
)
$userData = Join-Path $env:APPDATA 'FluxDM'
$smokeID = [guid]::NewGuid().ToString('N')
$downloadSentinel = Join-Path ([Environment]::GetFolderPath('UserProfile')) "Downloads\FluxDM-installer-smoke-preserve-$smokeID.txt"
$dataSentinel = Join-Path $userData "installer-smoke-preserve-$smokeID.txt"
$appShortcut = Join-Path $env:ProgramData 'Microsoft\Windows\Start Menu\Programs\FluxDM.lnk'
$setupShortcut = Join-Path $env:ProgramData 'Microsoft\Windows\Start Menu\Programs\FluxDM Browser Extension Setup.lnk'
$desktopShortcut = Join-Path ([Environment]::GetFolderPath('CommonDesktopDirectory')) 'FluxDM.lnk'
$reportFile = if ($ReportPath) {
  if ([IO.Path]::IsPathRooted($ReportPath)) { [IO.Path]::GetFullPath($ReportPath) } else { [IO.Path]::GetFullPath((Join-Path (Get-Location) $ReportPath)) }
} else { $null }
$os = Get-CimInstance Win32_OperatingSystem
$installerSignature = Get-AuthenticodeSignature -LiteralPath $installer
$report = [ordered]@{
  startedAt = (Get-Date).ToUniversalTime().ToString('o')
  completedAt = $null
  result = 'running'
  error = $null
  machine = [ordered]@{
    productName = $os.Caption
    version = $os.Version
    build = $os.BuildNumber
    architecture = $os.OSArchitecture
  }
  installer = [ordered]@{
    path = $installer
    sha256 = (Get-FileHash -Algorithm SHA256 -LiteralPath $installer).Hash
    signature = $installerSignature.Status.ToString()
    signer = if ($installerSignature.SignerCertificate) { $installerSignature.SignerCertificate.Subject } else { $null }
  }
  checks = [ordered]@{}
}
$app = $null

function Assert-Check([string]$Name, [bool]$Condition, [string]$Failure) {
  $report.checks[$Name] = $Condition
  if (-not $Condition) { throw $Failure }
}

function Get-FileEvidence([string]$Path) {
  $signature = Get-AuthenticodeSignature -LiteralPath $Path
  return [ordered]@{
    path = $Path
    bytes = (Get-Item -LiteralPath $Path).Length
    sha256 = (Get-FileHash -Algorithm SHA256 -LiteralPath $Path).Hash
    signature = $signature.Status.ToString()
    signer = if ($signature.SignerCertificate) { $signature.SignerCertificate.Subject } else { $null }
  }
}

try {
  $identity = [Security.Principal.WindowsIdentity]::GetCurrent()
  $principal = [Security.Principal.WindowsPrincipal]::new($identity)
  Assert-Check 'administrator' $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator) 'Run this smoke test from an elevated PowerShell session.'
  Assert-Check 'noExistingInstallDirectory' (-not (Test-Path -LiteralPath $installDir)) "Refusing to overwrite an existing FluxDM directory: $installDir"
  Assert-Check 'noExistingUninstallRegistration' (-not (Test-Path -LiteralPath $uninstallKey)) 'Refusing to overwrite an existing FluxDM uninstall registration.'
  foreach ($key in $nativeKeys) { Assert-Check "noExistingNativeHost:$key" (-not (Test-Path -LiteralPath $key)) "Refusing to overwrite an existing native-host registration: $key" }
  Assert-Check 'noExistingUserData' (-not (Test-Path -LiteralPath $userData)) "Refusing to use a profile with existing FluxDM data: $userData"
  if ($ExpectedInstallerSHA256) { Assert-Check 'expectedInstallerHash' ($report.installer.sha256 -eq $ExpectedInstallerSHA256.Trim().ToUpperInvariant()) 'Installer SHA-256 does not match the expected value.' }
  if ($RequireSignatures) { Assert-Check 'installerSignature' ($installerSignature.Status -eq 'Valid') "Installer signature is not valid: $($installerSignature.Status)" }

  [IO.File]::WriteAllText($downloadSentinel, 'must survive uninstall', [Text.UTF8Encoding]::new($false))
  $install = Start-Process -FilePath $installer -ArgumentList '/S' -Wait -PassThru -WindowStyle Hidden
  Assert-Check 'silentInstallExitCode' ($install.ExitCode -eq 0) "Installer exit code $($install.ExitCode)"

  $installedFiles = @(
    'FluxDM.exe',
    'FluxDM.NativeHost.exe',
    'com.fluxdm.browser.json',
    'uninstall.exe',
    'browser-extension\manifest.json',
    'browser-extension\install.html'
  )
  foreach ($file in $installedFiles) { Assert-Check "installedFile:$file" (Test-Path -LiteralPath (Join-Path $installDir $file)) "Missing installed file: $file" }
  foreach ($shortcut in @($appShortcut,$setupShortcut,$desktopShortcut)) { Assert-Check "shortcut:$shortcut" (Test-Path -LiteralPath $shortcut) "Missing installed shortcut: $shortcut" }

  $hostManifestPath = Join-Path $installDir 'com.fluxdm.browser.json'
  $hostManifest = Get-Content -Raw -Encoding utf8 -LiteralPath $hostManifestPath | ConvertFrom-Json
  $expectedHostPath = Join-Path $installDir 'FluxDM.NativeHost.exe'
  Assert-Check 'nativeManifestName' ($hostManifest.name -eq 'com.fluxdm.browser') 'Installed native-host manifest has the wrong name.'
  Assert-Check 'nativeManifestPath' ([IO.Path]::GetFullPath($hostManifest.path) -eq [IO.Path]::GetFullPath($expectedHostPath)) 'Installed native-host manifest has the wrong executable path.'
  Assert-Check 'nativeManifestOrigin' (@($hostManifest.allowed_origins).Count -eq 1 -and $hostManifest.allowed_origins[0] -eq 'chrome-extension://hnemapnmnkccfommbacamppclohhcbfn/') 'Installed native-host manifest has an unexpected allowed origin.'
  foreach ($key in $nativeKeys) {
    Assert-Check "nativeRegistration:$key" (Test-Path -LiteralPath $key) "Missing native host registration: $key"
    $registeredManifest = (Get-Item -LiteralPath $key).GetValue('', $null, [Microsoft.Win32.RegistryValueOptions]::DoNotExpandEnvironmentNames)
    Assert-Check "nativeRegistrationValue:$key" ([IO.Path]::GetFullPath($registeredManifest) -eq [IO.Path]::GetFullPath($hostManifestPath)) "Native-host registration points to the wrong manifest: $key"
  }

  $appPath = Join-Path $installDir 'FluxDM.exe'
  $nativeHostPath = Join-Path $installDir 'FluxDM.NativeHost.exe'
  $uninstallerPath = Join-Path $installDir 'uninstall.exe'
  $report.installedArtifacts = @(
    Get-FileEvidence $appPath
    Get-FileEvidence $nativeHostPath
    Get-FileEvidence $uninstallerPath
  )
  if ($RequireSignatures) {
    foreach ($artifact in $report.installedArtifacts) { Assert-Check "installedSignature:$($artifact.path)" ($artifact.signature -eq 'Valid') "Installed artifact signature is not valid: $($artifact.path) ($($artifact.signature))" }
  }

  $app = Start-Process -FilePath $appPath -PassThru
  Start-Sleep -Seconds 5
  $app.Refresh()
  Assert-Check 'installedApplicationRunning' (-not $app.HasExited) 'Installed application exited during startup smoke test.'
  Assert-Check 'installedApplicationResponding' $app.Responding 'Installed application is not responding.'
  Assert-Check 'installedApplicationWindow' ($app.MainWindowHandle -ne 0) 'Installed application did not create a desktop window.'

  Assert-Check 'userDataCreated' (Test-Path -LiteralPath $userData) 'FluxDM did not create its user-data directory.'
  [IO.File]::WriteAllText($dataSentinel, 'must survive default uninstall', [Text.UTF8Encoding]::new($false))
  $uninstall = Start-Process -FilePath $uninstallerPath -ArgumentList '/S' -Wait -PassThru -WindowStyle Hidden
  Assert-Check 'silentUninstallExitCode' ($uninstall.ExitCode -eq 0) "Uninstaller exit code $($uninstall.ExitCode)"
  $applicationStopped = $app.WaitForExit(5000)
  Assert-Check 'uninstallerStoppedRunningApplication' $applicationStopped 'Uninstaller left the running FluxDM process alive.'

  Assert-Check 'installDirectoryRemoved' (-not (Test-Path -LiteralPath $installDir)) 'Install directory survived uninstall.'
  Assert-Check 'uninstallRegistrationRemoved' (-not (Test-Path -LiteralPath $uninstallKey)) 'Uninstall registration survived uninstall.'
  foreach ($key in $nativeKeys) { Assert-Check "nativeRegistrationRemoved:$key" (-not (Test-Path -LiteralPath $key)) "Native host registration survived uninstall: $key" }
  foreach ($shortcut in @($appShortcut,$setupShortcut,$desktopShortcut)) { Assert-Check "shortcutRemoved:$shortcut" (-not (Test-Path -LiteralPath $shortcut)) "Shortcut survived uninstall: $shortcut" }
  Assert-Check 'downloadPreserved' (Test-Path -LiteralPath $downloadSentinel) 'Uninstaller deleted the Downloads sentinel.'
  Assert-Check 'defaultUserDataPreserved' (Test-Path -LiteralPath $dataSentinel) 'Default silent uninstall deleted FluxDM user data.'

  $report.result = 'passed'
  Write-Host 'Installer smoke test passed.'
} catch {
  $report.result = 'failed'
  $report.error = $_.Exception.Message
  throw
} finally {
  $report.completedAt = (Get-Date).ToUniversalTime().ToString('o')
  if ($app -and -not $app.HasExited) { Stop-Process -Id $app.Id -Force -ErrorAction SilentlyContinue }
  if (Test-Path -LiteralPath $downloadSentinel) { Remove-Item -LiteralPath $downloadSentinel -Force }
  if ($reportFile) {
    $reportDirectory = Split-Path -Parent $reportFile
    if (-not (Test-Path -LiteralPath $reportDirectory)) { New-Item -ItemType Directory -Path $reportDirectory -Force | Out-Null }
    [IO.File]::WriteAllText($reportFile, ($report | ConvertTo-Json -Depth 8), [Text.UTF8Encoding]::new($false))
  }
}
