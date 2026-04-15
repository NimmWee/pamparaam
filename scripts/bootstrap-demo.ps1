[CmdletBinding()]
param()

$ErrorActionPreference = "Stop"

$Root = Split-Path -Parent $PSScriptRoot
$EnvFile = Join-Path $Root ".env"

if (-not (Test-Path $EnvFile)) {
    Copy-Item (Join-Path $Root ".env.example") $EnvFile
    Write-Host "Created $EnvFile from .env.example"
}

Write-Host "Syncing Go workspace"
Push-Location $Root
try {
    go work sync

    Write-Host "Starting Docker Compose stack"
    docker compose --env-file .env -f deploy/docker-compose.yml up -d --build
} finally {
    Pop-Location
}

Write-Host "Running available migrations"
& (Join-Path $Root "scripts/migrate.ps1") -Direction up

Get-Content $EnvFile | ForEach-Object {
    if ($_ -match '^\s*#' -or $_ -notmatch '=') {
        return
    }

    $name, $value = $_ -split '=', 2
    Set-Item -Path "Env:$name" -Value $value
}

Write-Host "Seeding auth demo data"
Push-Location (Join-Path $Root "services/auth-service")
try {
    go run ./cmd/seed-demo
} finally {
    Pop-Location
}

Write-Host ""
Write-Host "Bootstrap complete."
Write-Host "Gateway:    http://localhost:8080"
Write-Host "Prometheus: http://localhost:9090"
Write-Host "MinIO API:  http://localhost:9000"
Write-Host "MinIO UI:   http://localhost:9001"
Write-Host "MWS Mock:   http://localhost:8090"
