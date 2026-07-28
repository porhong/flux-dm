[CmdletBinding()]
param(
  [string]$ExtensionPath,
  [Parameter(Mandatory)][string]$ExpectedVersion,
  [Parameter(Mandatory)][string]$OutputPath,
  [switch]$Force
)

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
if (-not $ExtensionPath) { $ExtensionPath = Join-Path $root 'browser-extension' }

function Assert-ContainedPath([string]$Root, [string]$Candidate, [string]$Message) {
  $rootURI = [Uri]([IO.Path]::GetFullPath($Root).TrimEnd('\') + '\')
  $candidateURI = [Uri]([IO.Path]::GetFullPath($Candidate))
  $relativePath = $rootURI.MakeRelativeUri($candidateURI).ToString()
  if ($relativePath -eq '..' -or $relativePath.StartsWith('../', [StringComparison]::Ordinal) -or $relativePath -match '^[a-z][a-z0-9+.-]*:') { throw $Message }
}

if ($ExpectedVersion -notmatch '^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$') {
  throw "ExpectedVersion must be X.Y.Z: '$ExpectedVersion'"
}

$extension = (Resolve-Path -LiteralPath $ExtensionPath -ErrorAction Stop).Path
$manifestPath = Join-Path $extension 'manifest.json'
if (-not (Test-Path -LiteralPath $manifestPath)) { throw "Extension manifest is missing: $manifestPath" }
$manifest = Get-Content -Raw -Encoding utf8 -LiteralPath $manifestPath | ConvertFrom-Json
if ($manifest.manifest_version -ne 3 -or $manifest.version -ne $ExpectedVersion -or -not $manifest.key) {
  throw 'Browser extension manifest must be Manifest V3, contain the fixed public key, and match ExpectedVersion.'
}
if (@($manifest.permissions) -notcontains 'nativeMessaging') { throw 'Browser extension manifest must request nativeMessaging permission.' }

$output = [IO.Path]::GetFullPath($OutputPath)
$outputDirectory = Split-Path -Parent $output
if (-not $outputDirectory) { throw "OutputPath must include a directory: '$OutputPath'" }
New-Item -ItemType Directory -Force -Path $outputDirectory | Out-Null
if ((Test-Path -LiteralPath $output) -and -not $Force) { throw "Output package already exists: $output" }

$temporaryRoot = Join-Path $env:TEMP ('fluxdm-extension-package-' + [guid]::NewGuid().ToString('N'))
$stagingDirectory = Join-Path $temporaryRoot 'extension'
try {
  New-Item -ItemType Directory -Force -Path $stagingDirectory | Out-Null
  $excludedRelativePaths = @('README.md', 'policy.test.cjs', 'native-host')
  $sourceFiles = @(Get-ChildItem -LiteralPath $extension -Recurse -File -Force)
  if ($sourceFiles.Count -eq 0) { throw 'Browser extension directory contains no files.' }
  foreach ($sourceFile in $sourceFiles) {
    $relativePath = $sourceFile.FullName.Substring($extension.Length + 1)
    if ($excludedRelativePaths | Where-Object { $relativePath -eq $_ -or $relativePath.StartsWith($_ + '\', [StringComparison]::OrdinalIgnoreCase) }) { continue }
    $destination = Join-Path $stagingDirectory $relativePath
    $resolvedDestination = [IO.Path]::GetFullPath($destination)
    Assert-ContainedPath $stagingDirectory $resolvedDestination "Refusing an extension file outside the package staging directory: $relativePath"
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $resolvedDestination) | Out-Null
    Copy-Item -LiteralPath $sourceFile.FullName -Destination $resolvedDestination -Force
  }
  if (-not (Test-Path -LiteralPath (Join-Path $stagingDirectory 'manifest.json'))) { throw 'Browser extension package is missing manifest.json.' }
  if (Test-Path -LiteralPath $output) { Remove-Item -LiteralPath $output -Force }
  Add-Type -AssemblyName System.IO.Compression.FileSystem
  [IO.Compression.ZipFile]::CreateFromDirectory($stagingDirectory, $output, [IO.Compression.CompressionLevel]::Optimal, $false)
  if (-not (Test-Path -LiteralPath $output)) { throw 'Browser extension package was not created.' }
  [ordered]@{
    package = $output
    sha256 = (Get-FileHash -Algorithm SHA256 -LiteralPath $output).Hash.ToLowerInvariant()
    bytes = (Get-Item -LiteralPath $output).Length
    version = $ExpectedVersion
  } | ConvertTo-Json -Depth 3
} finally {
  if (Test-Path -LiteralPath $temporaryRoot) { Remove-Item -LiteralPath $temporaryRoot -Recurse -Force }
}
