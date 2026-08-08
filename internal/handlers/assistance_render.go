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
  <meta name="theme-color" content="#1bae76"/>
  <title>%s · Table %s</title>
  <style>
    :root{
      --ink:#0f172a;
      --ink-soft:#334155;
      --muted:#64748b;
      --line:#e2e8f0;
      --surface:#ffffff;
      --bg:#f8fafc;
      --brand:#1bae76;
      --brand-dark:#0f8a5c;
      --brand-wash:#e8f8f1;
      --call:#d97706;
      --call-hover:#b45309;
      --call-disabled:#fbbf24;
      --danger:#dc2626;
    }
    *{box-sizing:border-box}
    html,body{margin:0;min-height:100%%}
    body{
      font-family:system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;
      color:var(--ink);
      background:var(--bg);
    }
    .page{
      min-height:100vh;max-width:560px;margin:0 auto;
      display:flex;flex-direction:column;
      background:var(--surface);
    }
    .header{
      padding:20px 16px 12px;
      background:var(--surface);
      border-bottom:1px solid var(--line);
      position:sticky;top:0;z-index:5;
    }
    h1{margin:0 0 4px;font-size:1.45rem;font-weight:800;letter-spacing:-.02em;color:var(--ink)}
    .sub{margin:0;color:var(--muted);font-size:.95rem}
    .sub strong{color:var(--ink-soft);font-weight:700}
    .meta{
      display:flex;justify-content:space-between;gap:12px;align-items:center;
      margin-top:14px;padding:10px 12px;border-radius:10px;
      background:var(--brand-wash);color:var(--ink-soft);font-size:.9rem;font-weight:600;
    }
    .meta:empty,.meta.hidden{display:none}
    #totalMeta{color:var(--brand-dark);font-weight:800}
    .actions{padding:14px 16px 0}
    .btn{
      display:flex;width:100%%;align-items:center;justify-content:center;
      padding:14px 16px;border-radius:12px;border:0;font-size:1rem;font-weight:700;
      cursor:pointer;font-family:inherit;
    }
    .btn-call{background:var(--call);color:#fff}
    .btn-call:hover:not(:disabled){background:var(--call-hover)}
    .btn-call:disabled{background:var(--call-disabled);color:#78350f;cursor:default}
    .note{margin:10px 16px 0;text-align:center;color:var(--muted);font-size:.85rem;min-height:1.2em}
    .content{padding:8px 16px 32px;flex:1}
    .items{margin-top:18px;display:none}
    .items.show{display:block}
    .items h2,.bill h2,.menu-head h2{
      margin:0 0 10px;font-size:1.05rem;font-weight:800;color:var(--ink);
    }
    .line{display:flex;justify-content:space-between;gap:12px;padding:12px 0;border-bottom:1px solid var(--line)}
    .line:last-child{border-bottom:0}
    .line-name{font-weight:700}
    .line-sub{margin-top:3px;color:var(--muted);font-size:.85rem}
    .line-total{font-weight:800;white-space:nowrap;color:var(--ink)}
    .totals{margin-top:14px;padding-top:12px;border-top:1px solid var(--line);display:none}
    .totals.show{display:block}
    .tot-row{display:flex;justify-content:space-between;gap:12px;padding:5px 0;color:var(--ink-soft);font-size:.95rem}
    .tot-row.discount{color:var(--brand-dark);font-weight:700}
    .tot-row.total{
      margin-top:8px;padding-top:10px;border-top:1px solid var(--line);
      font-size:1.1rem;font-weight:800;color:var(--ink);
    }
    .bill{margin-top:18px;display:none}
    .bill.show{display:block}
    .bill .hint{color:var(--muted);margin:0 0 4px;font-size:.9rem}
    .bill a{
      display:flex;width:100%%;align-items:center;justify-content:center;
      padding:13px 14px;border-radius:12px;font-size:.95rem;font-weight:700;
      text-decoration:none;margin-top:10px;
    }
    .bill .download{background:var(--brand);color:#fff}
    .menu{margin-top:20px;display:none}
    .menu.show{display:block}
    .menu-head{display:flex;align-items:baseline;justify-content:space-between;gap:12px;margin-bottom:10px}
    .menu-head h2{margin:0}
    .menu-count{font-size:.8rem;color:var(--muted);font-weight:600}
    .cat-scroll{
      display:flex;gap:8px;overflow-x:auto;overflow-y:hidden;
      -webkit-overflow-scrolling:touch;overscroll-behavior-x:contain;
      touch-action:pan-x;padding:2px 0 12px;margin:0;scrollbar-width:none;
    }
    .cat-scroll::-webkit-scrollbar{display:none}
    .cat-chip{
      flex:0 0 auto;border:1px solid var(--line);background:var(--bg);color:var(--ink-soft);
      border-radius:999px;padding:8px 14px;font-size:.86rem;font-weight:700;
      cursor:pointer;white-space:nowrap;font-family:inherit;
    }
    .cat-chip.active{
      background:var(--brand);border-color:var(--brand);color:#fff;
    }
    .menu-items{display:flex;flex-direction:column}
    .menu-row{
      display:flex;justify-content:space-between;gap:12px;align-items:flex-start;
      padding:14px 0;border-bottom:1px solid var(--line);
    }
    .menu-row:last-child{border-bottom:0}
    .menu-row-name{font-weight:700;font-size:.98rem;line-height:1.35}
    .menu-row-desc{margin-top:4px;color:var(--muted);font-size:.82rem;line-height:1.4}
    .menu-row-variants{margin-top:6px;color:var(--muted);font-size:.8rem;line-height:1.45}
    .menu-row-price{font-weight:800;white-space:nowrap;font-size:1rem;color:var(--brand-dark);padding-top:1px}
    .veg,.nonveg{
      display:inline-block;width:8px;height:8px;border-radius:2px;margin-right:6px;vertical-align:middle;
    }
    .veg{background:var(--brand)}
    .nonveg{background:var(--danger)}
    .menu-loading,.menu-empty{color:var(--muted);font-size:.9rem;padding:16px 0;text-align:center}
  </style>
</head>
<body>
  <div class="page">
    <header class="header">
      <h1 id="restaurant">%s</h1>
      <p class="sub">Table <strong id="tableName">%s</strong></p>
      <div class="meta" id="metaRow">
        <span id="orderMeta"></span>
        <span id="totalMeta"></span>
      </div>
    </header>
    <div class="actions">
      <button class="btn btn-call" id="callBtn" type="button">Call waiter</button>
    </div>
    <p class="note" id="note"></p>
    <main class="content">
      <div class="items" id="itemsPanel">
        <h2>Bill items</h2>
        <div id="itemsList"></div>
      </div>
      <div class="totals" id="totalsPanel"></div>
      <div class="bill" id="billPanel">
        <h2>Your bill</h2>
        <p class="hint">Review your bill and download a copy. Link stays valid for about an hour.</p>
        <a class="download" id="billDownload" href="#">Download bill</a>
      </div>
      <div class="menu" id="menuPanel">
        <div class="menu-head">
          <h2>Menu</h2>
          <span class="menu-count" id="menuCount"></span>
        </div>
        <div class="cat-scroll" id="menuCats" hidden></div>
        <div id="menuList" class="menu-items menu-loading">Loading menu…</div>
      </div>
    </main>
  </div>
  <script>
    const token = %q;
    let state = %s;
    let menuLoaded = false;
    let menuItems = [];
    let menuCategories = [];
    let selectedCategory = '';
    const callBtn = document.getElementById('callBtn');
    const note = document.getElementById('note');
    const itemsPanel = document.getElementById('itemsPanel');
    const itemsList = document.getElementById('itemsList');
    const totalsPanel = document.getElementById('totalsPanel');
    const billPanel = document.getElementById('billPanel');
    const billDownload = document.getElementById('billDownload');
    const menuPanel = document.getElementById('menuPanel');
    const menuList = document.getElementById('menuList');
    const menuCats = document.getElementById('menuCats');
    const menuCount = document.getElementById('menuCount');

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

    function buildCategories(){
      const seen = {};
      const cats = [];
      menuItems.forEach(item => {
        const cat = (item.category && String(item.category).trim()) || 'Other';
        if (!seen[cat]) {
          seen[cat] = true;
          cats.push(cat);
        }
      });
      return cats;
    }

    function renderCategoryChips(){
      menuCats.innerHTML = '';
      if (menuCategories.length <= 1) {
        menuCats.hidden = true;
        return;
      }
      menuCats.hidden = false;
      menuCategories.forEach(cat => {
        const btn = document.createElement('button');
        btn.type = 'button';
        btn.className = 'cat-chip' + (cat === selectedCategory ? ' active' : '');
        btn.textContent = cat;
        btn.addEventListener('click', () => {
          if (selectedCategory === cat) return;
          selectedCategory = cat;
          renderCategoryChips();
          renderMenuItems();
          const active = menuCats.querySelector('.cat-chip.active');
          if (active && active.scrollIntoView) {
            active.scrollIntoView({ behavior: 'smooth', inline: 'center', block: 'nearest' });
          }
        });
        menuCats.appendChild(btn);
      });
    }

    function renderMenuItemRow(item){
      const row = document.createElement('div');
      row.className = 'menu-row';
      const left = document.createElement('div');
      const name = document.createElement('div');
      name.className = 'menu-row-name';
      const dot = document.createElement('span');
      dot.className = item.is_veg ? 'veg' : 'nonveg';
      name.appendChild(dot);
      name.appendChild(document.createTextNode(item.name || 'Item'));
      left.appendChild(name);
      if (item.description) {
        const sub = document.createElement('div');
        sub.className = 'menu-row-desc';
        sub.textContent = item.description;
        left.appendChild(sub);
      }
      const variants = Array.isArray(item.variants) ? item.variants : [];
      if (variants.length) {
        const sub = document.createElement('div');
        sub.className = 'menu-row-variants';
        sub.textContent = variants.map(v => (v.label || 'Option') + ' ' + money(v.price)).join(' · ');
        left.appendChild(sub);
      }
      const right = document.createElement('div');
      right.className = 'menu-row-price';
      right.textContent = variants.length ? '' : money(item.price);
      row.appendChild(left);
      row.appendChild(right);
      return row;
    }

    function renderMenuItems(){
      menuList.innerHTML = '';
      menuList.className = 'menu-items';
      if (!menuItems.length) {
        menuList.className = 'menu-items menu-empty';
        menuList.textContent = 'Menu will appear here when available.';
        menuCount.textContent = '';
        return;
      }
      const filtered = menuCategories.length <= 1
        ? menuItems
        : menuItems.filter(item => ((item.category && String(item.category).trim()) || 'Other') === selectedCategory);
      menuCount.textContent = filtered.length + (filtered.length === 1 ? ' item' : ' items');
      if (!filtered.length) {
        menuList.className = 'menu-items menu-empty';
        menuList.textContent = 'No items in this category.';
        return;
      }
      filtered.forEach(item => menuList.appendChild(renderMenuItemRow(item)));
    }

    function renderMenu(){
      menuCategories = buildCategories();
      if (!selectedCategory || menuCategories.indexOf(selectedCategory) < 0) {
        selectedCategory = menuCategories[0] || '';
      }
      renderCategoryChips();
      renderMenuItems();
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
        menuCats.hidden = true;
        menuCount.textContent = '';
        menuList.className = 'menu-items menu-empty';
        menuList.textContent = 'Could not load menu.';
      }
    }

    function render(s){
      state = s || state;
      document.getElementById('restaurant').textContent = state.restaurant_name || 'Restaurant';
      document.getElementById('tableName').textContent = state.table_name || '';
      const metaRow = document.getElementById('metaRow');
      const meta = document.getElementById('orderMeta');
      const total = document.getElementById('totalMeta');
      const phase = state.phase || (state.bill_available ? 'checkout' : (state.has_active_order ? 'seated' : 'idle'));
      if (phase === 'checkout' || state.bill_available) {
        meta.textContent = 'Bill ready — please review';
        total.textContent = money(state.order_total);
        metaRow.classList.remove('hidden');
      } else {
        meta.textContent = '';
        total.textContent = '';
        metaRow.classList.add('hidden');
      }

      if (state.assistance_requested) {
        callBtn.disabled = true;
        callBtn.textContent = 'Waiter notified';
        callBtn.hidden = false;
        note.textContent = 'Staff has been notified. Someone will be with you shortly.';
      } else if (state.call_waiter_allowed) {
        callBtn.hidden = false;
        callBtn.disabled = false;
        callBtn.textContent = 'Call waiter';
        note.textContent = '';
      } else {
        callBtn.disabled = true;
        callBtn.hidden = true;
        note.textContent = 'Call waiter is available once you are seated at this table.';
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
      if (!state.call_waiter_allowed) {
        note.textContent = 'Call waiter is available once you are seated at this table.';
        return;
      }
      callBtn.disabled = true;
      note.textContent = 'Notifying staff…';
      try {
        const res = await fetch('/a/' + token + '/call-waiter', { method: 'POST' });
        const data = await res.json();
        if (data.status) render(data.status);
        else await refresh();
        if (!res.ok) note.textContent = data.error || 'Could not notify staff. Please try again.';
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
