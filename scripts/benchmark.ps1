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
  # Windows PowerShell 5 converts ordinary Docker progress on stderr into
  # ErrorRecord objects when the caller uses Stop. Cleanup is best-effort and
  # scoped to the dedicated project, so evaluate Docker's exit code instead.
  $previousPreference = $ErrorActionPreference
  $ErrorActionPreference = 'Continue'
  & docker @composeBase --profile benchmark down --volumes --remove-orphans 2>$null | Out-Null
  $exitCode = $LASTEXITCODE
  $ErrorActionPreference = $previousPreference
  if ($exitCode -ne 0) {
    Write-Warning "benchmark cleanup exited with code $exitCode"
  }
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
    # Use an explicit argument array: PowerShell otherwise interprets `-d` as
    # the common `-Debug` parameter of this wrapper instead of Compose detach.
    Invoke-Compose -Arguments @('up', '--detach', '--build', 'postgres', 'app')
    Wait-BenchmarkReadiness
    Invoke-Compose -Arguments @('--profile', 'benchmark', 'run', '--rm', 'benchmark-seed')

    $benchmarkVUs = if ($env:CRM_BENCHMARK_VUS) { $env:CRM_BENCHMARK_VUS } else { '50' }
    $benchmarkWarmup = if ($env:CRM_BENCHMARK_WARMUP) { $env:CRM_BENCHMARK_WARMUP } else { '1m' }
    $benchmarkDuration = if ($env:CRM_BENCHMARK_DURATION) { $env:CRM_BENCHMARK_DURATION } else { '5m' }
    $benchmarkThinkTime = if ($env:CRM_BENCHMARK_THINK_TIME) { $env:CRM_BENCHMARK_THINK_TIME } else { '0.1' }
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
      "virtual_users=$benchmarkVUs"
      "run=warm-after-$benchmarkWarmup-warmup-and-$benchmarkDuration-measured-on-clean-dataset"
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
      Invoke-Compose -Arguments @(
        '--profile', 'benchmark', 'run', '--rm', '--no-deps',
        '--env', "K6_PROFILE=$Profile", '--env', "K6_RESULT_PATH=$resultPath",
        '--env', "CRM_BENCHMARK_VUS=$benchmarkVUs",
        '--env', "CRM_BENCHMARK_WARMUP=$benchmarkWarmup",
        '--env', "CRM_BENCHMARK_DURATION=$benchmarkDuration",
        '--env', "CRM_BENCHMARK_THINK_TIME=$benchmarkThinkTime",
        'benchmark', 'run', '--out', "json=/benchmarks/results/k6-$Profile-run-$run.json",
        '/benchmarks/k6/baseline.js'
      )
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
    # Compose v5 accepts at most one optional service for `stats`; this
    # benchmark project contains only the measured app/postgres at this point,
    # so collect both by omitting a service filter.
    Invoke-Compose -Arguments @('stats', '--no-stream') |
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
