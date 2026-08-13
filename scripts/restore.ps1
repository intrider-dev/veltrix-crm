param(
  [Parameter(Mandatory = $true)][string]$BackupPath,
  [switch]$ConfirmDatabase
)

$ErrorActionPreference = 'Stop'
if (-not $ConfirmDatabase) {
  throw 'Restore replaces objects in the configured database. Re-run with -ConfirmDatabase.'
}
$resolvedBackup = (Resolve-Path -LiteralPath $BackupPath).Path
$projectRoot = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$composeFile = Join-Path $projectRoot 'compose.yaml'
$postgresUser = if ($env:POSTGRES_USER) { $env:POSTGRES_USER } else { 'postgres' }
$postgresDatabase = if ($env:POSTGRES_DB) { $env:POSTGRES_DB } else { 'veltrix_crm' }
$containerFile = "/tmp/veltrix-crm-restore-$([guid]::NewGuid().ToString('N')).dump"

try {
  docker compose -f $composeFile cp $resolvedBackup "postgres:$containerFile"
  if ($LASTEXITCODE -ne 0) { throw 'docker compose cp failed' }
  docker compose -f $composeFile exec -T postgres pg_restore -U $postgresUser -d $postgresDatabase `
    --clean --if-exists --no-owner --no-privileges --exit-on-error $containerFile
  if ($LASTEXITCODE -ne 0) { throw 'pg_restore failed' }
  Write-Host "Restore completed for database $postgresDatabase"
} finally {
  docker compose -f $composeFile exec -T postgres rm -f $containerFile 2>$null | Out-Null
}
