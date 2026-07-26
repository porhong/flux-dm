[CmdletBinding()]
param([switch]$Sign,[string]$Version,[string]$CertificateThumbprint,[string]$SignToolPath='signtool.exe',[string]$MakeNSISPath='makensis.exe',[string]$GCCPath='gcc.exe',[string]$SevenZipPath,[string]$TimestampUrl,[string]$ReleaseOutputDirectory,[switch]$AllowUntimestampedTestSignature)
$ErrorActionPreference='Stop'
$root=Split-Path -Parent $PSScriptRoot
Push-Location $root
try{
  $releaseVersion=& "$PSScriptRoot\get-product-version.ps1" -ExpectedVersion $Version
  if(-not $ReleaseOutputDirectory){$ReleaseOutputDirectory=Join-Path $root 'build\release'}
  $versionOutput=& go version
  if($versionOutput -notmatch 'go([0-9]+\.[0-9]+\.[0-9]+)'){throw "Could not parse Go version: $versionOutput"};if([version]$Matches[1] -lt [version]'1.26.5'){throw "Go 1.26.5 or newer is required; found $versionOutput"}
  $makeNsisCommand=(Get-Command $MakeNSISPath -ErrorAction Stop).Source;$env:PATH="$(Split-Path -Parent $makeNsisCommand);$env:PATH"
  $gccCommand=(Get-Command $GCCPath -ErrorAction Stop).Source;$env:PATH="$(Split-Path -Parent $gccCommand);$env:PATH"
  go fmt ./...;if($LASTEXITCODE){throw 'go fmt failed'}
  go vet ./...;if($LASTEXITCODE){throw 'go vet failed'}
  go test ./...;if($LASTEXITCODE){throw 'go test failed'}
  go mod verify;if($LASTEXITCODE){throw 'Go module verification failed'}
  go run golang.org/x/vuln/cmd/govulncheck@v1.1.4 ./...;if($LASTEXITCODE){throw 'Go vulnerability scan failed'}
  $originalCGO=$env:CGO_ENABLED
  try{$env:CGO_ENABLED='1';go test -race ./...;if($LASTEXITCODE){throw 'go race test failed'}}finally{$env:CGO_ENABLED=$originalCGO}
  Push-Location frontend
  try{npm ci;if($LASTEXITCODE){throw 'npm ci failed'};npm run lint;if($LASTEXITCODE){throw 'frontend lint failed'};npm run typecheck;if($LASTEXITCODE){throw 'frontend typecheck failed'};npm run test;if($LASTEXITCODE){throw 'frontend tests failed'};npm audit --audit-level=high;if($LASTEXITCODE){throw 'npm audit failed'}}finally{Pop-Location}
  node --check browser-extension\service-worker.js;if($LASTEXITCODE){throw 'browser extension syntax check failed'}
  node --check browser-extension\options.js;if($LASTEXITCODE){throw 'browser extension options syntax check failed'}
  node --check scripts\browser-extension-smoke-driver.mjs;if($LASTEXITCODE){throw 'browser extension smoke driver syntax check failed'}
  node --test browser-extension\policy.test.cjs;if($LASTEXITCODE){throw 'browser extension policy tests failed'}
  wails build -clean -trimpath -nocolour -ldflags '-s -w';if($LASTEXITCODE){throw 'Wails build failed'}
  go build -trimpath -ldflags '-s -w' -o build\bin\FluxDM.NativeHost.exe .\cmd\fluxdm-native-host;if($LASTEXITCODE){throw 'Native host build failed'}
  $installerPath=Join-Path $root 'build\bin\FluxDM-amd64-installer.exe';if(Test-Path -LiteralPath $installerPath){Remove-Item -LiteralPath $installerPath -Force}
  wails build -nsis -s -skipbindings -trimpath -nocolour -ldflags '-s -w' -webview2 download;if($LASTEXITCODE -or -not(Test-Path -LiteralPath $installerPath)){throw 'NSIS build failed or produced no installer'}
  Push-Location build\windows\installer
  try{& $makeNsisCommand /WX "-DARG_WAILS_AMD64_BINARY=..\..\bin\FluxDM.exe" project.nsi;if($LASTEXITCODE){throw 'Strict NSIS rebuild failed'}}finally{Pop-Location}
  $app=Join-Path $root 'build\bin\FluxDM.exe';$nativeHost=Join-Path $root 'build\bin\FluxDM.NativeHost.exe'
  & "$PSScriptRoot\verify-version-metadata.ps1" -Path @($app,$installerPath) -Version $releaseVersion
  if($Sign){if(-not $CertificateThumbprint){$CertificateThumbprint=$env:FLUXDM_CERT_THUMBPRINT};if(-not $TimestampUrl){$TimestampUrl=$env:FLUXDM_TIMESTAMP_URL};if(-not $CertificateThumbprint){throw '-CertificateThumbprint or FLUXDM_CERT_THUMBPRINT is required with -Sign'};if($AllowUntimestampedTestSignature -and $TimestampUrl){throw '-AllowUntimestampedTestSignature requires an empty -TimestampUrl.'};if(-not $TimestampUrl -and -not $AllowUntimestampedTestSignature){throw 'Production signatures require an RFC 3161 timestamp URL.'};if(-not $SevenZipPath){throw '-SevenZipPath is required for signed release builds.'};$signToolCommand=(Get-Command $SignToolPath -ErrorAction Stop).Source;& "$PSScriptRoot\sign-release.ps1" -Path @($app,$nativeHost) -CertificateThumbprint $CertificateThumbprint -SignToolPath $signToolCommand -TimestampUrl $TimestampUrl -AllowUntimestampedTestSignature:$AllowUntimestampedTestSignature;$env:FLUXDM_SIGNTOOL=$signToolCommand;$env:FLUXDM_CERT_THUMBPRINT=$CertificateThumbprint;$env:FLUXDM_TIMESTAMP_URL=$TimestampUrl;Push-Location build\windows\installer;try{if($AllowUntimestampedTestSignature){& $makeNsisCommand /WX -DFLUXDM_SIGN_INSTALLER -DFLUXDM_TEST_UNTIMESTAMPED "-DARG_WAILS_AMD64_BINARY=..\..\bin\FluxDM.exe" project.nsi}else{& $makeNsisCommand /WX -DFLUXDM_SIGN_INSTALLER "-DARG_WAILS_AMD64_BINARY=..\..\bin\FluxDM.exe" project.nsi};if($LASTEXITCODE){throw 'Signed NSIS rebuild failed'}}finally{Pop-Location}}
  $installer=Get-ChildItem build\bin\FluxDM-amd64-installer.exe -ErrorAction Stop
  $artifacts=@($app,$nativeHost,$installer.FullName);if($Sign){& "$PSScriptRoot\verify-release.ps1" -Path $artifacts -SignToolPath $SignToolPath}
  if($SevenZipPath){& "$PSScriptRoot\verify-installer-payload.ps1" -InstallerPath $installer.FullName -SevenZipPath $SevenZipPath -AppPath $app -NativeHostPath $nativeHost -ExtensionPath (Join-Path $root 'browser-extension') -Version $releaseVersion -RequireSignatures:$Sign}
  & "$PSScriptRoot\stage-release-assets.ps1" -Version $releaseVersion -InstallerPath $installer.FullName -OutputDirectory $ReleaseOutputDirectory -Signed:$Sign
  Write-Host "Release assets created in $ReleaseOutputDirectory"
}finally{Pop-Location}
