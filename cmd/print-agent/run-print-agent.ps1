# BillGenie print agent — laptop runner (production API)
# 1. Web → Printers → Generate agent key, paste below
# 2. From restaurant-api:  .\cmd\print-agent\run-print-agent.ps1

$env:BILLGENIE_API_URL = "https://billgenie-api.fly.dev"
$env:BILLGENIE_PRINT_AGENT_KEY = "bgpa_PASTE_KEY_HERE"
# Optional: $env:BILLGENIE_PRINT_AGENT_ID = "laptop"

if ($env:BILLGENIE_PRINT_AGENT_KEY -eq "bgpa_PASTE_KEY_HERE" -or [string]::IsNullOrWhiteSpace($env:BILLGENIE_PRINT_AGENT_KEY)) {
    Write-Error "Set BILLGENIE_PRINT_AGENT_KEY in this script (Web → Printers → Generate agent key)."
    exit 1
}

Set-Location (Join-Path $PSScriptRoot "..\..")
go run ./cmd/print-agent
