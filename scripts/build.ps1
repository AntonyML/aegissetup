[CmdletBinding()]
param(
    [switch]$Installer,
    [switch]$CleanOnly,
    [string]$Version = ""
)

& (Join-Path (Split-Path $PSScriptRoot -Parent) "build.ps1") @PSBoundParameters
