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
    .items h2,.bill h2,.menu h2{margin:0 0 10px;font-size:1.05rem}
    .line{display:flex;justify-content:space-between;gap:12px;padding:11px 0;border-bottom:1px solid #e2e8f0}
    .line:last-child{border-bottom:0}
    .line-name{font-weight:700}
    .line-sub{margin-top:3px;color:#64748b;font-size:.85rem}
    .line-total{font-weight:800;white-space:nowrap}
    .totals{margin-top:14px;padding-top:12px;border-top:1px solid #e2e8f0;display:none}
    .totals.show{display:block}
    .tot-row{display:flex;justify-content:space-between;gap:12px;padding:5px 0;color:#475569;font-size:.95rem}
    .tot-row.discount{color:#16a34a}
    .tot-row.total{margin-top:8px;padding-top:10px;border-top:1px solid #e2e8f0;font-size:1.1rem;font-weight:800;color:#0f172a}
    .bill{margin-top:20px;display:none}
    .bill.show{display:block}
    .bill a{display:flex;width:100%%;align-items:center;justify-content:center;padding:12px 14px;border-radius:12px;font-size:.95rem;font-weight:600;text-decoration:none;margin-top:8px}
    .bill .download{background:#0f172a;color:#fff}
    .menu{margin-top:22px;display:none}
    .menu.show{display:block}
    .menu-cat{margin:18px 0 6px;font-size:.75rem;font-weight:800;letter-spacing:.08em;text-transform:uppercase;color:#64748b}
    .menu-cat:first-child{margin-top:0}
    .veg{display:inline-block;width:8px;height:8px;border-radius:2px;background:#16a34a;margin-right:6px;vertical-align:middle}
    .nonveg{display:inline-block;width:8px;height:8px;border-radius:2px;background:#dc2626;margin-right:6px;vertical-align:middle}
    .menu-loading,.menu-empty{color:#94a3b8;font-size:.9rem;padding:8px 0}
  </style>
</head>
<body>
  <div class="page">
    <div class="card">
      <div class="brand">Table session</div>
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
      <div class="totals" id="totalsPanel"></div>
      <div class="bill" id="billPanel">
        <h2>Your bill</h2>
        <p class="sub" style="margin-bottom:0">Review your bill and download a copy. Link stays valid for about an hour.</p>
        <a class="download" id="billDownload" href="#">Download bill</a>
      </div>
      <div class="menu" id="menuPanel">
        <h2>Menu</h2>
        <div id="menuList" class="menu-loading">Loading menu…</div>
      </div>
    </div>
  </div>
  <script>
    const token = %q;
    let state = %s;
    let menuLoaded = false;
    let menuItems = [];
    const callBtn = document.getElementById('callBtn');
    const note = document.getElementById('note');
    const itemsPanel = document.getElementById('itemsPanel');
    const itemsList = document.getElementById('itemsList');
    const totalsPanel = document.getElementById('totalsPanel');
    const billPanel = document.getElementById('billPanel');
    const billDownload = document.getElementById('billDownload');
    const menuPanel = document.getElementById('menuPanel');
    const menuList = document.getElementById('menuList');

    function money(n){ return '₹' + Number(n||0).toFixed(2); }

    function subtotalLabel(s){
      if (s.composite_scheme) return 'Subtotal';
      return s.prices_include_gst ? 'Subtotal (excl. GST)' : 'Subtotal';
    }

    function renderTotals(s){
      totalsPanel.innerHTML = '';
      if (!s.bill_available) {
        totalsPanel.classList.remove('show');
        return;
      }
      const rows = [];
      if (Number(s.sub_total) > 0 && !s.composite_scheme) {
        rows.push(['tot-row', subtotalLabel(s), money(s.sub_total)]);
      }
      if (s.show_tax && Number(s.tax_amount) > 0) {
        rows.push(['tot-row', 'GST (5%%)', money(s.tax_amount)]);
      }
      if (Number(s.discount_amount) > 0) {
        rows.push(['tot-row discount', 'Discount', '-' + money(s.discount_amount)]);
      }
      rows.push(['tot-row total', 'Total', money(s.order_total)]);
      rows.forEach(([cls, label, value]) => {
        const row = document.createElement('div');
        row.className = cls;
        const left = document.createElement('span');
        left.textContent = label;
        const right = document.createElement('span');
        right.textContent = value;
        row.appendChild(left);
        row.appendChild(right);
        totalsPanel.appendChild(row);
      });
      totalsPanel.classList.add('show');
    }

    function renderMenu(){
      menuList.innerHTML = '';
      if (!menuItems.length) {
        menuList.className = 'menu-empty';
        menuList.textContent = 'Menu will appear here when available.';
        return;
      }
      menuList.className = '';
      let lastCat = null;
      menuItems.forEach(item => {
        const cat = item.category || 'Other';
        if (cat !== lastCat) {
          lastCat = cat;
          const h = document.createElement('div');
          h.className = 'menu-cat';
          h.textContent = cat;
          menuList.appendChild(h);
        }
        const row = document.createElement('div');
        row.className = 'line';
        const left = document.createElement('div');
        const name = document.createElement('div');
        name.className = 'line-name';
        const dot = document.createElement('span');
        dot.className = item.is_veg ? 'veg' : 'nonveg';
        name.appendChild(dot);
        name.appendChild(document.createTextNode(item.name || 'Item'));
        left.appendChild(name);
        if (item.description) {
          const sub = document.createElement('div');
          sub.className = 'line-sub';
          sub.textContent = item.description;
          left.appendChild(sub);
        }
        const variants = Array.isArray(item.variants) ? item.variants : [];
        if (variants.length) {
          const sub = document.createElement('div');
          sub.className = 'line-sub';
          sub.textContent = variants.map(v => (v.label || '') + ' ' + money(v.price)).join(' · ');
          left.appendChild(sub);
        }
        const right = document.createElement('div');
        right.className = 'line-total';
        right.textContent = variants.length ? '' : money(item.price);
        row.appendChild(left);
        row.appendChild(right);
        menuList.appendChild(row);
      });
    }

    async function ensureMenu(){
      if (menuLoaded) {
        renderMenu();
        return;
      }
      try {
        const res = await fetch('/a/' + token + '/menu');
        if (!res.ok) throw new Error('menu');
        const data = await res.json();
        menuItems = Array.isArray(data.items) ? data.items : [];
        menuLoaded = true;
        renderMenu();
      } catch (e) {
        menuList.className = 'menu-empty';
        menuList.textContent = 'Could not load menu.';
      }
    }

    function render(s){
      state = s || state;
      document.getElementById('restaurant').textContent = state.restaurant_name || 'Restaurant';
      document.getElementById('tableName').textContent = state.table_name || '';
      const meta = document.getElementById('orderMeta');
      const total = document.getElementById('totalMeta');
      const phase = state.phase || (state.bill_available ? 'checkout' : (state.has_active_order ? 'seated' : 'idle'));
      if (phase === 'checkout' || state.bill_available) {
        meta.textContent = 'Bill ready — please review';
        total.textContent = money(state.order_total);
      } else if (phase === 'seated' || state.has_active_order) {
        meta.textContent = 'Tell your order to the staff';
        total.textContent = '';
      } else {
        meta.textContent = 'Welcome — browse the menu';
        total.textContent = '';
      }

      if (state.assistance_requested) {
        callBtn.disabled = true;
        callBtn.textContent = 'Waiter notified';
        note.textContent = 'Staff has been notified. Someone will be with you shortly.';
      } else {
        callBtn.disabled = false;
        callBtn.textContent = 'Call waiter';
        note.textContent = '';
      }

      const showBill = !!(state.bill_available && state.bill_url);
      const showMenu = state.menu_visible !== false && !showBill;

      const items = Array.isArray(state.items) ? state.items : [];
      itemsList.innerHTML = '';
      if (showBill && items.length) {
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
          parts.push('Qty ' + (item.quantity || 0));
          parts.push('Rate ' + money(item.unit_rate));
          sub.textContent = parts.join(' · ');
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
      renderTotals(state);
      if (showBill) {
        billPanel.classList.add('show');
        billDownload.href = state.bill_download_url || (state.bill_url + '/download');
      } else {
        billPanel.classList.remove('show');
      }
      if (showMenu) {
        menuPanel.classList.add('show');
        ensureMenu();
      } else {
        menuPanel.classList.remove('show');
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

    const pollId = setInterval(() => { refresh(); }, 2500);

    if (window.EventSource) {
      const es = new EventSource('/a/' + token + '/events');
      es.onmessage = (ev) => {
        try { render(JSON.parse(ev.data)); } catch (e) {}
      };
      es.onerror = () => {};
      window.addEventListener('beforeunload', () => { clearInterval(pollId); es.close(); });
    }
  </script>
</body>
</html>`, restaurant, tableName, restaurant, tableName, token, string(initial))
}
