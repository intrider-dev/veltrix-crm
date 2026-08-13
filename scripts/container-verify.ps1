param(
    [string]$Image = "crm-app:local"
)

$ErrorActionPreference = "Stop"

$runtimeUser = docker image inspect --format '{{.Config.User}}' $Image
if ($LASTEXITCODE -ne 0) {
    throw "Unable to inspect image $Image"
}
if ($runtimeUser.Trim() -ne "65532:65532") {
    throw "Unexpected runtime user: $runtimeUser"
}

$previousErrorAction = $ErrorActionPreference
try {
    # A scratch image is expected to fail this probe. Temporarily suppress the
    # native stderr record and make the assertion from the process exit code.
    $ErrorActionPreference = "SilentlyContinue"
    docker run --rm --entrypoint /usr/local/bin/node $Image --version *> $null
    $nodeExitCode = $LASTEXITCODE
}
finally {
    $ErrorActionPreference = $previousErrorAction
}
if ($nodeExitCode -eq 0) {
    throw "Node.js was found in the runtime image"
}

$healthcheck = docker image inspect --format '{{json .Config.Healthcheck.Test}}' $Image
if ($LASTEXITCODE -ne 0) {
    throw "Unable to inspect image healthcheck"
}

Write-Output $healthcheck
Write-Output "Runtime user and Node.js absence checks passed."
