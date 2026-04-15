[CmdletBinding()]
param(
    [ValidateSet("up", "down")]
    [string]$Direction = "up"
)

$ErrorActionPreference = "Stop"

$Root = Split-Path -Parent $PSScriptRoot
$EnvFile = Join-Path $Root ".env"
if (-not (Test-Path $EnvFile)) {
    $EnvFile = Join-Path $Root ".env.example"
}

Get-Content $EnvFile | ForEach-Object {
    if ($_ -match '^\s*#' -or $_ -notmatch '=') {
        return
    }

    $name, $value = $_ -split '=', 2
    Set-Item -Path "Env:$name" -Value $value
}

$network = "$($env:COMPOSE_PROJECT_NAME)_default"
if ([string]::IsNullOrWhiteSpace($env:COMPOSE_PROJECT_NAME)) {
    $network = "wiki-editor_default"
}

function Invoke-Migration {
    param(
        [string]$ServiceName,
        [string]$RelativePath,
        [string]$DatabaseUrl
    )

    $fullPath = Join-Path $Root $RelativePath
    if (-not (Test-Path $fullPath)) {
        Write-Host "Skipping ${ServiceName}: missing ${RelativePath}"
        return
    }

    $sqlFiles = Get-ChildItem -Path $fullPath -Filter *.sql -File -ErrorAction SilentlyContinue
    if (-not $sqlFiles) {
        Write-Host "Skipping ${ServiceName}: no SQL migrations in ${RelativePath}"
        return
    }

    $args = @(
        "run", "--rm",
        "--network", $network,
        "-v", "${Root}:/workspace",
        "migrate/migrate:v4.18.2",
        "-path=/workspace/$RelativePath",
        "-database", $DatabaseUrl
    )

    if ($Direction -eq "down") {
        $args += @("down", "1")
    } else {
        $args += "up"
    }

    & docker @args
}

Invoke-Migration -ServiceName "auth-service" -RelativePath "services/auth-service/migrations" -DatabaseUrl $env:AUTH_DATABASE_URL
Invoke-Migration -ServiceName "page-service" -RelativePath "services/page-service/migrations" -DatabaseUrl $env:PAGE_DATABASE_URL
Invoke-Migration -ServiceName "knowledge-graph-search-service" -RelativePath "services/knowledge-graph-search-service/migrations" -DatabaseUrl $env:SEARCH_DATABASE_URL
Invoke-Migration -ServiceName "file-service" -RelativePath "services/file-service/migrations" -DatabaseUrl $env:FILE_DATABASE_URL
