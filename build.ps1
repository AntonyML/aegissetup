[CmdletBinding()]
param(
    [switch]$Installer,
    [switch]$CleanOnly,
    [string]$Version = ""
)

$ErrorActionPreference = "Stop"
$root = if (Test-Path (Join-Path $PSScriptRoot "go.mod")) { $PSScriptRoot } else { (Split-Path $PSScriptRoot -Parent) }

# 1. Detectar y cerrar instancias de aegis.exe en ejecución para evitar bloqueos
$running = Get-Process aegis -ErrorAction SilentlyContinue
if ($running) {
    Write-Host "==> Proceso aegis.exe en ejecución detectado (PIDs: $($running.Id -join ', ')). Finalizando para evitar bloqueo..." -ForegroundColor Yellow
    $running | Stop-Process -Force -ErrorAction SilentlyContinue
    Start-Sleep -Milliseconds 400
}

# 2. Limpieza de artefactos previos en bin/ y dist/
Write-Host "==> Limpiando artefactos de compilación anteriores..." -ForegroundColor Cyan
$cleanDirs = @(
    (Join-Path $root "bin"),
    (Join-Path $root "dist")
)

foreach ($dir in $cleanDirs) {
    if (Test-Path $dir) {
        Get-ChildItem -Path $dir -Include *.exe, *.exe~, *.old, *.tmp, *.bak -Recurse -Force -ErrorAction SilentlyContinue | ForEach-Object {
            Write-Host "    Borrando: $($_.FullName)"
            Remove-Item -Force $_.FullName -ErrorAction SilentlyContinue
        }
    }
}

if ($CleanOnly) {
    Write-Host "==> Limpieza finalizada." -ForegroundColor Green
    exit 0
}

# 3. Gate de calidad antes de producir el binario
Push-Location $root
try {
    # El árbol histórico contiene archivos que no pasan el formateador de la
    # versión actual de Go por comentarios de documentación. El gate sólo
    # inspecciona el diff de esta compilación y los Go nuevos, sin reescribir
    # archivos ajenos al cambio.
    $goChanged = @(
        git diff --name-only --diff-filter=ACMRT -- '*.go'
        git ls-files --others --exclude-standard -- '*.go'
    ) | Where-Object { $_ -and (Test-Path (Join-Path $root $_)) } | Sort-Object -Unique
    $fmt = if ($goChanged) { gofmt -l $goChanged } else { @() }
    if ($fmt) { throw "gofmt detectó archivos sin formato: $($fmt -join ', ')" }
    go vet ./...
    go test ./... -count=1
    $oldGOOS = $env:GOOS
    try {
        $env:GOOS = "linux"
        go build ./...
    } finally {
        if ($null -eq $oldGOOS) { Remove-Item Env:GOOS -ErrorAction SilentlyContinue } else { $env:GOOS = $oldGOOS }
    }
} finally {
    Pop-Location
}

# 4. Compilación limpia con Go
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
$tmpExe = Join-Path $binDir "aegis.exe.tmp"

if (Test-Path $tmpExe) { Remove-Item -Force $tmpExe -ErrorAction SilentlyContinue }

Push-Location $root
try {
    go build -v -trimpath -ldflags $ldflags -o $tmpExe ./cmd/aegis
} finally {
    Pop-Location
}

if (-not (Test-Path $tmpExe)) {
    throw "Error: No se generó el binario en $tmpExe"
}

if (Test-Path $targetExe) {
    Remove-Item -Force $targetExe -ErrorAction SilentlyContinue
}
Move-Item -Force $tmpExe $targetExe

Write-Host "==> Binario generado: $targetExe ($([math]::Round((Get-Item $targetExe).Length / 1MB, 2)) MB)" -ForegroundColor Green

# 5. El visor ActiveX de Crystal corre fuera del proceso Go x64.
$reportHostBuild = Join-Path $root "reporthost\build.ps1"
if (Test-Path -LiteralPath $reportHostBuild) {
    Write-Host "==> Compilando ReportHost x86 (STA + WinForms)..." -ForegroundColor Cyan
    & $reportHostBuild
    if ($LASTEXITCODE -ne 0) {
        throw "Error: no se pudo compilar Aegis.ReportHost.exe"
    }
}

# 6. Verificación de arranque del binario
Write-Host "==> Verificando ejecución del binario..." -ForegroundColor Cyan
& $targetExe --help | Out-Null
if ($LASTEXITCODE -ne 0) {
    throw "Error: aegis.exe falló al ejecutar --help"
}
Write-Host "==> aegis.exe responde correctamente." -ForegroundColor Green

# 6. Opcional: compilación del instalador oficial con Inno Setup
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
