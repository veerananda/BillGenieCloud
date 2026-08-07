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
  <meta name="theme-color" content="#0f8a5c"/>
  <title>%s · Table %s</title>
  <link rel="preconnect" href="https://fonts.googleapis.com"/>
  <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin/>
  <link href="https://fonts.googleapis.com/css2?family=DM+Sans:opsz,wght@9..40,500;9..40,600;9..40,700;9..40,800&family=Fraunces:opsz,wght@9..144,600;9..144,700&display=swap" rel="stylesheet"/>
  <style>
    :root{
      --ink:#14352a;
      --ink-soft:#3d5c50;
      --muted:#6b857a;
      --line:#d7ebe2;
      --surface:#ffffff;
      --surface-soft:#f3faf7;
      --brand:#1bae76;
      --brand-dark:#0f8a5c;
      --brand-deep:#0a6b48;
      --brand-wash:#dff7ec;
      --brand-mist:#eefaf4;
      --amber:#c47a1a;
      --amber-soft:#fff6e8;
      --danger:#c43c3c;
      --shadow:0 18px 40px rgba(15,90,60,.10);
    }
    *{box-sizing:border-box}
    body{
      font-family:"DM Sans",system-ui,-apple-system,sans-serif;
      margin:0;color:var(--ink);
      background:
        radial-gradient(1200px 500px at 10%% -10%%, rgba(27,174,118,.22), transparent 55%%),
        radial-gradient(900px 420px at 100%% 0%%, rgba(196,122,26,.12), transparent 50%%),
        linear-gradient(165deg, #eaf8f1 0%%, #f7fbf9 42%%, #fff8f0 100%%);
      min-height:100vh;padding:20px 14px 48px;
    }
    .page{max-width:420px;margin:0 auto}
    .card{
      background:var(--surface);border:1px solid rgba(15,138,92,.12);
      border-radius:24px;overflow:hidden;box-shadow:var(--shadow);
    }
    .hero{
      background:linear-gradient(145deg, var(--brand-deep) 0%%, var(--brand-dark) 52%%, #1bae76 100%%);
      color:#fff;padding:22px 20px 20px;position:relative;
    }
    .hero::after{
      content:"";position:absolute;inset:auto -20%% -40%% auto;width:180px;height:180px;
      background:radial-gradient(circle, rgba(255,255,255,.18), transparent 70%%);pointer-events:none;
    }
    .brand{
      font-size:11px;letter-spacing:.16em;text-transform:uppercase;
      color:rgba(255,255,255,.78);font-weight:700;
    }
    h1{
      font-family:"Fraunces",Georgia,serif;margin:10px 0 6px;
      font-size:1.7rem;font-weight:700;letter-spacing:-.02em;line-height:1.15;
    }
    .sub{color:rgba(255,255,255,.88);margin:0;font-size:.95rem}
    .sub strong{color:#fff;font-weight:800}
    .body{padding:18px 18px 22px}
    .meta{
      display:flex;justify-content:space-between;gap:12px;align-items:center;
      padding:12px 14px;border-radius:14px;margin-bottom:14px;
      background:linear-gradient(135deg, var(--brand-mist), var(--amber-soft));
      border:1px solid rgba(27,174,118,.14);font-size:.9rem;color:var(--ink-soft);font-weight:600;
    }
    #totalMeta{color:var(--brand-dark);font-weight:800}
    .btn{
      display:flex;width:100%%;align-items:center;justify-content:center;
      padding:15px 16px;border-radius:14px;border:0;font-size:1rem;font-weight:800;cursor:pointer;
      font-family:inherit;letter-spacing:.01em;
    }
    .btn-call{
      background:linear-gradient(135deg, #e8a23a 0%%, #c47a1a 100%%);color:#fff;
      box-shadow:0 10px 22px rgba(196,122,26,.28);
    }
    .btn-call:disabled{
      background:linear-gradient(135deg, #9ed9bf, #6fbf9a);color:#fff;
      box-shadow:none;cursor:default;opacity:.9;
    }
    .note{margin-top:12px;text-align:center;color:var(--muted);font-size:.85rem;min-height:1.2em}
    .items{margin-top:20px;display:none}
    .items.show{display:block}
    .items h2,.bill h2,.menu-head h2{
      margin:0 0 10px;font-size:1.05rem;font-family:"Fraunces",Georgia,serif;color:var(--brand-deep);
    }
    .line{display:flex;justify-content:space-between;gap:12px;padding:11px 0;border-bottom:1px solid var(--line)}
    .line:last-child{border-bottom:0}
    .line-name{font-weight:700}
    .line-sub{margin-top:3px;color:var(--muted);font-size:.85rem}
    .line-total{font-weight:800;white-space:nowrap;color:var(--brand-dark)}
    .totals{margin-top:14px;padding-top:12px;border-top:1px solid var(--line);display:none}
    .totals.show{display:block}
    .tot-row{display:flex;justify-content:space-between;gap:12px;padding:5px 0;color:var(--ink-soft);font-size:.95rem}
    .tot-row.discount{color:var(--brand-dark);font-weight:700}
    .tot-row.total{
      margin-top:8px;padding-top:10px;border-top:1px solid var(--line);
      font-size:1.12rem;font-weight:800;color:var(--brand-deep);
    }
    .bill{margin-top:20px;display:none}
    .bill.show{display:block}
    .bill .hint{color:var(--muted);margin:0 0 4px;font-size:.9rem}
    .bill a{
      display:flex;width:100%%;align-items:center;justify-content:center;
      padding:13px 14px;border-radius:14px;font-size:.95rem;font-weight:700;
      text-decoration:none;margin-top:10px;
    }
    .bill .download{
      background:linear-gradient(135deg, var(--brand-dark), var(--brand-deep));color:#fff;
      box-shadow:0 10px 20px rgba(15,138,92,.25);
    }
    .menu{margin-top:22px;display:none}
    .menu.show{display:block}
    .menu-head{display:flex;align-items:baseline;justify-content:space-between;gap:12px;margin-bottom:12px}
    .menu-head h2{margin:0}
    .menu-count{
      font-size:.78rem;color:var(--brand-dark);font-weight:700;
      background:var(--brand-wash);border:1px solid rgba(27,174,118,.2);
      border-radius:999px;padding:4px 10px;
    }
    .cat-scroll{
      display:flex;gap:8px;overflow-x:auto;-webkit-overflow-scrolling:touch;
      padding:2px 2px 14px;margin:0 -2px 2px;scrollbar-width:none;
    }
    .cat-scroll::-webkit-scrollbar{display:none}
    .cat-chip{
      flex:0 0 auto;border:1px solid rgba(15,138,92,.18);background:var(--brand-mist);color:var(--brand-deep);
      border-radius:999px;padding:9px 15px;font-size:.86rem;font-weight:700;
      cursor:pointer;white-space:nowrap;font-family:inherit;
      transition:background .15s,color .15s,border-color .15s,box-shadow .15s,transform .15s;
    }
    .cat-chip.active{
      background:linear-gradient(135deg, var(--brand), var(--brand-dark));
      border-color:transparent;color:#fff;
      box-shadow:0 8px 18px rgba(27,174,118,.28);
      transform:translateY(-1px);
    }
    .menu-items{display:flex;flex-direction:column;gap:10px;min-height:80px}
    .menu-card{
      display:flex;justify-content:space-between;gap:12px;align-items:flex-start;
      padding:14px;border:1px solid rgba(15,138,92,.12);border-radius:16px;
      background:linear-gradient(180deg, #ffffff 0%%, var(--brand-mist) 160%%);
    }
    .menu-card-name{font-weight:700;font-size:.98rem;line-height:1.35;color:var(--ink)}
    .menu-card-desc{margin-top:4px;color:var(--muted);font-size:.82rem;line-height:1.4}
    .menu-card-variants{margin-top:8px;display:flex;flex-wrap:wrap;gap:6px}
    .variant-pill{
      display:inline-flex;align-items:center;gap:4px;padding:4px 9px;border-radius:999px;
      background:var(--amber-soft);border:1px solid rgba(196,122,26,.22);
      font-size:.74rem;font-weight:700;color:var(--amber);
    }
    .menu-card-price{font-weight:800;white-space:nowrap;font-size:1.02rem;color:var(--brand-dark);padding-top:1px}
    .veg,.nonveg{
      display:inline-block;width:9px;height:9px;border-radius:2px;margin-right:7px;vertical-align:middle;
      box-shadow:inset 0 0 0 1px rgba(0,0,0,.08);
    }
    .veg{background:#22a45a}
    .nonveg{background:var(--danger)}
    .menu-loading,.menu-empty{color:var(--muted);font-size:.9rem;padding:16px 0;text-align:center}
  </style>
</head>
<body>
  <div class="page">
    <div class="card">
      <div class="hero">
        <div class="brand">BillGenie · Table session</div>
        <h1 id="restaurant">%s</h1>
        <p class="sub">Table <strong id="tableName">%s</strong></p>
      </div>
      <div class="body">
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
      </div>
    </div>
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

    function renderMenuItemCard(item){
      const row = document.createElement('div');
      row.className = 'menu-card';
      const left = document.createElement('div');
      const name = document.createElement('div');
      name.className = 'menu-card-name';
      const dot = document.createElement('span');
      dot.className = item.is_veg ? 'veg' : 'nonveg';
      name.appendChild(dot);
      name.appendChild(document.createTextNode(item.name || 'Item'));
      left.appendChild(name);
      if (item.description) {
        const sub = document.createElement('div');
        sub.className = 'menu-card-desc';
        sub.textContent = item.description;
        left.appendChild(sub);
      }
      const variants = Array.isArray(item.variants) ? item.variants : [];
      if (variants.length) {
        const wrap = document.createElement('div');
        wrap.className = 'menu-card-variants';
        variants.forEach(v => {
          const pill = document.createElement('span');
          pill.className = 'variant-pill';
          pill.textContent = (v.label || 'Option') + ' · ' + money(v.price);
          wrap.appendChild(pill);
        });
        left.appendChild(wrap);
      }
      const right = document.createElement('div');
      right.className = 'menu-card-price';
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
      filtered.forEach(item => menuList.appendChild(renderMenuItemCard(item)));
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
