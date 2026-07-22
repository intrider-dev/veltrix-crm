param([string]$OutputPath = '')

$ErrorActionPreference = 'Stop'
$projectRoot = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$composeFile = Join-Path $projectRoot 'compose.yaml'
if ([string]::IsNullOrWhiteSpace($OutputPath)) {
  $stamp = (Get-Date).ToUniversalTime().ToString('yyyyMMddTHHmmssZ')
  $OutputPath = Join-Path $projectRoot "backups\veltrix-crm-$stamp.dump"
}
$outputParent = Split-Path -Parent $OutputPath
if ([string]::IsNullOrWhiteSpace($outputParent)) { $outputParent = $projectRoot }
New-Item -ItemType Directory -Force -Path $outputParent | Out-Null
$postgresUser = if ($env:POSTGRES_USER) { $env:POSTGRES_USER } else { 'postgres' }
$postgresDatabase = if ($env:POSTGRES_DB) { $env:POSTGRES_DB } else { 'veltrix_crm' }
$containerFile = "/tmp/veltrix-crm-backup-$([guid]::NewGuid().ToString('N')).dump"

try {
  docker compose -f $composeFile exec -T postgres pg_dump -U $postgresUser -d $postgresDatabase `
    --format=custom --no-owner --no-privileges --file=$containerFile
  if ($LASTEXITCODE -ne 0) { throw 'pg_dump failed' }
  docker compose -f $composeFile cp "postgres:$containerFile" $OutputPath
  if ($LASTEXITCODE -ne 0) { throw 'docker compose cp failed' }
  Write-Host "Backup written to $OutputPath"
} finally {
  docker compose -f $composeFile exec -T postgres rm -f $containerFile 2>$null | Out-Null
}
