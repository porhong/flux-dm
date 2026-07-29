[CmdletBinding()]
param([switch]$Sign,[string]$Version,[string]$ReleaseVersion,[string]$CertificateThumbprint,[string]$SignToolPath='signtool.exe',[string]$MakeNSISPath='makensis.exe',[string]$GCCPath='gcc.exe',[string]$SevenZipPath,[string]$TimestampUrl,[string]$ReleaseOutputDirectory,[switch]$AllowUntimestampedTestSignature,[string]$UpdateManifestPrivateKey,[string]$UpdateStablePublicKey,[string]$UpdatePreviewPublicKey)
$ErrorActionPreference='Stop'
$root=Split-Path -Parent $PSScriptRoot
Push-Location $root
try{
  $productVersion=& "$PSScriptRoot\get-product-version.ps1" -ExpectedVersion $Version
  if(-not $ReleaseVersion){$ReleaseVersion=$productVersion}
  if($ReleaseVersion -notmatch '^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-rc\.[1-9][0-9]*)?$'){throw "-ReleaseVersion must be X.Y.Z or X.Y.Z-rc.N: '$ReleaseVersion'"}
  if(($ReleaseVersion -replace '-rc\.[1-9][0-9]*$', '') -ne $productVersion){throw "-ReleaseVersion '$ReleaseVersion' does not match product version '$productVersion'."}
  if($Sign -and $ReleaseVersion -ne $productVersion){throw 'Signed releases must use the exact product version; release-candidate versions cannot be signed through this workflow.'}
  if(-not $ReleaseOutputDirectory){$ReleaseOutputDirectory=Join-Path $root 'build\release'}
  if(-not $UpdateManifestPrivateKey){$UpdateManifestPrivateKey=$env:FLUXDM_UPDATE_MANIFEST_PRIVATE_KEY}
  if(-not $UpdateStablePublicKey){$UpdateStablePublicKey=$env:FLUXDM_UPDATE_STABLE_PUBLIC_KEY}
  if(-not $UpdatePreviewPublicKey){$UpdatePreviewPublicKey=$env:FLUXDM_UPDATE_PREVIEW_PUBLIC_KEY}
  $versionOutput=& go version
  if($versionOutput -notmatch 'go([0-9]+\.[0-9]+\.[0-9]+)'){throw "Could not parse Go version: $versionOutput"};if([version]$Matches[1] -lt [version]'1.26.5'){throw "Go 1.26.5 or newer is required; found $versionOutput"}
  $makeNsisCommand=(Get-Command $MakeNSISPath -ErrorAction Stop).Source;$env:PATH="$(Split-Path -Parent $makeNsisCommand);$env:PATH"
  $gccCommand=(Get-Command $GCCPath -ErrorAction Stop).Source;$env:PATH="$(Split-Path -Parent $gccCommand);$env:PATH"
  # main.go embeds dist/, so generate it before any Go command resolves the
  # package graph. A clean GitHub runner does not contain generated assets.
  Push-Location frontend
  try{npm ci;if($LASTEXITCODE){throw 'npm ci failed'};npm run lint;if($LASTEXITCODE){throw 'frontend lint failed'};npm run typecheck;if($LASTEXITCODE){throw 'frontend typecheck failed'};npm run test;if($LASTEXITCODE){throw 'frontend tests failed'};npm audit --audit-level=high;if($LASTEXITCODE){throw 'npm audit failed'};npm run build;if($LASTEXITCODE){throw 'frontend build failed'}}finally{Pop-Location}
  go fmt ./...;if($LASTEXITCODE){throw 'go fmt failed'}
  go vet ./...;if($LASTEXITCODE){throw 'go vet failed'}
  go test ./...;if($LASTEXITCODE){throw 'go test failed'}
  go mod verify;if($LASTEXITCODE){throw 'Go module verification failed'}
  go run golang.org/x/vuln/cmd/govulncheck@v1.1.4 ./...;if($LASTEXITCODE){throw 'Go vulnerability scan failed'}
  $originalCGO=$env:CGO_ENABLED
  try{$env:CGO_ENABLED='1';go test -race ./...;if($LASTEXITCODE){throw 'go race test failed'}}finally{$env:CGO_ENABLED=$originalCGO}
  node --check browser-extension\service-worker.js;if($LASTEXITCODE){throw 'browser extension syntax check failed'}
  node --check browser-extension\options.js;if($LASTEXITCODE){throw 'browser extension options syntax check failed'}
  node --check scripts\browser-extension-smoke-driver.mjs;if($LASTEXITCODE){throw 'browser extension smoke driver syntax check failed'}
  node --test browser-extension\policy.test.cjs;if($LASTEXITCODE){throw 'browser extension policy tests failed'}
  $buildLDFlags="-s -w -X github.com/fluxdm/fluxdm/internal/application.ReleaseVersion=$ReleaseVersion"
  if($UpdateStablePublicKey -and $UpdatePreviewPublicKey){$buildLDFlags="$buildLDFlags -X github.com/fluxdm/fluxdm/internal/application.StableUpdatePublicKeyBase64=$UpdateStablePublicKey -X github.com/fluxdm/fluxdm/internal/application.PreviewUpdatePublicKeyBase64=$UpdatePreviewPublicKey"}
  $env:FLUXDM_BUILD_LDFLAGS="$buildLDFlags -H windowsgui"
  wails3 build;if($LASTEXITCODE){throw 'Wails v3 build failed'}
  go build -trimpath -ldflags '-s -w' -o build\bin\FluxDM.NativeHost.exe .\cmd\fluxdm-native-host;if($LASTEXITCODE){throw 'Native host build failed'}
  go build -trimpath -ldflags '-s -w' -o build\bin\FluxDM.UpdateLauncher.exe .\cmd\fluxdm-update-launcher;if($LASTEXITCODE){throw 'Update launcher build failed'}
  $extensionPackage=Join-Path $root 'build\bin\FluxDM-browser-extension.zip'
  & "$PSScriptRoot\package-browser-extension.ps1" -ExpectedVersion $productVersion -OutputPath $extensionPackage -Force;if($LASTEXITCODE){throw 'Browser extension package build failed'}
  $installerPath=Join-Path $root 'build\bin\FluxDM-amd64-installer.exe';if(Test-Path -LiteralPath $installerPath){Remove-Item -LiteralPath $installerPath -Force}
  wails3 generate webview2bootstrapper -dir build\windows\installer;if($LASTEXITCODE){throw 'WebView2 bootstrapper generation failed'}
  Push-Location build\windows\installer
  try{& $makeNsisCommand /WX "-DARG_WAILS_AMD64_BINARY=..\..\bin\FluxDM.exe" project.nsi;if($LASTEXITCODE -or -not(Test-Path -LiteralPath $installerPath)){throw 'Strict NSIS build failed'}}finally{Pop-Location}
  $app=Join-Path $root 'build\bin\FluxDM.exe';$nativeHost=Join-Path $root 'build\bin\FluxDM.NativeHost.exe';$updateLauncher=Join-Path $root 'build\bin\FluxDM.UpdateLauncher.exe'
  & "$PSScriptRoot\verify-version-metadata.ps1" -Path @($app,$installerPath) -Version $productVersion
  if($Sign){if(-not $CertificateThumbprint){$CertificateThumbprint=$env:FLUXDM_CERT_THUMBPRINT};if(-not $TimestampUrl){$TimestampUrl=$env:FLUXDM_TIMESTAMP_URL};if(-not $CertificateThumbprint){throw '-CertificateThumbprint or FLUXDM_CERT_THUMBPRINT is required with -Sign'};if($AllowUntimestampedTestSignature -and $TimestampUrl){throw '-AllowUntimestampedTestSignature requires an empty -TimestampUrl.'};if(-not $TimestampUrl -and -not $AllowUntimestampedTestSignature){throw 'Production signatures require an RFC 3161 timestamp URL.'};if(-not $SevenZipPath){throw '-SevenZipPath is required for signed release builds.'};$signToolCommand=(Get-Command $SignToolPath -ErrorAction Stop).Source;& "$PSScriptRoot\sign-release.ps1" -Path @($app,$nativeHost,$updateLauncher) -CertificateThumbprint $CertificateThumbprint -SignToolPath $signToolCommand -TimestampUrl $TimestampUrl -AllowUntimestampedTestSignature:$AllowUntimestampedTestSignature;$env:FLUXDM_SIGNTOOL=$signToolCommand;$env:FLUXDM_CERT_THUMBPRINT=$CertificateThumbprint;$env:FLUXDM_TIMESTAMP_URL=$TimestampUrl;Push-Location build\windows\installer;try{if($AllowUntimestampedTestSignature){& $makeNsisCommand /WX -DFLUXDM_SIGN_INSTALLER -DFLUXDM_TEST_UNTIMESTAMPED "-DARG_WAILS_AMD64_BINARY=..\..\bin\FluxDM.exe" project.nsi}else{& $makeNsisCommand /WX -DFLUXDM_SIGN_INSTALLER "-DARG_WAILS_AMD64_BINARY=..\..\bin\FluxDM.exe" project.nsi};if($LASTEXITCODE){throw 'Signed NSIS rebuild failed'}}finally{Pop-Location}}
  $installer=Get-ChildItem build\bin\FluxDM-amd64-installer.exe -ErrorAction Stop
  $artifacts=@($app,$nativeHost,$updateLauncher,$installer.FullName);if($Sign){& "$PSScriptRoot\verify-release.ps1" -Path $artifacts -SignToolPath $SignToolPath}
  if($SevenZipPath){& "$PSScriptRoot\verify-installer-payload.ps1" -InstallerPath $installer.FullName -SevenZipPath $SevenZipPath -AppPath $app -NativeHostPath $nativeHost -UpdateLauncherPath $updateLauncher -ExtensionPath (Join-Path $root 'browser-extension') -Version $productVersion -RequireSignatures:$Sign}
  & "$PSScriptRoot\stage-release-assets.ps1" -Version $ReleaseVersion -ProductVersion $productVersion -InstallerPath $installer.FullName -ExtensionPackagePath $extensionPackage -OutputDirectory $ReleaseOutputDirectory -Signed:$Sign
  Write-Host "Release assets created in $ReleaseOutputDirectory"
}finally{Pop-Location}
