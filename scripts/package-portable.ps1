[CmdletBinding()]
param(
  [Parameter(Mandatory)][string]$Version,
  [Parameter(Mandatory)][string]$AppPath,
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

$app = Resolve-RequiredFile $AppPath 'AppPath'
$nativeHost = Resolve-RequiredFile $NativeHostPath 'NativeHostPath'
$extension = (Resolve-Path -LiteralPath $ExtensionPath -ErrorAction Stop).Path
if (-not (Test-Path -LiteralPath (Join-Path $extension 'manifest.json'))) { throw 'ExtensionPath is missing manifest.json.' }
$output = [IO.Path]::GetFullPath($OutputPath)
if ([IO.Path]::GetExtension($output) -ne '.zip') { throw 'OutputPath must end in .zip.' }
if ((Test-Path -LiteralPath $output) -and -not $Force) { throw "OutputPath already exists: $output" }
New-Item -ItemType Directory -Force -Path (Split-Path -Parent $output) | Out-Null

$temporaryRoot = Join-Path $env:TEMP ('fluxdm-portable-package-' + [guid]::NewGuid().ToString('N'))
$packageRoot = Join-Path $temporaryRoot "FluxDM-$Version"
try {
  New-Item -ItemType Directory -Force -Path $packageRoot | Out-Null
  Copy-Item -LiteralPath $app -Destination (Join-Path $packageRoot 'FluxDM.exe')
  Copy-Item -LiteralPath $nativeHost -Destination (Join-Path $packageRoot 'FluxDM.NativeHost.exe')
  $packagedExtension = Join-Path $packageRoot 'browser-extension'
  foreach ($sourceFile in Get-ChildItem -LiteralPath $extension -Recurse -File) {
    $relativePath = $sourceFile.FullName.Substring($extension.Length + 1)
    if ($relativePath -eq 'README.md' -or $relativePath -eq 'policy.test.cjs' -or $relativePath.StartsWith('native-host\', [StringComparison]::OrdinalIgnoreCase)) { continue }
    $destination = Join-Path $packagedExtension $relativePath
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $destination) | Out-Null
    Copy-Item -LiteralPath $sourceFile.FullName -Destination $destination
  }
  if (-not (Test-Path -LiteralPath (Join-Path $packagedExtension 'manifest.json'))) { throw 'Portable package is missing the browser extension manifest.' }
  @(
    '# FluxDM portable',
    '',
    '1. Extract this folder to a user-writable location, such as Documents or a dedicated folder under LocalAppData.',
    '2. Run FluxDM.exe. Its settings, database, and logs are stored in the adjacent data folder.',
    '3. FluxDM automatically installs its browser integration in LocalAppData. In Settings, choose Set up and open extension folder, then load that folder unpacked in Chrome, Edge, or Brave.',
    '4. If you move this folder, open FluxDM once so it can refresh the browser integration registration.',
    '',
    'The portable application does not require administrator access and does not create Program Files shortcuts or an uninstaller. Replace this extracted folder with a newer verified portable ZIP to update it.'
  ) -join "`r`n" | Set-Content -Encoding utf8 -NoNewline -LiteralPath (Join-Path $packageRoot 'README-PORTABLE.md')
  if (Test-Path -LiteralPath $output) { Remove-Item -LiteralPath $output -Force }
  Add-Type -AssemblyName System.IO.Compression.FileSystem
  [IO.Compression.ZipFile]::CreateFromDirectory($temporaryRoot, $output, [IO.Compression.CompressionLevel]::Optimal, $false)
  [ordered]@{ package=$output; sha256=(Get-FileHash -Algorithm SHA256 -LiteralPath $output).Hash.ToLowerInvariant(); bytes=(Get-Item -LiteralPath $output).Length; version=$Version } | ConvertTo-Json -Depth 3
} finally {
  if (Test-Path -LiteralPath $temporaryRoot) { Remove-Item -LiteralPath $temporaryRoot -Recurse -Force }
}
