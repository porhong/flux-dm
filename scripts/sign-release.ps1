[CmdletBinding()]
param(
  [Parameter(Mandatory)][string[]]$Path,
  [Parameter(Mandatory)][string]$CertificateThumbprint,
  [string]$SignToolPath = 'signtool.exe',
  [string]$TimestampUrl = 'http://timestamp.digicert.com',
  [switch]$AllowUntimestampedTestSignature
)
$ErrorActionPreference = 'Stop'
$thumbprint = ($CertificateThumbprint -replace '\s','').ToUpperInvariant()
if ($thumbprint -notmatch '^[0-9A-F]{40,64}$') { throw 'Certificate thumbprint must be 40-64 hexadecimal characters.' }
if ($AllowUntimestampedTestSignature -and $TimestampUrl) { throw '-AllowUntimestampedTestSignature requires an empty -TimestampUrl.' }
if (-not $TimestampUrl -and -not $AllowUntimestampedTestSignature) { throw 'Production signatures require an RFC 3161 timestamp URL.' }
foreach ($item in $Path) {
  $resolved = (Resolve-Path -LiteralPath $item).Path
  $signArguments = @('sign', '/sha1', $thumbprint, '/fd', 'SHA256')
  if ($TimestampUrl) { $signArguments += @('/tr', $TimestampUrl, '/td', 'SHA256') }
  $signArguments += $resolved
  & $SignToolPath @signArguments
  if ($LASTEXITCODE -ne 0) { throw "Signing failed: $resolved" }
  & $SignToolPath verify /pa /all $resolved
  if ($LASTEXITCODE -ne 0) { throw "Signature verification failed: $resolved" }
}
