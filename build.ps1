[CmdletBinding()]
param(
    [switch]$Installer,
    [switch]$CleanOnly,
    [string]$Version = ""
)

$ErrorActionPreference = "Stop"
$root = if (Test-Path (Join-Path $PSScriptRoot "go.mod")) { $PSScriptRoot } else { (Split-Path $PSScriptRoot -Parent) }

# 1. Clean previous build artifacts in bin and dist
Write-Host "==> Limpiando artefactos de compilación anteriores..." -ForegroundColor Cyan
$cleanDirs = @(
    (Join-Path $root "bin"),
    (Join-Path $root "dist")
)

foreach ($dir in $cleanDirs) {
    if (Test-Path $dir) {
        Get-ChildItem -Path $dir -Include *.exe, *.exe~, *.old, *.tmp, *.bak -Recurse -Force -ErrorAction SilentlyContinue | ForEach-Object {
            Write-Host "    Borrando: $($_.FullName)"
            Remove-Item -Force $_.FullName
        }
    }
}

if ($CleanOnly) {
    Write-Host "==> Limpieza finalizada." -ForegroundColor Green
    exit 0
}

# 2. Build Go binary
Write-Host "==> Compilando binario aegis.exe..." -ForegroundColor Cyan
$binDir = Join-Path $root "bin"
if (-not (Test-Path $binDir)) {
    New-Item -ItemType Directory -Force -Path $binDir | Out-Null
}

$ldflags = "-s -w"
if ($Version) {
    $ldflags += " -X aegis-setup/internal/version.Current=$Version"
}

$targetExe = Join-Path $binDir "aegis.exe"
Push-Location $root
try {
    go build -v -trimpath -ldflags $ldflags -o $targetExe ./cmd/aegis
} finally {
    Pop-Location
}

if (-not (Test-Path $targetExe)) {
    throw "Error: No se generó el binario en $targetExe"
}

Write-Host "==> Binario generado: $targetExe ($([math]::Round((Get-Item $targetExe).Length / 1MB, 2)) MB)" -ForegroundColor Green

# 3. Test running the binary
Write-Host "==> Verificando ejecución del binario..." -ForegroundColor Cyan
& $targetExe --help | Out-Null
if ($LASTEXITCODE -ne 0) {
    throw "Error: aegis.exe falló al ejecutar --help"
}
Write-Host "==> aegis.exe responde correctamente." -ForegroundColor Green

# 4. Optional: build installer with Inno Setup
if ($Installer) {
    Write-Host "==> Compilando instalador Inno Setup..." -ForegroundColor Cyan
    $iscc = if (Test-Path "C:\Program Files (x86)\Inno Setup 6\ISCC.exe") {
        "C:\Program Files (x86)\Inno Setup 6\ISCC.exe"
    } elseif (Get-Command iscc.exe -ErrorAction SilentlyContinue) {
        "iscc.exe"
    } else {
        $null
    }

    if ($iscc) {
        $issArgs = @("/O+")
        if ($Version) {
            $issArgs += "/DMyAppVersion=$Version"
        }
        $issArgs += (Join-Path $root "installer.iss")
        & $iscc $issArgs
        Write-Host "==> Instalador generado en dist/" -ForegroundColor Green
    } else {
        Write-Warning "ISCC.exe no encontrado; omitiendo empaquetado del instalador."
    }
}

Write-Host "==> Compilación finalizada exitosamente." -ForegroundColor Green
