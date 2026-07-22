param()

$ErrorActionPreference = 'Stop'
$projectRoot = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$environmentFile = Join-Path $projectRoot '.env'
if (-not (Test-Path -LiteralPath $environmentFile)) {
  Copy-Item -LiteralPath (Join-Path $projectRoot '.env.example') -Destination $environmentFile
  Write-Host 'Created .env from .env.example'
}

docker compose -f (Join-Path $projectRoot 'compose.yaml') up --build
exit $LASTEXITCODE
