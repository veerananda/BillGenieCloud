# BillGenie print agent

On-site helper that polls the BillGenie API for queued **KOT** and **bill** jobs and prints them to LAN/Wi‑Fi ESC/POS printers (TCP port **9100**).

Browsers cannot talk to thermal printers directly. The web/app enqueue jobs on the API; this agent delivers them.

## Setup

1. In **Web → Profile → Printers (cloud agent)**:
   - Set **KOT printer** IP (kitchen) and **Bill printer** IP (reception)
   - Turn on **KOT printing** / **Bill printing** as needed
   - Click **Generate agent key** and copy it once

2. On a PC that can reach both the printers and the internet:

```bash
set BILLGENIE_API_URL=https://billgenie-api.fly.dev
set BILLGENIE_PRINT_AGENT_KEY=bgpa_...
go run ./cmd/print-agent
```

Or build a binary:

```bash
go build -o print-agent.exe ./cmd/print-agent
```

Optional: `BILLGENIE_PRINT_AGENT_ID` (defaults to hostname).

## Behavior

| Event | Job |
|--------|-----|
| Dine-in save / add items | KOT (if KOT printing on) |
| Counter create | KOT (if on) |
| Payment complete / `POST /orders/:id/print-bill` | Bill (if bill printing on) |

Agent claims jobs every ~2s, prints over TCP, then marks done/failed.
