# BillGenie print agent

On-site helper that receives **SSE wake events** from BillGenieCloud, claims queued **KOT** and **bill** jobs, and prints them to:

- **LAN/Wi‑Fi** ESC/POS printers (TCP port **9100**)
- **Classic Bluetooth** printers paired to this PC and exposed as a **serial/COM** port

Browsers cannot talk to thermal printers reliably on their own for restaurant-wide jobs. The web/app enqueue jobs on the API; this agent delivers them. Chrome/Edge can also pair a Bluetooth printer **in that browser** (see Web → Printers) for local bill printing.

## Setup

1. In **Web → Printers**:
   - Set **KOT printer** and **Bill printer**
     - Network: `192.168.1.50` (port 9100)
     - Bluetooth (Windows): pair the printer in OS Settings, note its COM port, enter `COM5` (or `serial:COM5`)
   - Turn on **KOT printing** / **Bill printing** as needed
   - Click **Generate agent key** and copy it once

2. On a PC that can reach the printers (or has the Bluetooth printer paired):

```bash
set BILLGENIE_API_URL=https://api.thebillgenie.com
set BILLGENIE_PRINT_AGENT_KEY=bgpa_...
go run ./cmd/print-agent
```

Or build a binary:

```bash
go build -o print-agent.exe ./cmd/print-agent
```

Optional: `BILLGENIE_PRINT_AGENT_ID` (defaults to hostname).

## Bluetooth via COM port

1. Pair the thermal printer in Windows Bluetooth settings.
2. Open **Device Manager → Ports (COM & LPT)** and find the printer’s COMx.
3. In BillGenie Printers, set host to `COM5` (example). Port field is ignored for COM targets.
4. Keep the print agent running on **that same PC**.

macOS/Linux: use the device path, e.g. `/dev/cu.Bluetooth-Incoming-Port` or `/dev/rfcomm0`.

## Behavior

| Event | Job |
|--------|-----|
| Dine-in save / add items | KOT (if KOT printing on) |
| Counter create | KOT (if on) |
| `POST /orders/:id/print-bill` | Bill (if bill printing on) |

The agent keeps an open `GET /print-agent/events` SSE connection. When the API enqueues a job it pushes `event: jobs`; the agent then claims and prints. Heartbeats keep the stream alive; on disconnect it reconnects with backoff. There is **no** 2-second polling loop.
