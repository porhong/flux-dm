[CmdletBinding()]
param(
  [Parameter(Mandatory)]
  [string[]]$Path,

  [string]$Version = '1.0.0',

  [string]$ProductName = 'FluxDM',

  [string]$CompanyName = 'FluxDM'
)

$ErrorActionPreference = 'Stop'

foreach ($item in $Path) {
  $resolved = (Resolve-Path -LiteralPath $item).Path
  $metadata = [Diagnostics.FileVersionInfo]::GetVersionInfo($resolved)

  if ($metadata.FileVersion -ne $Version) {
    throw "Unexpected FileVersion on ${resolved}: '$($metadata.FileVersion)'"
  }
  if ($metadata.ProductVersion -ne $Version) {
    throw "Unexpected ProductVersion on ${resolved}: '$($metadata.ProductVersion)'"
  }
  if ($metadata.ProductName -ne $ProductName) {
    throw "Unexpected ProductName on ${resolved}: '$($metadata.ProductName)'"
  }
  if ($metadata.CompanyName -ne $CompanyName) {
    throw "Unexpected CompanyName on ${resolved}: '$($metadata.CompanyName)'"
  }

  Write-Host "Version metadata verified: $resolved ($Version)"
}
