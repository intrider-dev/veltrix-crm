[CmdletBinding()]
param(
  [string]$BaseUrl = '',
  [ValidateRange(1, 100)]
  [int]$Cycles = 20,
  [ValidateSet('desktop', 'mobile', 'all')]
  [string]$LighthouseProfile = 'all',
  [string]$LighthousePath = '/login',
  [string]$ChromePath = '',
  [switch]$Headed,
  [switch]$SkipMeasurement,
  [switch]$SkipLighthouse,
  [switch]$DryRun,
  [switch]$Help
)

$ErrorActionPreference = 'Stop'
$projectRoot = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$node = (Get-Command node.exe -ErrorAction Stop).Source

if ($Help) {
  @'
Usage:
  ./scripts/benchmark-browser.ps1 [-BaseUrl URL] [-Cycles 20]
    [-LighthouseProfile desktop|mobile|all] [-LighthousePath /login]
    [-ChromePath PATH] [-Headed] [-SkipMeasurement] [-SkipLighthouse]
    [-DryRun]

Credentials are read only from E2E_EMAIL and E2E_PASSWORD. Artifacts are written
under benchmarks/results and are ignored by Git. -ChromePath overrides local
Chrome/Chromium for Lighthouse without changing the machine environment.
'@ | Write-Host
  exit 0
}

if (-not $BaseUrl) {
  if ($env:BROWSER_BENCHMARK_BASE_URL) { $BaseUrl = $env:BROWSER_BENCHMARK_BASE_URL }
  elseif ($env:E2E_BASE_URL) { $BaseUrl = $env:E2E_BASE_URL }
  else { $BaseUrl = 'http://127.0.0.1:8080' }
}

Push-Location $projectRoot
try {
  $measurementArguments = @(
    'benchmarks/browser/measure-performance.mjs',
    '--base-url', $BaseUrl,
    '--cycles', $Cycles.ToString()
  )
  if ($Headed) { $measurementArguments += '--headed' }

  $lighthouseArguments = @(
    'scripts/run-lighthouse.mjs',
    '--base-url', $BaseUrl,
    '--path', $LighthousePath,
    '--profile', $LighthouseProfile
  )
  if ($ChromePath) { $lighthouseArguments += @('--chrome-path', $ChromePath) }

  if ($DryRun) {
    if (-not $SkipMeasurement) {
      Write-Host "node $($measurementArguments -join ' ')"
    }
    if (-not $SkipLighthouse) {
      & $node @lighthouseArguments --dry-run
      if ($LASTEXITCODE -ne 0) { throw 'Lighthouse dry-run validation failed' }
    }
    exit 0
  }

  if (-not $SkipMeasurement) {
    & $node @measurementArguments
    if ($LASTEXITCODE -ne 0) { throw 'browser performance measurement failed' }
  }
  if (-not $SkipLighthouse) {
    & $node @lighthouseArguments
    if ($LASTEXITCODE -ne 0) { throw 'Lighthouse measurement failed' }
  }
}
finally {
  Pop-Location
}
