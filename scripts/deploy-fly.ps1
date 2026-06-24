# Deploy restaurant-api to Fly.io (Mumbai / bom)
# Prerequisites: fly auth login, DO Postgres + Upstash Redis, fly-secrets.env configured

param(
    [string]$AppName = "billgenie-api",
    [switch]$SkipSecrets,
    [switch]$Launch
)

$ErrorActionPreference = "Stop"
$root = Split-Path $PSScriptRoot -Parent
Set-Location $root

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

if ($Launch) {
    Write-Host "Creating Fly app (region bom)..." -ForegroundColor Cyan
    & $fly launch --no-deploy --name $AppName --region bom --copy-config --yes
}

if (-not $SkipSecrets) {
    $secretsScript = Join-Path $PSScriptRoot "set-fly-secrets.ps1"
    if (Test-Path (Join-Path $PSScriptRoot "fly-secrets.env")) {
        & $secretsScript -AppName $AppName
    } else {
        Write-Host "Skipping secrets (fly-secrets.env not found). Run set-fly-secrets.ps1 after creating it." -ForegroundColor Yellow
    }
}

Write-Host "Deploying to Fly.io..." -ForegroundColor Cyan
& $fly deploy --app $AppName --yes

Write-Host ""
Write-Host "Health check:" -ForegroundColor Green
Write-Host "  curl https://$AppName.fly.dev/health"
Write-Host ""
Write-Host "Update BillGenieApp-new/.env.production with:"
Write-Host "  EXPO_PUBLIC_API_BASE_URL=https://$AppName.fly.dev"
Write-Host "  EXPO_PUBLIC_WS_BASE_URL=wss://$AppName.fly.dev"
