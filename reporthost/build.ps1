# © Antony Monge López — Costa Rica — Céd. 604700548

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
$csc = Join-Path $env:WINDIR 'Microsoft.NET\Framework\v4.0.30319\csc.exe'
$outDir = Join-Path $root 'bin'
$out = Join-Path $outDir 'Aegis.ReportHost.exe'

if (-not (Test-Path -LiteralPath $csc)) {
    throw "No se encontró el compilador x86 de .NET Framework: $csc"
}
New-Item -ItemType Directory -Force -Path $outDir | Out-Null
& $csc /nologo /target:winexe /platform:x86 /optimize+ /out:$out `
    /reference:System.dll `
    /reference:System.Core.dll `
    /reference:System.Drawing.dll `
    /reference:System.Windows.Forms.dll `
    /reference:Microsoft.CSharp.dll `
    (Join-Path $PSScriptRoot 'ReportHost.cs')
if ($LASTEXITCODE -ne 0) { throw "Falló la compilación de Aegis.ReportHost.exe" }
Write-Output "ReportHost x86 compilado: $out"
