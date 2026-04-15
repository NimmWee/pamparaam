[CmdletBinding()]
param()

$ErrorActionPreference = "Stop"

$Root = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$EnvFile = Join-Path $Root ".env"
if (-not (Test-Path $EnvFile)) {
    Copy-Item (Join-Path $Root ".env.example") $EnvFile
    Write-Host "Created $EnvFile from .env.example"
}

$composeArgs = @("--env-file", $EnvFile, "-f", (Join-Path $Root "deploy/docker-compose.yml"))
$network = if ($env:COMPOSE_PROJECT_NAME) { "$($env:COMPOSE_PROJECT_NAME)_default" } else { "wiki-editor_default" }

function Invoke-Compose {
    param([Parameter(ValueFromRemainingArguments = $true)] [string[]]$Args)
    & docker compose @composeArgs @Args
    if ($LASTEXITCODE -ne 0) {
        throw "docker compose $($Args -join ' ') failed with exit code $LASTEXITCODE"
    }
}

function Show-FailureContext {
    Write-Host "Compose smoke test failed. Recent service state:"
    try { Invoke-Compose ps } catch {}
    try { Invoke-Compose logs --tail=100 } catch {}
}

function Wait-Health {
    param(
        [string]$Url,
        [string]$Name
    )

    for ($attempt = 0; $attempt -lt 60; $attempt++) {
        try {
            $response = Invoke-WebRequest -Uri $Url -UseBasicParsing -TimeoutSec 5
            if ($response.StatusCode -ge 200 -and $response.StatusCode -lt 300) {
                return
            }
        } catch {}
        Start-Sleep -Seconds 2
    }

    throw "Timed out waiting for $Name at $Url"
}

function Assert-ServiceEnv {
    param(
        [string]$Service,
        [string]$Variable
    )

    $result = & docker compose @composeArgs exec -T $Service sh -lc "printenv $Variable >/dev/null"
    if ($LASTEXITCODE -ne 0) {
        throw "Missing $Variable in $Service"
    }
    $null = $result
}

try {
    Write-Host "Starting demo runtime"
    Invoke-Compose up -d --build

    Write-Host "Running migrations"
    & (Join-Path $Root "scripts/migrate.ps1") -Direction up

    Write-Host "Seeding auth demo data"
    Invoke-Compose exec -T auth-service /app/seed-demo

    Wait-Health -Url "http://localhost:8081/health/ready" -Name "auth-service"
    Wait-Health -Url "http://localhost:8082/health/ready" -Name "page-service"
    Wait-Health -Url "http://localhost:8083/health/ready" -Name "collaboration-service"
    Wait-Health -Url "http://localhost:8084/health/ready" -Name "search-service"
    Wait-Health -Url "http://localhost:8085/health/ready" -Name "mws-integration-service"
    Wait-Health -Url "http://localhost:8086/health/ready" -Name "file-service"
    Wait-Health -Url "http://localhost:8080/health/ready" -Name "gateway"

    Assert-ServiceEnv -Service "gateway" -Variable "AUTH_SERVICE_GRPC_ADDR"
    Assert-ServiceEnv -Service "page-service" -Variable "AUTH_SERVICE_GRPC_ADDR"
    Assert-ServiceEnv -Service "page-service" -Variable "MWS_INTEGRATION_SERVICE_GRPC_ADDR"
    Assert-ServiceEnv -Service "page-service" -Variable "FILE_SERVICE_GRPC_ADDR"
    Assert-ServiceEnv -Service "page-service" -Variable "PAGE_REDIS_ADDR"
    Assert-ServiceEnv -Service "page-service" -Variable "PAGE_NATS_URL"
    Assert-ServiceEnv -Service "collaboration-service" -Variable "PAGE_SERVICE_GRPC_ADDR"
    Assert-ServiceEnv -Service "collaboration-service" -Variable "AUTH_SERVICE_GRPC_ADDR"
    Assert-ServiceEnv -Service "collaboration-service" -Variable "COLLABORATION_REDIS_ADDR"
    Assert-ServiceEnv -Service "collaboration-service" -Variable "COLLABORATION_NATS_URL"
    Assert-ServiceEnv -Service "knowledge-graph-search-service" -Variable "AUTH_SERVICE_GRPC_ADDR"
    Assert-ServiceEnv -Service "knowledge-graph-search-service" -Variable "SEARCH_NATS_URL"
    Assert-ServiceEnv -Service "file-service" -Variable "FILE_MINIO_ENDPOINT"
    Assert-ServiceEnv -Service "file-service" -Variable "FILE_MINIO_PUBLIC_BASE_URL"
    Assert-ServiceEnv -Service "file-service" -Variable "FILE_MINIO_BUCKET"

    Write-Host "Running end-to-end compose smoke probe"
    & docker run --rm `
        --network $network `
        -e "GATEWAY_BASE_URL=http://gateway:8080/api/v1" `
        -e "SMOKE_MINIO_INTERNAL_BASE_URL=http://minio:9000" `
        -v "${Root}:/workspace" `
        -w /workspace `
        golang:1.23 `
        sh -lc "export PATH=/usr/local/go/bin:`$PATH && go run ./tests/compose/demo_smoke_probe.go"
    if ($LASTEXITCODE -ne 0) {
        throw "docker run smoke probe failed with exit code $LASTEXITCODE"
    }

    Write-Host "Compose smoke test passed"
} catch {
    Show-FailureContext
    throw
}
