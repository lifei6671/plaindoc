param(
    [Parameter(Mandatory = $true)]
    [string]$RootDir,

    [Parameter(Mandatory = $true)]
    [string]$ServerDir,

    [Parameter(Mandatory = $true)]
    [string]$EnvFile,

    [Parameter(Mandatory = $true)]
    [string]$AppAddr,

    [Parameter(Mandatory = $true)]
    [string]$WebOrigin,

    [Parameter(Mandatory = $true)]
    [ValidateSet("true", "false")]
    [string]$SSREnabled,

    [string]$SSRWorkerExec = "node",
    [string]$SSRWorkerEntry = ""
)

$ErrorActionPreference = "Stop"

$envPath = Join-Path $RootDir $EnvFile
if (Test-Path -LiteralPath $envPath) {
    Write-Host "load env from $EnvFile"
    Get-Content -LiteralPath $envPath | ForEach-Object {
        $line = $_.Trim()
        if ($line -ne "" -and -not $line.StartsWith("#") -and $line.Contains("=")) {
            $parts = $line.Split("=", 2)
            $name = $parts[0].Trim()
            $value = $parts[1].Trim().Trim('"').Trim("'")
            if ($name -ne "") {
                Set-Item -Path "Env:$name" -Value $value
            }
        }
    }
}

$env:APP_ENV = "development"
$env:APP_ADDR = $AppAddr
$env:WEB_ORIGIN = $WebOrigin
$env:SSR_WORKER_ENABLED = $SSREnabled

if ($SSREnabled -eq "true") {
    $env:SSR_WORKER_EXEC = $SSRWorkerExec
    $env:SSR_WORKER_ENTRY = $SSRWorkerEntry
}

Set-Location -LiteralPath $ServerDir
go run ./cmd/server
exit $LASTEXITCODE
