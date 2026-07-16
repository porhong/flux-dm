[CmdletBinding()]
param([Parameter(Mandatory)][string[]]$Path,[string]$SignToolPath='signtool.exe')
$ErrorActionPreference='Stop'
foreach($item in $Path){$resolved=(Resolve-Path -LiteralPath $item).Path;$signature=Get-AuthenticodeSignature -LiteralPath $resolved;if($signature.Status -ne 'Valid'){throw "Invalid Authenticode signature on ${resolved}: $($signature.Status)"};& $SignToolPath verify /pa /all $resolved;if($LASTEXITCODE -ne 0){throw "WinVerifyTrust verification failed: $resolved"};$hash=(Get-FileHash -Algorithm SHA256 -LiteralPath $resolved).Hash;Write-Host "$hash  $resolved"}
