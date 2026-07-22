param()

$ErrorActionPreference = 'Stop'
$projectRoot = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$composeFile = Join-Path $projectRoot 'compose.yaml'

docker compose -f $composeFile up -d postgres
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
docker compose -f $composeFile run --rm --no-deps app migrate
exit $LASTEXITCODE
