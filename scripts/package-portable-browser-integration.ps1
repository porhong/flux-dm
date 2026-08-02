[CmdletBinding()]
param(
  [Parameter(Mandatory)][string]$Version,
  [Parameter(Mandatory)][string]$NativeHostPath,
  [string]$ExtensionPath,
  [Parameter(Mandatory)][string]$OutputPath,
  [switch]$Force
)

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
if ($Version -notmatch '^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-rc\.[1-9][0-9]*)?$') { throw "Version must be X.Y.Z or X.Y.Z-rc.N: '$Version'" }
if (-not $ExtensionPath) { $ExtensionPath = Join-Path $root 'browser-extension' }

function Resolve-RequiredFile([string]$Path, [string]$Label) {
  $resolved = (Resolve-Path -LiteralPath $Path -ErrorAction Stop).Path
  if ((Get-Item -LiteralPath $resolved).PSIsContainer) { throw "$Label must be a file: $resolved" }
  return $resolved
}

$nativeHost = Resolve-RequiredFile $NativeHostPath 'NativeHostPath'
$extension = (Resolve-Path -LiteralPath $ExtensionPath -ErrorAction Stop).Path
if (-not (Test-Path -LiteralPath (Join-Path $extension 'manifest.json'))) { throw 'ExtensionPath is missing manifest.json.' }
$output = [IO.Path]::GetFullPath($OutputPath)
if ([IO.Path]::GetExtension($output) -ne '.zip') { throw 'OutputPath must end in .zip.' }
if ((Test-Path -LiteralPath $output) -and -not $Force) { throw "OutputPath already exists: $output" }
New-Item -ItemType Directory -Force -Path (Split-Path -Parent $output) | Out-Null

$temporaryRoot = Join-Path $env:TEMP ('fluxdm-portable-browser-integration-' + [guid]::NewGuid().ToString('N'))
try {
  New-Item -ItemType Directory -Force -Path $temporaryRoot | Out-Null
  Copy-Item -LiteralPath $nativeHost -Destination (Join-Path $temporaryRoot 'FluxDM.NativeHost.exe')
  $packagedExtension = Join-Path $temporaryRoot 'browser-extension'
  Copy-Item -LiteralPath $extension -Destination $temporaryRoot -Recurse
  foreach ($developmentPath in @('README.md', 'policy.test.cjs', 'native-host')) {
    $candidate = Join-Path $packagedExtension $developmentPath
    $resolvedCandidate = [IO.Path]::GetFullPath($candidate)
    $temporaryPrefix = [IO.Path]::GetFullPath($temporaryRoot).TrimEnd('\') + '\'
    if (-not $resolvedCandidate.StartsWith($temporaryPrefix, [StringComparison]::OrdinalIgnoreCase)) { throw "Refusing cleanup outside package staging: $resolvedCandidate" }
    if (Test-Path -LiteralPath $resolvedCandidate) { Remove-Item -LiteralPath $resolvedCandidate -Recurse -Force }
  }
  if (-not (Test-Path -LiteralPath (Join-Path $packagedExtension 'manifest.json'))) { throw 'Portable browser integration package is missing manifest.json.' }
  New-Item -ItemType Directory -Force -Path (Join-Path $temporaryRoot 'scripts') | Out-Null
  Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'install-browser-integration.ps1') -Destination (Join-Path $temporaryRoot 'scripts\install-browser-integration.ps1')
  Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'uninstall-browser-integration.ps1') -Destination (Join-Path $temporaryRoot 'scripts\uninstall-browser-integration.ps1')
  New-Item -ItemType Directory -Force -Path (Join-Path $temporaryRoot 'browser-integration') | Out-Null
  Copy-Item -LiteralPath (Join-Path $root 'browser-extension\native-host\com.fluxdm.browser.template.json') -Destination (Join-Path $temporaryRoot 'browser-integration\com.fluxdm.browser.template.json')
  @(
    '# FluxDM portable browser integration',
    '',
    '1. Save the versioned FluxDM portable EXE in a user-writable folder.',
    '2. Extract this ZIP into that same folder; do not create an additional top-level folder.',
    '3. Run scripts\install-browser-integration.ps1, then load browser-extension as an unpacked extension in Chrome, Edge, or Brave.',
    '4. Keep exactly one FluxDM-*-windows-amd64-portable.exe in this folder. If you replace it with a newer version, rerun the install script.',
    '',
    'The registration is current-user only and does not require administrator access.'
  ) -join "`r`n" | Set-Content -Encoding utf8 -NoNewline -LiteralPath (Join-Path $temporaryRoot 'README-PORTABLE-BROWSER-INTEGRATION.md')
  if (Test-Path -LiteralPath $output) { Remove-Item -LiteralPath $output -Force }
  Add-Type -AssemblyName System.IO.Compression.FileSystem
  [IO.Compression.ZipFile]::CreateFromDirectory($temporaryRoot, $output, [IO.Compression.CompressionLevel]::Optimal, $false)
  [ordered]@{ package=$output; sha256=(Get-FileHash -Algorithm SHA256 -LiteralPath $output).Hash.ToLowerInvariant(); bytes=(Get-Item -LiteralPath $output).Length; version=$Version } | ConvertTo-Json -Depth 3
} finally {
  if (Test-Path -LiteralPath $temporaryRoot) { Remove-Item -LiteralPath $temporaryRoot -Recurse -Force }
}
