[CmdletBinding()]
param(
  [Parameter(Mandatory)][string]$Version,
  [string]$ReleaseVersion,
  [string]$ReleaseOutputDirectory
)

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
Push-Location $root
try {
  $productVersion = & "$PSScriptRoot\get-product-version.ps1" -ExpectedVersion $Version
  if (-not $ReleaseVersion) { $ReleaseVersion = $productVersion }
  if ($ReleaseVersion -notmatch '^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-rc\.[1-9][0-9]*)?$') { throw "ReleaseVersion must be X.Y.Z or X.Y.Z-rc.N: '$ReleaseVersion'" }
  if (($ReleaseVersion -replace '-rc\.[1-9][0-9]*$', '') -ne $productVersion) { throw "ReleaseVersion '$ReleaseVersion' does not match product version '$productVersion'." }
  if (-not $ReleaseOutputDirectory) { $ReleaseOutputDirectory = Join-Path $root 'build\release' }

  Push-Location frontend
  try { npm ci; if($LASTEXITCODE){throw 'npm ci failed'}; npm run lint; if($LASTEXITCODE){throw 'frontend lint failed'}; npm run typecheck; if($LASTEXITCODE){throw 'frontend typecheck failed'}; npm run test; if($LASTEXITCODE){throw 'frontend tests failed'}; npm audit --audit-level=high; if($LASTEXITCODE){throw 'npm audit failed'}; npm run build; if($LASTEXITCODE){throw 'frontend build failed'} } finally { Pop-Location }
  go fmt ./...; if($LASTEXITCODE){throw 'go fmt failed'}
  go vet ./...; if($LASTEXITCODE){throw 'go vet failed'}
  go test ./...; if($LASTEXITCODE){throw 'go test failed'}
  go mod verify; if($LASTEXITCODE){throw 'Go module verification failed'}
  go run golang.org/x/vuln/cmd/govulncheck@v1.1.4 ./...; if($LASTEXITCODE){throw 'Go vulnerability scan failed'}
  $originalCGO = $env:CGO_ENABLED
  try { $env:CGO_ENABLED='1'; go test -race ./...; if($LASTEXITCODE){throw 'go race test failed'} } finally { $env:CGO_ENABLED = $originalCGO }
  node --check browser-extension\service-worker.js; if($LASTEXITCODE){throw 'browser extension syntax check failed'}
  node --check browser-extension\options.js; if($LASTEXITCODE){throw 'browser extension options syntax check failed'}
  node --check scripts\browser-extension-smoke-driver.mjs; if($LASTEXITCODE){throw 'browser extension smoke driver syntax check failed'}
  node --test browser-extension\policy.test.cjs; if($LASTEXITCODE){throw 'browser extension policy tests failed'}

  $commonLDFlags = "-s -w -X github.com/fluxdm/fluxdm/internal/application.ReleaseVersion=$ReleaseVersion -X github.com/fluxdm/fluxdm/internal/application.PortableMode=true"
  $env:FLUXDM_BUILD_LDFLAGS = "$commonLDFlags -H windowsgui"
  wails3 build; if($LASTEXITCODE){throw 'Wails v3 build failed'}
  $extensionPackage = Join-Path $root 'build\bin\FluxDM-browser-extension.zip'
  & "$PSScriptRoot\package-browser-extension.ps1" -ExpectedVersion $productVersion -OutputPath $extensionPackage -Force; if($LASTEXITCODE){throw 'Browser extension package build failed'}
  $portableApp = Join-Path $root 'build\bin\FluxDM.exe'
  & "$PSScriptRoot\verify-version-metadata.ps1" -Path $portableApp -Version $productVersion
  & "$PSScriptRoot\stage-portable-release-assets.ps1" -Version $ReleaseVersion -ProductVersion $productVersion -PortablePath $portableApp -ExtensionPackagePath $extensionPackage -OutputDirectory $ReleaseOutputDirectory | Out-Null
  Write-Host "Portable release assets created in $ReleaseOutputDirectory"
} finally { Pop-Location }
