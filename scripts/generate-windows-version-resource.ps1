[CmdletBinding()]
param(
  [string]$WailsConfig,
  [string]$OutputDirectory
)

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
if (-not $WailsConfig) { $WailsConfig = Join-Path $root 'wails.json' }
if (-not $OutputDirectory) { $OutputDirectory = Join-Path $root 'build\windows' }
$config = Get-Content -Raw -Encoding utf8 -LiteralPath $WailsConfig | ConvertFrom-Json
$info = $config.info
if (-not $info -or $info.productVersion -notmatch '^\d+\.\d+\.\d+$') { throw 'wails.json must contain a strict productVersion.' }

New-Item -ItemType Directory -Force -Path $OutputDirectory | Out-Null
$version = [string]$info.productVersion
$resource = [ordered]@{
  fixed = [ordered]@{ file_version = $version; product_version = $version }
  info = [ordered]@{
    '0409' = [ordered]@{
      FileVersion = $version; ProductVersion = $version; CompanyName = [string]$info.companyName
      FileDescription = [string]$info.productName; LegalCopyright = [string]$info.copyright
      ProductName = [string]$info.productName; Comments = [string]$info.comments
    }
  }
}
$utf8NoBom = New-Object System.Text.UTF8Encoding($false)
[IO.File]::WriteAllText((Join-Path $OutputDirectory 'generated-info.json'), ($resource | ConvertTo-Json -Depth 5), $utf8NoBom)

@"
<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<assembly manifestVersion="1.0" xmlns="urn:schemas-microsoft-com:asm.v1" xmlns:asmv3="urn:schemas-microsoft-com:asm.v3">
  <assemblyIdentity type="win32" name="com.fluxdm.$($config.name)" version="$version.0" processorArchitecture="*"/>
  <dependency><dependentAssembly><assemblyIdentity type="win32" name="Microsoft.Windows.Common-Controls" version="6.0.0.0" processorArchitecture="*" publicKeyToken="6595b64144ccf1df" language="*"/></dependentAssembly></dependency>
  <asmv3:application><asmv3:windowsSettings><dpiAware xmlns="http://schemas.microsoft.com/SMI/2005/WindowsSettings">true/pm</dpiAware><dpiAwareness xmlns="http://schemas.microsoft.com/SMI/2016/WindowsSettings">permonitorv2,permonitor</dpiAwareness></asmv3:windowsSettings></asmv3:application>
</assembly>
"@ | ForEach-Object { [IO.File]::WriteAllText((Join-Path $OutputDirectory 'generated-manifest.xml'), $_, $utf8NoBom) }
