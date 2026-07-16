[CmdletBinding()]
param(
  [string]$InstallerPath = (Join-Path (Split-Path -Parent $PSScriptRoot) 'build\bin\FluxDM-amd64-installer.exe'),
  [string]$ReportPath,
  [string]$ExpectedInstallerSHA256,
  [switch]$RequireSignatures
)
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
$installer = (Resolve-Path -LiteralPath $InstallerPath).Path
if (-not $ReportPath) { $ReportPath = Join-Path $root ('build\bin\installer-smoke-' + (Get-Date).ToUniversalTime().ToString('yyyyMMddTHHmmssZ') + '.json') }
$report = if ([IO.Path]::IsPathRooted($ReportPath)) { [IO.Path]::GetFullPath($ReportPath) } else { [IO.Path]::GetFullPath((Join-Path (Get-Location) $ReportPath)) }
$reportDirectory = Split-Path -Parent $report
if (-not (Test-Path -LiteralPath $reportDirectory)) { New-Item -ItemType Directory -Path $reportDirectory -Force | Out-Null }
$smokeScript = Join-Path $PSScriptRoot 'smoke-test-installer.ps1'
$identity = [Security.Principal.WindowsIdentity]::GetCurrent()
$principal = [Security.Principal.WindowsPrincipal]::new($identity)

if ($principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
  & $smokeScript -InstallerPath $installer -ReportPath $report -ExpectedInstallerSHA256 $ExpectedInstallerSHA256 -RequireSignatures:$RequireSignatures
} else {
  $arguments = @(
    '-NoLogo',
    '-NoProfile',
    '-ExecutionPolicy', 'Bypass',
    '-File', "`"$smokeScript`"",
    '-InstallerPath', "`"$installer`"",
    '-ReportPath', "`"$report`""
  )
  if ($ExpectedInstallerSHA256) { $arguments += @('-ExpectedInstallerSHA256', $ExpectedInstallerSHA256) }
  if ($RequireSignatures) { $arguments += '-RequireSignatures' }
  Write-Host 'Approve the Windows administrator prompt to run the isolated installer/uninstaller smoke test.'
  $process = Start-Process powershell.exe -Verb RunAs -ArgumentList $arguments -Wait -PassThru -WindowStyle Hidden
  if ($process.ExitCode -ne 0) { throw "Elevated installer smoke process exited with code $($process.ExitCode). See $report" }
}

if (-not (Test-Path -LiteralPath $report)) { throw "Installer smoke test did not create its report: $report" }
$result = Get-Content -Raw -Encoding utf8 -LiteralPath $report | ConvertFrom-Json
if ($result.result -ne 'passed') { throw "Installer smoke report is not passing: $($result.error)" }
Write-Host "Installer smoke report: $report"
$result | ConvertTo-Json -Depth 8
