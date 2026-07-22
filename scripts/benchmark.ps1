param(
  [ValidateSet('baseline', 'stretch')]
  [string]$Profile = 'baseline',
  [ValidateRange(1, 20)]
  [int]$Runs = 3
)

$ErrorActionPreference = 'Stop'
$projectRoot = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$resultRoot = Join-Path $projectRoot 'benchmarks\results'
$composeProject = 'veltrix-crm-benchmark'
$appPort = if ($env:BENCHMARK_APP_PORT) { $env:BENCHMARK_APP_PORT } else { '18080' }
$postgresPort = if ($env:BENCHMARK_POSTGRES_PORT) { $env:BENCHMARK_POSTGRES_PORT } else { '55433' }
$composeBase = @('compose', '--project-name', $composeProject, '-f', (Join-Path $projectRoot 'compose.yaml'))
$previousEnvironment = @{
  DEMO_SEED = $env:DEMO_SEED
  APP_PORT = $env:APP_PORT
  POSTGRES_PORT = $env:POSTGRES_PORT
}

function Invoke-Compose {
  param([Parameter(ValueFromRemainingArguments = $true)][string[]]$Arguments)
  & docker @composeBase @Arguments
  if ($LASTEXITCODE -ne 0) {
    throw "docker compose failed with exit code $LASTEXITCODE"
  }
}

function Remove-BenchmarkEnvironment {
  & docker @composeBase --profile benchmark down --volumes --remove-orphans *> $null
}

function Wait-BenchmarkReadiness {
  for ($attempt = 1; $attempt -le 90; $attempt++) {
    try {
      Invoke-WebRequest -UseBasicParsing -TimeoutSec 2 `
        -Uri "http://127.0.0.1:$appPort/api/v1/health/ready" | Out-Null
      return
    }
    catch {
      Start-Sleep -Seconds 2
    }
  }
  & docker @composeBase logs --no-color app postgres *> (Join-Path $resultRoot 'compose-readiness-failure.log')
  throw 'benchmark application did not become ready'
}

New-Item -ItemType Directory -Force -Path $resultRoot | Out-Null
$env:DEMO_SEED = 'false'
$env:APP_PORT = $appPort
$env:POSTGRES_PORT = $postgresPort

try {
  docker info | Out-Null
  if ($LASTEXITCODE -ne 0) { throw 'Docker is unavailable' }
  $benchmarkFailed = $false

  for ($run = 1; $run -le $Runs; $run++) {
    # Only the dedicated benchmark project is reset; application development
    # volumes use a different Compose project and are never touched here.
    Remove-BenchmarkEnvironment
    Invoke-Compose up -d --build postgres app
    Wait-BenchmarkReadiness
    Invoke-Compose --profile benchmark run --rm benchmark-seed

    $computer = Get-CimInstance Win32_ComputerSystem
    $processor = Get-CimInstance Win32_Processor | Select-Object -First 1
    $commit = & git -C $projectRoot rev-parse HEAD 2>$null
    if (-not $commit) { $commit = 'uncommitted' }
    @(
      "date=$((Get-Date).ToUniversalTime().ToString('o'))"
      "commit=$commit"
      "os=$([System.Environment]::OSVersion.VersionString)"
      "cpu=$($processor.Name)"
      "ram_bytes=$($computer.TotalPhysicalMemory)"
      "docker=$(& docker version --format '{{.Server.Version}}')"
      "compose=$(& docker compose version --short)"
      'browser=not-applicable-k6'
      'dataset_profile=benchmark'
      'application_limit=0.5 CPU,128 MiB'
      'postgres_limit=0.5 CPU,384 MiB'
      'run=warm-after-1m-warmup-on-clean-dataset'
      'frontend_bundle_report=benchmarks/results/bundle-report.json'
    ) | Set-Content -Encoding UTF8 (Join-Path $resultRoot "benchmark-metadata-$Profile-run-$run.txt")

    $resultPath = "/benchmarks/results/k6-$Profile-run-$run.summary.json"
    $statsJob = Start-Job -ArgumentList $composeProject -ScriptBlock {
      param($Project)
      while ($true) {
        Write-Output "{`"sampledAt`":`"$([DateTime]::UtcNow.ToString('o'))`"}"
        & docker stats --no-stream --format '{{json .}}' `
          "${Project}-app-1" "${Project}-postgres-1" 2>&1
        Start-Sleep -Seconds 2
      }
    }
    try {
      Invoke-Compose --profile benchmark run --rm --no-deps `
        -e "K6_PROFILE=$Profile" -e "K6_RESULT_PATH=$resultPath" benchmark `
        run --out "json=/benchmarks/results/k6-$Profile-run-$run.json" `
        /benchmarks/k6/baseline.js
    }
    catch {
      $benchmarkFailed = $true
      Write-Warning "k6 run $run failed its command or thresholds; collecting the remaining runs"
    }
    finally {
      Stop-Job $statsJob -ErrorAction SilentlyContinue
      Receive-Job $statsJob -ErrorAction SilentlyContinue |
        Set-Content -Encoding UTF8 (Join-Path $resultRoot "docker-stats-$Profile-run-$run.jsonl")
      Remove-Job $statsJob -Force -ErrorAction SilentlyContinue
    }
    Invoke-Compose stats --no-stream app postgres |
      Set-Content -Encoding UTF8 (Join-Path $resultRoot "docker-stats-$Profile-run-$run.txt")
    & docker @composeBase logs --no-color app postgres *> (Join-Path $resultRoot "compose-$Profile-run-$run.log")
  }

  & node (Join-Path $projectRoot 'scripts\summarize-benchmarks.mjs') $Profile
  if ($LASTEXITCODE -ne 0) { throw 'benchmark summary generation failed' }
  Write-Host "Raw benchmark artifacts are in $resultRoot. Use docs/BENCHMARK_METHODOLOGY.md to report medians."
  if ($benchmarkFailed) { throw 'one or more k6 runs failed; inspect the retained summaries' }
}
finally {
  Remove-BenchmarkEnvironment
  foreach ($name in $previousEnvironment.Keys) {
    $value = $previousEnvironment[$name]
    if ($null -eq $value) { Remove-Item "Env:$name" -ErrorAction SilentlyContinue }
    else { Set-Item "Env:$name" $value }
  }
}
