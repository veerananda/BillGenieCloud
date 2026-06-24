# Load secrets from fly-secrets.env and push to Fly.io
# Usage: .\scripts\set-fly-secrets.ps1 [-AppName billgenie-api]

param(
    [string]$AppName = "billgenie-api",
    [string]$SecretsFile = "$PSScriptRoot\fly-secrets.env"
)

$ErrorActionPreference = "Stop"

function Get-FlyCommand {
    if (Get-Command fly -ErrorAction SilentlyContinue) { return "fly" }
    if (Get-Command flyctl -ErrorAction SilentlyContinue) { return "flyctl" }
    $flyctl = Join-Path $env:USERPROFILE ".fly\bin\flyctl.exe"
    if (Test-Path $flyctl) { return $flyctl }
    return $null
}

$fly = Get-FlyCommand
if (-not $fly) {
    Write-Error "Fly CLI not found. Install: iwr https://fly.io/install.ps1 -useb | iex"
}

if (-not (Test-Path $SecretsFile)) {
    Write-Host "Create $SecretsFile from scripts\fly-secrets.example.env" -ForegroundColor Yellow
    exit 1
}

# Build import file (KEY=VALUE per line). fly secrets import handles = in values correctly.
$lines = New-Object System.Collections.Generic.List[string]
Get-Content $SecretsFile | ForEach-Object {
    $line = $_.Trim()
    if ($line -eq "" -or $line.StartsWith("#")) { return }
    $idx = $line.IndexOf("=")
    if ($idx -lt 1) { return }
    $key = $line.Substring(0, $idx).Trim()
    $val = $line.Substring($idx + 1).Trim()
    if ($val -ne "" -and -not $val.StartsWith("REPLACE_") -and -not $val.Contains("YOUR_")) {
        $lines.Add("$key=$val")
    }
}

if ($lines.Count -eq 0) {
    Write-Error "No valid secrets found in $SecretsFile"
}

$importFile = Join-Path $env:TEMP "fly-secrets-import-$AppName.env"
$lines | Set-Content -Path $importFile -Encoding UTF8

Write-Host "Importing $($lines.Count) secrets to $AppName (via fly secrets import)..." -ForegroundColor Cyan
Get-Content $importFile | & $fly secrets import --app $AppName
Remove-Item $importFile -Force -ErrorAction SilentlyContinue
Write-Host "Done." -ForegroundColor Green
