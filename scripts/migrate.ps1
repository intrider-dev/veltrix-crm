param()

$ErrorActionPreference = 'Stop'
$projectRoot = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$composeFile = Join-Path $projectRoot 'compose.yaml'

docker compose -f $composeFile up -d --no-recreate --wait --wait-timeout 120 postgres
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
docker compose -f $composeFile build postgres
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
docker compose -f $composeFile run --rm --no-deps --user postgres `
  --entrypoint /app/veltrix-crm postgres migrate
exit $LASTEXITCODE
