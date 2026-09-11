# © Antony Monge López — Costa Rica — Céd. 604700548
# LEGACY: reemplazado por `aegis setup-db`. Se conserva como referencia.
# Carga .env a $env: para migrate.ps1
Get-Content -LiteralPath "$PSScriptRoot\.env" -ErrorAction SilentlyContinue | ForEach-Object {
  if ($_ -match '^\s*#' -or $_ -match '^\s*$') { return }
  $k,$v = $_ -split '=', 2
  if ($k) { [Environment]::SetEnvironmentVariable($k.Trim(), $v.Trim(), "Process"); Set-Item -Path "env:$($k.Trim())" -Value $v.Trim() }
}
