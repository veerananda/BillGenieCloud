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
  <meta name="theme-color" content="#d97706"/>
  <title>%s · Table %s</title>
  <style>
    :root{
      --ink:#0f172a;
      --ink-soft:#334155;
      --muted:#64748b;
      --line:rgba(15,23,42,.08);
      --surface:#ffffff;
      --bg:#faf6f0;
      --brand:#1bae76;
      --brand-dark:#0f8a5c;
      --brand-wash:rgba(232,248,241,.72);
      --call:#d97706;
      --call-hover:#b45309;
      --call-disabled:#fbbf24;
      --danger:#dc2626;
      --food-tile:url("data:image/svg+xml;utf8,<svg xmlns='http://www.w3.org/2000/svg' width='160' height='160' viewBox='0 0 160 160'><g fill='none' stroke='%%230f8a5c' stroke-width='2.2' opacity='.28' stroke-linecap='round' stroke-linejoin='round'><path d='M28 36c10-2 18 6 18 16 0 12-10 20-18 22-8-2-18-10-18-22 0-10 8-18 18-16z'/><path d='M18 54h20'/><circle cx='118' cy='40' r='14'/><path d='M104 40h28M118 26v28'/><path d='M36 108c0-10 8-18 18-18h8c10 0 18 8 18 18v6H36v-6z'/><path d='M44 96v-6c0-6 4-10 10-10s10 4 10 10v6'/><path d='M112 100c12 0 22 6 22 14s-10 14-22 14-22-6-22-14 10-14 22-14z'/><path d='M98 114h28'/></g><g fill='%%23d97706' opacity='.18'><circle cx='72' cy='28' r='3'/><circle cx='48' cy='78' r='2.5'/><circle cx='132' cy='88' r='3'/><circle cx='24' cy='132' r='2.5'/><circle cx='96' cy='140' r='3'/></g></svg>");
    }
    *{box-sizing:border-box}
    html,body{margin:0;min-height:100%%}
    body{
      font-family:system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;
      color:var(--ink);
      background-color:var(--bg);
      background-image:
        var(--food-tile),
        radial-gradient(900px 420px at 0%% -10%%, rgba(217,119,6,.14), transparent 55%%),
        radial-gradient(700px 360px at 100%% 0%%, rgba(27,174,118,.16), transparent 50%%),
        linear-gradient(180deg, #fff8ef 0%%, #f7faf6 45%%, #f3f7f4 100%%);
      background-size:160px 160px, auto, auto, auto;
      background-attachment:fixed;
    }
    .page{
      min-height:100vh;max-width:560px;margin:0 auto;
      display:flex;flex-direction:column;
      background:transparent;
    }
    .header{
      padding:20px 16px 12px;
      background:transparent;
      border-bottom:0;
      position:sticky;top:0;z-index:5;
    }
    h1{
      margin:0 0 4px;font-size:1.45rem;font-weight:800;letter-spacing:-.02em;color:var(--ink);
      text-shadow:0 1px 0 rgba(255,255,255,.65);
    }
    .sub{margin:0;color:var(--ink-soft);font-size:.95rem;text-shadow:0 1px 0 rgba(255,255,255,.55)}
    .sub strong{color:var(--ink);font-weight:700}
    .meta{
      display:flex;justify-content:space-between;gap:12px;align-items:center;
      margin-top:14px;padding:10px 12px;border-radius:10px;
      background:var(--brand-wash);color:var(--ink-soft);font-size:.9rem;font-weight:600;
      backdrop-filter:blur(4px);
    }
    .meta:empty,.meta.hidden{display:none}
    #totalMeta{color:var(--brand-dark);font-weight:800}
    .actions{padding:14px 16px 0}
    .btn{
      display:flex;width:100%%;align-items:center;justify-content:center;
      padding:14px 16px;border-radius:12px;border:0;font-size:1rem;font-weight:700;
      cursor:pointer;font-family:inherit;
    }
    .btn-call{
      background:var(--call);color:#fff;
      box-shadow:0 8px 20px rgba(217,119,6,.28);
    }
    .btn-call:hover:not(:disabled){background:var(--call-hover)}
    .btn-call:disabled{background:var(--call-disabled);color:#78350f;box-shadow:none;cursor:default}
    .note{margin:10px 16px 0;text-align:center;color:var(--ink-soft);font-size:.85rem;min-height:1.2em;text-shadow:0 1px 0 rgba(255,255,255,.5)}
    .content{
      padding:8px 16px 32px;flex:1;
      margin:4px 0 0;
      background:transparent;
      border:0;
      border-radius:0;
      box-shadow:none;
      backdrop-filter:none;
    }
    .items{margin-top:18px;display:none}
    .items.show{display:block}
    .items h2,.bill h2,.menu-head h2{
      margin:0 0 10px;font-size:1.05rem;font-weight:800;color:var(--ink);
      text-shadow:0 1px 0 rgba(255,255,255,.65);
    }
    .line{display:flex;justify-content:space-between;gap:12px;padding:12px 0;border-bottom:1px solid rgba(15,23,42,.1)}
    .line:last-child{border-bottom:0}
    .line-name{font-weight:700}
    .line-sub{margin-top:3px;color:var(--muted);font-size:.85rem}
    .line-total{font-weight:800;white-space:nowrap;color:var(--ink)}
    .totals{margin-top:14px;padding-top:12px;border-top:1px solid rgba(15,23,42,.1);display:none}
    .totals.show{display:block}
    .tot-row{display:flex;justify-content:space-between;gap:12px;padding:5px 0;color:var(--ink-soft);font-size:.95rem}
    .tot-row.discount{color:var(--brand-dark);font-weight:700}
    .tot-row.total{
      margin-top:8px;padding-top:10px;border-top:1px solid rgba(15,23,42,.1);
      font-size:1.1rem;font-weight:800;color:var(--ink);
    }
    .bill{margin-top:18px;display:none}
    .bill.show{display:block}
    .bill .hint{color:var(--ink-soft);margin:0 0 4px;font-size:.9rem}
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
    .menu-count{font-size:.8rem;color:var(--ink-soft);font-weight:600;text-shadow:0 1px 0 rgba(255,255,255,.55)}
    .cat-scroll{
      display:flex;gap:8px;overflow-x:auto;overflow-y:hidden;
      -webkit-overflow-scrolling:touch;overscroll-behavior-x:contain;
      touch-action:pan-x;padding:2px 0 12px;margin:0;scrollbar-width:none;
    }
    .cat-scroll::-webkit-scrollbar{display:none}
    .cat-chip{
      flex:0 0 auto;border:1px solid rgba(15,138,92,.28);background:transparent;color:var(--ink-soft);
      border-radius:999px;padding:8px 14px;font-size:.86rem;font-weight:700;
      cursor:pointer;white-space:nowrap;font-family:inherit;
      backdrop-filter:blur(2px);
    }
    .cat-chip.active{
      background:var(--brand);border-color:var(--brand);color:#fff;
      backdrop-filter:none;
    }
    .menu-items{display:flex;flex-direction:column;gap:10px}
    .menu-row{
      display:flex;justify-content:space-between;gap:12px;align-items:flex-start;
      padding:14px 14px;
      border:1px solid rgba(15,23,42,.06);
      border-radius:14px;
      background:var(--surface);
      box-shadow:0 4px 14px rgba(15,23,42,.06);
    }
    .menu-row:last-child{border-bottom:1px solid rgba(15,23,42,.06)}
    .menu-row-name{font-weight:700;font-size:.98rem;line-height:1.35}
    .menu-row-desc{margin-top:4px;color:var(--muted);font-size:.82rem;line-height:1.4}
    .menu-row-variants{margin-top:6px;color:var(--muted);font-size:.8rem;line-height:1.45}
    .menu-row-price{font-weight:800;white-space:nowrap;font-size:1rem;color:var(--brand-dark);padding-top:1px}
    .veg,.nonveg{
      display:inline-block;width:8px;height:8px;border-radius:2px;margin-right:6px;vertical-align:middle;
    }
    .veg{background:var(--brand)}
    .nonveg{background:var(--danger)}
    .menu-loading,.menu-empty{color:var(--ink-soft);font-size:.9rem;padding:16px 0;text-align:center;text-shadow:0 1px 0 rgba(255,255,255,.55)}
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
