package handlers

import (
	"encoding/json"
	"fmt"
	"html"

	"restaurant-api/internal/services"
)

func renderAssistancePageHTML(token string, status services.AssistanceStatus) string {
	initial, _ := json.Marshal(status)
	restaurant := html.EscapeString(status.RestaurantName)
	if restaurant == "" {
		restaurant = "Restaurant"
	}
	tableName := html.EscapeString(status.TableName)

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8"/>
  <meta name="viewport" content="width=device-width,initial-scale=1"/>
  <title>%s · Table %s</title>
  <style>
    *{box-sizing:border-box}
    body{font-family:system-ui,-apple-system,sans-serif;margin:0;background:#f8fafc;color:#0f172a;padding:24px 16px 40px}
    .page{max-width:420px;margin:0 auto}
    .card{background:#fff;border:1px solid #e2e8f0;border-radius:18px;padding:24px 20px;box-shadow:0 10px 30px rgba(15,23,42,.08)}
    .brand{font-size:11px;letter-spacing:.14em;text-transform:uppercase;color:#64748b;font-weight:700}
    h1{margin:8px 0 4px;font-size:1.5rem}
    .sub{color:#64748b;margin:0 0 20px;font-size:.95rem}
    .meta{display:flex;justify-content:space-between;gap:12px;padding:12px 14px;border-radius:12px;background:#f8fafc;margin-bottom:16px;font-size:.92rem;color:#475569}
    .btn{display:flex;width:100%%;align-items:center;justify-content:center;padding:14px 16px;border-radius:12px;border:0;font-size:1rem;font-weight:700;cursor:pointer}
    .btn-call{background:#2563eb;color:#fff}
    .btn-call:disabled{opacity:.55;cursor:default}
    .note{margin-top:12px;text-align:center;color:#94a3b8;font-size:.85rem;min-height:1.2em}
    .items{margin-top:20px;display:none}
    .items.show{display:block}
    .items h2,.bill h2{margin:0 0 10px;font-size:1.05rem}
    .line{display:flex;justify-content:space-between;gap:12px;padding:11px 0;border-bottom:1px solid #e2e8f0}
    .line:last-child{border-bottom:0}
    .line-name{font-weight:700}
    .line-sub{margin-top:3px;color:#64748b;font-size:.85rem}
    .line-total{font-weight:800;white-space:nowrap}
    .bill{margin-top:20px;display:none}
    .bill.show{display:block}
    .bill a{display:flex;width:100%%;align-items:center;justify-content:center;padding:12px 14px;border-radius:12px;font-size:.95rem;font-weight:600;text-decoration:none;margin-top:8px}
    .bill .download{background:#0f172a;color:#fff}
  </style>
</head>
<body>
  <div class="page">
    <div class="card">
      <div class="brand">Customer assistance</div>
      <h1 id="restaurant">%s</h1>
      <p class="sub">Table <strong id="tableName">%s</strong></p>
      <div class="meta">
        <span id="orderMeta">Loading…</span>
        <span id="totalMeta"></span>
      </div>
      <button class="btn btn-call" id="callBtn" type="button">Call waiter</button>
      <p class="note" id="note"></p>
      <div class="items" id="itemsPanel">
        <h2>Bill items</h2>
        <div id="itemsList"></div>
      </div>
      <div class="bill" id="billPanel">
        <h2>Download bill</h2>
        <p class="sub" style="margin-bottom:0">Your bill is ready to download.</p>
        <a class="download" id="billDownload" href="#">Download bill</a>
      </div>
    </div>
  </div>
  <script>
    const token = %q;
    let state = %s;
    const callBtn = document.getElementById('callBtn');
    const note = document.getElementById('note');
    const itemsPanel = document.getElementById('itemsPanel');
    const itemsList = document.getElementById('itemsList');
    const billPanel = document.getElementById('billPanel');
    const billDownload = document.getElementById('billDownload');

    function money(n){ return '₹' + Number(n||0).toFixed(2); }

    function render(s){
      state = s || state;
      document.getElementById('restaurant').textContent = state.restaurant_name || 'Restaurant';
      document.getElementById('tableName').textContent = state.table_name || '';
      const meta = document.getElementById('orderMeta');
      const total = document.getElementById('totalMeta');
      if (state.bill_available) {
        meta.textContent = 'Bill ready';
        total.textContent = money(state.order_total);
      } else if (state.has_active_order) {
        meta.textContent = 'Table session active';
        total.textContent = '';
      } else if (state.order_status === 'completed') {
        meta.textContent = 'Order completed';
        total.textContent = money(state.order_total);
      } else if (state.order_status === 'cancelled') {
        meta.textContent = 'Order cancelled';
        total.textContent = '';
      } else {
        meta.textContent = 'No active order yet';
        total.textContent = '';
      }
      if (!state.has_active_order) {
        callBtn.disabled = true;
        callBtn.textContent = 'Session closed';
        note.textContent = 'This table session is no longer active.';
      } else if (state.assistance_requested) {
        callBtn.disabled = true;
        callBtn.textContent = 'Waiter notified';
        note.textContent = 'Staff has been notified. Someone will be with you shortly.';
      } else {
        callBtn.disabled = false;
        callBtn.textContent = 'Call waiter';
        note.textContent = '';
      }
      const items = Array.isArray(state.items) ? state.items : [];
      itemsList.innerHTML = '';
      if (items.length) {
        itemsPanel.classList.add('show');
        items.forEach(item => {
          const row = document.createElement('div');
          row.className = 'line';
          const left = document.createElement('div');
          const name = document.createElement('div');
          name.className = 'line-name';
          name.appendChild(document.createTextNode(item.name || 'Item'));
          const sub = document.createElement('div');
          sub.className = 'line-sub';
          const parts = [];
          if (item.category) parts.push(item.category);
          parts.push((item.quantity || 0) + ' x ' + money(item.unit_rate));
          sub.textContent = parts.join(' - ');
          left.appendChild(name);
          left.appendChild(sub);
          const right = document.createElement('div');
          right.className = 'line-total';
          right.textContent = money(item.total);
          row.appendChild(left);
          row.appendChild(right);
          itemsList.appendChild(row);
        });
      } else {
        itemsPanel.classList.remove('show');
      }
      if (state.bill_available && state.bill_url) {
        billPanel.classList.add('show');
        billDownload.href = state.bill_download_url || (state.bill_url + '/download');
      } else {
        billPanel.classList.remove('show');
      }
    }

    async function refresh(){
      try {
        const res = await fetch('/a/' + token + '/status');
        if (!res.ok) return;
        render(await res.json());
      } catch (e) {}
    }

    callBtn.addEventListener('click', async () => {
      callBtn.disabled = true;
      note.textContent = 'Notifying staff…';
      try {
        const res = await fetch('/a/' + token + '/call-waiter', { method: 'POST' });
        const data = await res.json();
        if (data.status) render(data.status);
        else await refresh();
      } catch (e) {
        note.textContent = 'Could not notify staff. Please try again.';
        callBtn.disabled = false;
      }
    });

    render(state);

    if (window.EventSource) {
      const es = new EventSource('/a/' + token + '/events');
      es.onmessage = (ev) => {
        try { render(JSON.parse(ev.data)); } catch (e) {}
      };
      es.onerror = () => {
        // Browser reconnects automatically; keep last known UI state.
      };
    }
  </script>
</body>
</html>`, restaurant, tableName, restaurant, tableName, token, string(initial))
}
