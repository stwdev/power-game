// Power Game UI. Talks to the Go engine (in worker.js) through JSON
// messages. All text is inserted with textContent, never as HTML.
'use strict';

const SAVE_KEY = 'pg-save-v1';
const PREFS_KEY = 'pg-prefs-v1';
const COLORS = ['#e53935', '#1e88e5', '#43a047', '#fdd835', '#8e24aa', '#fb8c00'];
const COLOR_NAMES = ['Red', 'Blue', 'Green', 'Yellow', 'Purple', 'Orange'];
const LEVELS = ['Human', 'AI · Easy', 'AI · Medium', 'AI · Hard'];
const RES = [
  { key: 'coal', icon: '🪨', name: 'Coal' },
  { key: 'oil', icon: '🛢️', name: 'Oil' },
  { key: 'garbage', icon: '🗑️', name: 'Garbage' },
  { key: 'uranium', icon: '☢️', name: 'Uranium' },
];
const KIND = [
  { icon: '🪨', name: 'Coal', cls: 'k-coal' },
  { icon: '🛢️', name: 'Oil', cls: 'k-oil' },
  { icon: '🗑️', name: 'Garbage', cls: 'k-garbage' },
  { icon: '☢️', name: 'Uranium', cls: 'k-uranium' },
  { icon: '🪨🛢️', name: 'Coal/Oil', cls: 'k-hybrid' },
  { icon: '🌱', name: 'Eco', cls: 'k-eco' },
];
const REGION_TINT = ['#4f7cac', '#c9a227', '#3d9970', '#b5523b', '#8e6bbf', '#2a9d8f'];
const SPEEDS = { slow: 1000, normal: 500, fast: 150 };
const SVGNS = 'http://www.w3.org/2000/svg';

// ---------- small DOM helpers ----------
const $ = (id) => document.getElementById(id);

function h(tag, attrs, ...kids) {
  const el = document.createElement(tag);
  setAttrs(el, attrs);
  append(el, kids);
  return el;
}
function s(tag, attrs, ...kids) {
  const el = document.createElementNS(SVGNS, tag);
  setAttrs(el, attrs);
  append(el, kids);
  return el;
}
function setAttrs(el, attrs) {
  if (!attrs) return;
  for (const [k, v] of Object.entries(attrs)) {
    if (v === undefined || v === null || v === false) continue;
    if (k === 'class') el.setAttribute('class', v);
    else if (k === 'text') el.textContent = v;
    else if (k.startsWith('on')) el.addEventListener(k.slice(2), v);
    else el.setAttribute(k, v === true ? '' : String(v));
  }
}
function append(el, kids) {
  for (const k of kids.flat()) {
    if (k === null || k === undefined || k === false) continue;
    el.append(k instanceof Node ? k : document.createTextNode(String(k)));
  }
}
function clear(el) { while (el.firstChild) el.removeChild(el.firstChild); }

function loadJSON(key, fallback) {
  try {
    const v = localStorage.getItem(key);
    return v ? JSON.parse(v) : fallback;
  } catch (_) { return fallback; }
}
function storeJSON(key, v) {
  try { localStorage.setItem(key, typeof v === 'string' ? v : JSON.stringify(v)); } catch (_) { /* storage may be unavailable */ }
}

// ---------- engine bridge ----------
const worker = new Worker('worker.js');
let callSeq = 0;
const pending = new Map();
worker.onmessage = (e) => {
  const { id, out } = e.data || {};
  let res;
  try { res = JSON.parse(out); } catch (_) { res = { error: 'bad engine reply' }; }
  if (id === 0) { showFatal(res.error); return; }
  const cb = pending.get(id);
  if (cb) { pending.delete(id); cb(res); }
};
function call(fn, arg) {
  return new Promise((resolve) => {
    const id = ++callSeq;
    pending.set(id, resolve);
    worker.postMessage({ id, fn, arg: arg === undefined ? '' : arg });
  });
}

// ---------- state ----------
let board = null;
let plants = {};
let view = null;
let busy = false;
let aiTimer = null;
let lastHuman = -1;
let gate = false; // waiting for a pass-and-play hand-over or summary
let ui = { tab: 'map', pick: null, bid: 0, city: null, fire: null, hcoal: 0 };
const prefs = Object.assign({ speed: 'normal', seats: null, count: 3 }, loadJSON(PREFS_KEY, {}));

function game() { return view && view.game; }
function me() { return view && view.current >= 0 ? view.game.players[view.current] : null; }
function isHumanTurn() { const p = me(); return !!p && p.ai === 0; }
function humans() { return view.game.players.filter((p) => p.ai === 0).length; }
function pname(i) { return view.game.players[i].name; }

// ---------- boot ----------
async function boot() {
  const b = await call('board');
  const p = await call('plants');
  if (b.error || p.error) { showFatal(b.error || p.error); return; }
  board = b.ok;
  for (const pl of p.ok) plants[pl.num] = pl;
  buildMap();
  wireUI();
  showSetup();
  // The Android app ships its files in the APK, so it needs no offline cache.
  const inApp = navigator.userAgent.includes('PowerGameApp');
  if ('serviceWorker' in navigator && location.protocol !== 'file:' && !inApp) {
    navigator.serviceWorker.register('sw.js').catch(() => {});
  }
}

function showFatal(msg) {
  const l = $('loading');
  l.classList.remove('hidden');
  clear(l);
  append(l, [h('div', { class: 'logo' }, '⚠️'), h('p', { text: 'Something went wrong: ' + (msg || 'unknown error') })]);
}

function screen(id) {
  for (const s of ['loading', 'setup', 'game']) $(s).classList.toggle('hidden', s !== id);
}

// ---------- setup ----------
function showSetup() {
  screen('setup');
  $('resume').classList.toggle('hidden', !loadJSON(SAVE_KEY, null));
  $('count').value = String(prefs.count || 3);
  renderSeats();
}

function defaultSeats() {
  return COLORS.map((_, i) => ({ name: i === 0 ? 'You' : 'Bot ' + COLOR_NAMES[i], ai: i === 0 ? 0 : 2 }));
}

function renderSeats() {
  const n = Number($('count').value);
  const seats = prefs.seats && prefs.seats.length === 6 ? prefs.seats : defaultSeats();
  const box = $('seats');
  clear(box);
  for (let i = 0; i < n; i++) {
    const name = h('input', { type: 'text', maxlength: 16, value: seats[i].name, 'aria-label': 'Name of player ' + (i + 1), autocomplete: 'off' });
    const sel = h('select', { 'aria-label': 'Who plays seat ' + (i + 1) },
      LEVELS.map((l, k) => h('option', { value: k, selected: seats[i].ai === k }, l)));
    box.append(h('div', { class: 'seat' }, h('span', { class: 'dot', 'data-c': i }), name, sel));
  }
  paintDots(box);
}

function paintDots(root) {
  for (const d of root.querySelectorAll('[data-c]')) d.style.background = COLORS[Number(d.dataset.c)];
}

function readSeats() {
  const out = [];
  for (const row of $('seats').children) {
    const [_, input, sel] = row.children;
    out.push({ name: input.value.slice(0, 16), ai: Number(sel.value) });
  }
  return out;
}

async function startGame() {
  const seats = readSeats();
  const all = defaultSeats();
  seats.forEach((st, i) => { all[i] = st; });
  prefs.seats = all;
  prefs.count = seats.length;
  storeJSON(PREFS_KEY, prefs);
  const r = await call('newGame', JSON.stringify({ players: seats }));
  if (r.error) { toast(r.error); return; }
  lastHuman = -1;
  enterGame(r.ok);
}

async function resumeGame() {
  const saved = loadJSON(SAVE_KEY, null);
  if (!saved) return;
  const r = await call('load', JSON.stringify(saved));
  if (r.error) {
    toast('Saved game could not be loaded.');
    try { localStorage.removeItem(SAVE_KEY); } catch (_) { /* ignore */ }
    showSetup();
    return;
  }
  lastHuman = -1;
  enterGame(r.ok);
}

function enterGame(v) {
  screen('game');
  ui = { tab: 'map', pick: null, bid: 0, city: null, fire: null, hcoal: 0 };
  setTab('map');
  view = null;
  update(v);
  fitMap();
}

// ---------- game loop ----------
// Normalise lists that may arrive as null from an older save.
function tidy(v) {
  const g = v.game;
  v.actual = v.actual || [];
  v.future = v.future || [];
  for (const k of ['market', 'log', 'order', 'regions', 'owners', 'removed']) g[k] = g[k] || [];
  g.owners = g.owners.map((o) => o || []);
  for (const p of g.players) { p.plants = p.plants || []; p.cities = p.cities || []; }
  if (g.auction) g.auction.bidders = g.auction.bidders || [];
  return v;
}

async function update(v) {
  tidy(v);
  const prev = view;
  view = v;
  ui.pick = null;
  ui.fire = null; // power phase: null means "start from the suggestion"
  ui.cart = null; // fuel order not yet confirmed
  ui.city = null;
  render();
  persist();
  if (prev && prev.game.round !== v.game.round && v.game.phase !== 'over') {
    showRoundSummary(prev.game.round);
    return;
  }
  if (v.game.phase === 'over') {
    showGameOver();
    return;
  }
  schedule();
}

async function persist() {
  const r = await call('save');
  if (r.ok) storeJSON(SAVE_KEY, r.ok);
}

function schedule() {
  clearTimeout(aiTimer);
  if (!view || gate || view.current < 0) return;
  const p = me();
  if (p.ai !== 0) {
    aiTimer = setTimeout(botStep, SPEEDS[prefs.speed] || 500);
    return;
  }
  if (humans() > 1 && lastHuman !== view.current) {
    handOver(view.current);
    return;
  }
  lastHuman = view.current;
}

async function botStep() {
  if (busy || gate) return;
  busy = true;
  const r = await call('aiStep');
  busy = false;
  if (r.error) { toast(r.error); renderPanel(); return; }
  update(r.ok);
}

async function act(action) {
  if (busy || !isHumanTurn()) return;
  busy = true;
  action.player = view.current;
  const r = await call('apply', JSON.stringify(action));
  busy = false;
  if (r.error) { toast(r.error); return; }
  update(r.ok);
}

// ---------- dialogs ----------
function openDialog(...content) {
  const d = $('dialog');
  clear(d);
  append(d, content);
  $('overlay').classList.remove('hidden');
}
function closeDialog() { $('overlay').classList.add('hidden'); }

function handOver(i) {
  gate = true;
  const dot = h('span', { class: 'dot big' });
  dot.style.background = COLORS[i];
  openDialog(
    h('h2', {}, dot, ' Pass the phone'),
    h('p', { text: 'Next up: ' + pname(i) + '.' }),
    h('button', { class: 'btn primary wide', type: 'button', onclick: () => {
      gate = false; lastHuman = i; closeDialog(); schedule();
    } }, "I'm " + pname(i) + ' — go'));
}

function showRoundSummary(round) {
  gate = true;
  const g = game();
  const rows = g.order.map((i) => {
    const p = g.players[i];
    const dot = h('span', { class: 'dot' });
    dot.style.background = COLORS[i];
    return h('tr', {}, h('td', {}, dot, ' ', p.name), h('td', { text: String(p.lastPowered) }), h('td', { text: '$' + p.lastIncome }), h('td', { text: '$' + p.money }));
  });
  openDialog(
    h('h2', { text: 'Round ' + round + ' results' }),
    h('table', { class: 'summary' },
      h('thead', {}, h('tr', {}, h('th', { text: 'Player' }), h('th', { text: 'Powered' }), h('th', { text: 'Income' }), h('th', { text: 'Cash' }))),
      h('tbody', {}, rows)),
    stepNote(),
    h('button', { class: 'btn primary wide', type: 'button', onclick: () => { gate = false; closeDialog(); schedule(); } }, 'Next round'));
}

const STEP_INFO = [
  'Each city holds 1 house. 8 plants show in the market; the lowest 4 are for sale.',
  'Each city holds 2 houses. The lowest plant leaves the market when step 2 begins.',
  'Each city holds 3 houses. The market shrinks to 6 plants and all of them are for sale.',
];

function stepTriggers() {
  return [
    'Starts the game.',
    'Starts when someone has ' + view.step2At + ' cities after building.',
    'Starts when the Step 3 card is drawn from the plant deck.',
  ];
}

function restockTable() {
  const g = game();
  return h('table', { class: 'summary restock' },
    h('thead', {}, h('tr', {}, h('th', { text: 'Restock / round' }), RES.map((r) => h('th', { text: r.icon })))),
    h('tbody', {}, view.restock.map((row, i) => h('tr', { class: i + 1 === g.step ? 'current' : '' },
      h('td', { text: 'Step ' + (i + 1) + (i + 1 === g.step ? ' (now)' : '') }),
      row.map((n) => h('td', { text: '+' + n }))))));
}

function stepCard() {
  const g = game();
  const trig = stepTriggers();
  return h('div', { class: 'card step-card' },
    h('div', { class: 'steps' }, [1, 2, 3].map((n) => h('div', { class: 'step' + (n === g.step ? ' now' : n < g.step ? ' done' : '') },
      h('strong', { text: 'Step ' + n + (n === g.step ? ' · now' : '') }),
      h('span', { class: 'small', text: STEP_INFO[n - 1] }),
      h('span', { class: 'small muted', text: trig[n - 1] })))),
    g.step3Pending ? h('p', { class: 'warn', text: 'The Step 3 card has been drawn: step 3 starts at the end of this phase.' }) : null,
    restockTable(),
    h('p', { class: 'muted small', text: 'Restock is limited by what is left in the bank. The game ends when someone reaches ' + view.endAt + ' cities.' }));
}

function showStepInfo() {
  openDialog(h('h2', { text: 'Step ' + game().step + ' of 3' }), stepCard(),
    h('button', { class: 'btn primary wide', type: 'button', onclick: closeDialog }, 'Close'));
}

function stepNote() {
  const g = game();
  const notes = ['Step ' + g.step + '.'];
  if (g.step === 1) notes.push('Step 2 starts when someone has ' + view.step2At + ' cities.');
  notes.push('The game ends when someone reaches ' + view.endAt + ' cities.');
  return h('p', { class: 'muted small', text: notes.join(' ') });
}

function showGameOver() {
  const g = game();
  const list = g.ranking.map((i, k) => {
    const p = g.players[i];
    const dot = h('span', { class: 'dot' });
    dot.style.background = COLORS[i];
    return h('li', {}, dot, ' ', p.name, h('span', { class: 'muted', text: ' — powered ' + p.lastPowered + ', $' + p.money + (k === 0 ? ' 🏆' : '') }));
  });
  try { localStorage.removeItem(SAVE_KEY); } catch (_) { /* ignore */ }
  openDialog(
    h('h2', { text: '🏆 ' + pname(g.ranking[0]) + ' wins!' }),
    h('p', { class: 'muted small', text: 'Most cities powered in the final round wins; ties go to cash.' }),
    h('ol', { class: 'ranking' }, list),
    h('button', { class: 'btn primary wide', type: 'button', onclick: () => { closeDialog(); view = null; showSetup(); } }, 'New game'),
    h('button', { class: 'btn wide', type: 'button', onclick: closeDialog }, 'Look at the board'));
}

function showMenu() {
  const speedSel = h('select', { onchange: (e) => { prefs.speed = e.target.value; storeJSON(PREFS_KEY, prefs); } },
    Object.keys(SPEEDS).map((k) => h('option', { value: k, selected: prefs.speed === k }, k[0].toUpperCase() + k.slice(1))));
  openDialog(
    h('h2', { text: 'Menu' }),
    h('label', { class: 'row' }, 'Bot speed', speedSel),
    h('button', { class: 'btn wide', type: 'button', onclick: showHelp }, 'How to play'),
    h('button', { class: 'btn wide danger', type: 'button', onclick: () => {
      if (!confirm('Abandon this game and start a new one?')) return;
      clearTimeout(aiTimer);
      try { localStorage.removeItem(SAVE_KEY); } catch (_) { /* ignore */ }
      view = null; gate = false; closeDialog(); showSetup();
    } }, 'Quit to new game'),
    h('button', { class: 'btn primary wide', type: 'button', onclick: () => { closeDialog(); } }, 'Close'));
}

function showHelp() {
  const steps = [
    ['1. Turn order', 'The player with the most cities goes first (ties: biggest plant). Leaders choose plants first but buy fuel and build last.'],
    ['2. Auction plants', 'In turn, pick a plant from the top row of the market and bid at least its number. Others raise or pass. Each player buys at most one plant per round (everyone must buy in round 1). You may own ' + (view ? view.maxPlants : 3) + ' plants.'],
    ['3. Buy fuel', 'Last player first. Each plant stores twice what it burns. Prices rise as the market empties.'],
    ['4. Build', 'Last player first. Your first house costs $10. After that you pay the house plus the cheapest road from your network. In step 1 each city holds one house, step 2 two, step 3 three.'],
    ['5. Power & get paid', 'Pick which plants to run (or none, to save fuel). Each powered city earns money; you can never power more cities than you own.'],
    ['Winning', 'When someone reaches the end city count, the player who powers the most cities that round wins.'],
  ];
  const back = view ? showMenu : closeDialog;
  openDialog(
    h('h2', { text: 'How to play' }),
    steps.map(([t, d]) => h('div', { class: 'help' }, h('strong', { text: t }), h('p', { text: d }))),
    h('p', { class: 'muted small', text: 'Unofficial fan game inspired by the classic power-network board game. Original map and art.' }),
    h('button', { class: 'btn primary wide', type: 'button', onclick: back }, 'Back'));
}

let toastTimer = null;
function toast(msg) {
  const t = $('toast');
  t.textContent = msg;
  t.classList.remove('hidden');
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => t.classList.add('hidden'), 2600);
}

// ---------- rendering ----------
function render() {
  renderStatus();
  renderPlayers();
  renderMap();
  if (ui.tab === 'market') renderMarket();
  if (ui.tab === 'log') renderLog();
  renderPanel();
}

const PHASE_NAMES = { auction: 'Auction', resources: 'Buy fuel', build: 'Build', power: 'Power', over: 'Game over' };
function renderStatus() {
  const g = game();
  const st = $('status');
  clear(st);
  append(st, [
    h('strong', { text: 'Round ' + g.round }), ' · ' + PHASE_NAMES[g.phase] + ' ',
    h('button', { class: 'step-pill', type: 'button', onclick: showStepInfo, 'aria-label': 'About step ' + g.step }, 'Step ' + g.step + '/3 ⓘ'),
  ]);
}

function plantCard(num, opts = {}) {
  const pl = plants[num];
  if (!pl) return h('div', { class: 'plant' }, '?');
  const k = KIND[pl.kind];
  const io = pl.input > 0 ? pl.input + k.icon + ' → ' + pl.cities + '🏠' : k.icon + ' → ' + pl.cities + '🏠';
  const tag = opts.onclick ? 'button' : 'div';
  return h(tag, {
    class: 'plant ' + k.cls + (opts.small ? ' small' : '') + (opts.selected ? ' selected' : '') + (opts.dim ? ' dim' : ''),
    type: opts.onclick ? 'button' : undefined,
    disabled: opts.disabled,
    onclick: opts.onclick,
    title: k.name + ' plant ' + num,
  }, h('span', { class: 'num', text: String(num) }), h('span', { class: 'io', text: io }));
}

function renderPlayers() {
  const g = game();
  const box = $('players');
  clear(box);
  for (const i of g.order) {
    const p = g.players[i];
    const dot = h('span', { class: 'dot' });
    dot.style.background = COLORS[i];
    const res = RES.map((r, k) => (p.res[k] ? r.icon + p.res[k] : '')).filter(Boolean).join(' ');
    const chip = h('div', { class: 'chip' + (view.current === i ? ' active' : '') },
      h('div', { class: 'chip-top' }, dot, h('span', { class: 'name', text: p.name }), p.ai ? h('span', { class: 'tag', text: ['', 'E', 'M', 'H'][p.ai] }) : null),
      h('div', { class: 'chip-stats', text: '$' + p.money + ' · ' + p.cities.length + '🏠' + (res ? ' · ' + res : '') }),
      h('div', { class: 'chip-plants' }, p.plants.map((n) => plantCard(n, { small: true }))));
    box.append(chip);
  }
}

function renderMarket() {
  const g = game();
  const box = $('marketView');
  clear(box);
  box.append(h('h3', { text: g.step >= 3 ? 'Plants for sale' : 'Plants for sale (top row)' }));
  box.append(h('div', { class: 'plants' }, view.actual.map((n) => plantCard(n))));
  if (view.future.length) {
    box.append(h('h3', { text: 'Coming next' }));
    box.append(h('div', { class: 'plants' }, view.future.map((n) => plantCard(n, { dim: true }))));
  }
  box.append(h('p', { class: 'muted small', text: 'Deck: ' + g.deckSize + ' cards left.' + (g.step3Pending ? ' Step 3 is coming!' : '') }));
  box.append(h('h3', { text: 'Fuel market' }));
  RES.forEach((r, k) => box.append(resourceTrack(k)));
  box.append(h('h3', { text: 'Game step' }));
  box.append(stepCard());
}

function resourceTrack(k) {
  const g = game();
  const avail = g.resMarket[k];
  const cap = k === 3 ? 12 : 24;
  const cells = [];
  if (k === 3) {
    const prices = [1, 2, 3, 4, 5, 6, 7, 8, 10, 12, 14, 16];
    prices.forEach((pr, idx) => cells.push(trackCell(pr, idx >= cap - avail ? 1 : 0, 1)));
  } else {
    for (let pr = 1; pr <= 8; pr++) {
      let filled = 0;
      for (let idx = (pr - 1) * 3; idx < pr * 3; idx++) if (idx >= cap - avail) filled++;
      cells.push(trackCell(pr, filled, 3));
    }
  }
  return h('div', { class: 'track' },
    h('div', { class: 'track-head' }, RES[k].icon + ' ' + RES[k].name, h('span', { class: 'muted', text: ' ' + avail + ' left, next $' + (view.prices[k] || '—') + ' · restocks +' + view.restock[g.step - 1][k] + '/round · bank ' + g.supply[k] })),
    h('div', { class: 'cells' }, cells));
}

function trackCell(price, filled, slots) {
  const pips = [];
  for (let i = 0; i < slots; i++) pips.push(h('span', { class: 'pip' + (i >= slots - filled ? ' on' : '') }));
  return h('div', { class: 'cell' }, h('span', { class: 'price', text: '$' + price }), h('div', { class: 'pips' }, pips));
}

function renderLog() {
  const box = $('logView');
  clear(box);
  const lines = game().log.slice().reverse();
  box.append(h('ul', { class: 'log' }, lines.map((l) => h('li', { text: l }))));
}

// ---------- action panel ----------
function renderPanel() {
  const panel = $('panel');
  clear(panel);
  const g = game();
  if (g.phase === 'over') {
    panel.append(h('p', { text: '🏆 ' + pname(g.ranking[0]) + ' won the game.' }),
      h('button', { class: 'btn primary wide', type: 'button', onclick: showGameOver }, 'Results'));
    return;
  }
  const p = me();
  if (!p) return;
  if (p.ai !== 0) {
    panel.append(h('p', { class: 'thinking', text: '🤖 ' + p.name + ' is thinking…' }));
    return;
  }
  const head = h('div', { class: 'panel-head' });
  const dot = h('span', { class: 'dot' });
  dot.style.background = COLORS[view.current];
  append(head, [dot, h('strong', { text: p.name }), h('span', { class: 'money', text: '$' + p.money })]);
  panel.append(head);
  if (g.phase === 'auction') {
    if (g.discardFor >= 0) panelDiscard(panel, p);
    else if (g.auction) panelBid(panel, p);
    else panelChoose(panel, p);
  } else if (g.phase === 'resources') panelResources(panel, p);
  else if (g.phase === 'build') panelBuild(panel, p);
  else if (g.phase === 'power') panelPower(panel, p);
}

function stepper(min, max, value, onchange) {
  const out = h('span', { class: 'amount', text: '$' + value });
  let v = value;
  const set = (nv) => { v = Math.max(min, Math.min(max, nv)); out.textContent = '$' + v; onchange(v); };
  return h('div', { class: 'stepper' },
    h('button', { class: 'btn icon', type: 'button', onclick: () => set(v - 5) }, '−5'),
    h('button', { class: 'btn icon', type: 'button', onclick: () => set(v - 1) }, '−'),
    out,
    h('button', { class: 'btn icon', type: 'button', onclick: () => set(v + 1) }, '+'),
    h('button', { class: 'btn icon', type: 'button', onclick: () => set(v + 5) }, '+5'));
}

function panelChoose(panel, p) {
  panel.append(h('p', { class: 'hint', text: 'Pick a plant to auction' + (view.canPass ? ', or pass.' : ' (everyone buys one in round 1).') }));
  panel.append(h('div', { class: 'plants' }, view.actual.map((n) => plantCard(n, {
    selected: ui.pick === n,
    disabled: n > p.money,
    onclick: () => { ui.pick = n; ui.bid = n; renderPanel(); },
  }))));
  const row = h('div', { class: 'actions' });
  if (ui.pick !== null) {
    panel.append(stepper(ui.pick, p.money, ui.bid, (v) => { ui.bid = v; offer.textContent = 'Offer $' + v; }));
    const offer = h('button', { class: 'btn primary', type: 'button', onclick: () => act({ type: 'choose', plant: ui.pick, bid: ui.bid }) }, 'Offer $' + ui.bid);
    row.append(offer);
  }
  if (view.canPass) row.append(h('button', { class: 'btn', type: 'button', onclick: () => act({ type: 'pass' }) }, 'Pass'));
  panel.append(row);
  panel.append(ownPlants(p));
}

function panelBid(panel, p) {
  const a = game().auction;
  panel.append(h('div', { class: 'auction' }, plantCard(a.plant),
    h('div', {}, h('div', { text: 'High bid: $' + a.high }), h('div', { class: 'muted', text: 'by ' + pname(a.highBidder) }),
      h('div', { class: 'muted small', text: 'Still in: ' + a.bidders.map(pname).join(', ') }))));
  const row = h('div', { class: 'actions' });
  if (p.money > a.high) {
    ui.bid = Math.max(ui.bid || 0, a.high + 1);
    if (ui.bid > p.money) ui.bid = a.high + 1;
    let bidBtn;
    panel.append(stepper(a.high + 1, p.money, ui.bid, (v) => { ui.bid = v; bidBtn.textContent = 'Bid $' + v; }));
    bidBtn = h('button', { class: 'btn primary', type: 'button', onclick: () => act({ type: 'bid', bid: ui.bid }) }, 'Bid $' + ui.bid);
    row.append(bidBtn);
  } else {
    panel.append(h('p', { class: 'muted', text: 'You cannot outbid $' + a.high + '.' }));
  }
  row.append(h('button', { class: 'btn', type: 'button', onclick: () => { ui.bid = 0; act({ type: 'pass' }); } }, 'Pass'));
  panel.append(row);
  panel.append(ownPlants(p));
}

function panelDiscard(panel, p) {
  const g = game();
  panel.append(h('p', { class: 'hint', text: 'You can only keep ' + view.maxPlants + ' plants. Scrap one (fuel moves to your other plants where it fits).' }));
  panel.append(h('div', { class: 'plants' }, p.plants.map((n) => plantCard(n, {
    disabled: n === g.justBought,
    onclick: () => act({ type: 'discard', plant: n }),
  }))));
}

function ownPlants(p) {
  if (!p.plants.length) return h('p', { class: 'muted small', text: 'You have no plants yet.' });
  return h('div', { class: 'own' }, h('span', { class: 'muted small', text: 'Yours: ' }), p.plants.map((n) => plantCard(n, { small: true })));
}

// storage mirrors engine.StorageOf / Capacity.Fits.
function storage(list) {
  const c = { coal: 0, oil: 0, hybrid: 0, garbage: 0, uranium: 0 };
  for (const n of list) {
    const pl = plants[n];
    const amt = 2 * pl.input;
    if (pl.kind === 0) c.coal += amt;
    else if (pl.kind === 1) c.oil += amt;
    else if (pl.kind === 2) c.garbage += amt;
    else if (pl.kind === 3) c.uranium += amt;
    else if (pl.kind === 4) c.hybrid += amt;
  }
  return c;
}
function fits(c, res) {
  return res[0] <= c.coal + c.hybrid && res[1] <= c.oil + c.hybrid && res[0] + res[1] <= c.coal + c.oil + c.hybrid &&
    res[2] <= c.garbage && res[3] <= c.uranium;
}
function roomFor(p, k) {
  const c = storage(p.plants);
  let n = 0;
  const res = p.res.slice();
  while (n < 30) { res[k]++; if (!fits(c, res)) break; n++; }
  return n;
}

// unitPrice mirrors engine.UnitPrice: the cheapest unit when avail are left.
const URANIUM_PRICES = [1, 2, 3, 4, 5, 6, 7, 8, 10, 12, 14, 16];
function unitPrice(k, avail) {
  if (avail <= 0) return 0;
  if (k === 3) return URANIUM_PRICES[12 - avail];
  return Math.floor((24 - avail) / 3) + 1;
}
function cartCost(k, n) {
  let cost = 0;
  let avail = game().resMarket[k];
  for (let i = 0; i < n; i++) cost += unitPrice(k, avail--);
  return cost;
}

// The fuel order is only a plan until confirmed; Reset empties it. The
// engine buys the whole order at once (or nothing) when it is confirmed.
function panelResources(panel, p) {
  const g = game();
  if (!ui.cart) ui.cart = [0, 0, 0, 0];
  const cart = ui.cart;
  const total = cart.reduce((sum, n, k) => sum + cartCost(k, n), 0);
  const store = storage(p.plants);
  const after = p.res.map((v, k) => v + cart[k]);
  panel.append(h('p', { class: 'hint', text: 'Add fuel to your order, then confirm. Each plant holds twice what it burns.' }));
  const list = h('div', { class: 'res-list' });
  RES.forEach((r, k) => {
    const more = after.slice();
    more[k]++;
    const fitsMore = fits(store, more);
    if (!fitsMore && p.res[k] === 0 && cart[k] === 0) return;
    const avail = g.resMarket[k] - cart[k];
    const next = unitPrice(k, avail);
    const canAdd = fitsMore && avail > 0 && total + next <= p.money;
    list.append(h('div', { class: 'res-row cart-row' },
      h('span', { class: 'res-name', text: r.icon + ' ' + r.name }),
      h('span', { class: 'muted small', text: 'have ' + p.res[k] + (cart[k] ? ' +' + cart[k] : '') + ' · ' + g.resMarket[k] + ' in market · restocks +' + view.restock[g.step - 1][k] }),
      h('div', { class: 'cart-ctl' },
        h('button', { class: 'btn icon', type: 'button', 'aria-label': 'Remove one ' + r.name, disabled: cart[k] === 0, onclick: () => { cart[k]--; renderPanel(); } }, '−'),
        h('span', { class: 'cart-n', text: String(cart[k]) }),
        h('button', { class: 'btn icon primary', type: 'button', 'aria-label': 'Add one ' + r.name, disabled: !canAdd, onclick: () => { cart[k]++; renderPanel(); } },
          avail > 0 ? '+ $' + next : '—'))));
  });
  if (!list.children.length) list.append(h('p', { class: 'muted', text: 'Your plants need no fuel.' }));
  panel.append(list);
  const items = RES.map((r, k) => (cart[k] ? r.icon + cart[k] : '')).filter(Boolean).join(' ');
  panel.append(h('p', { class: 'small' }, h('strong', { text: 'Order: ' + (items || 'nothing') + ' · $' + total }),
    h('span', { class: 'muted', text: ' (you have $' + p.money + ')' })));
  panel.append(h('div', { class: 'actions' },
    h('button', { class: 'btn', type: 'button', disabled: total === 0, onclick: () => { ui.cart = [0, 0, 0, 0]; renderPanel(); } }, 'Reset'),
    h('button', { class: 'btn primary', type: 'button', onclick: () => act({ type: 'buyAll', amounts: cart.slice() }) },
      total ? 'Confirm · $' + total : 'Finish without buying')));
  panel.append(ownPlants(p));
}

// Income for powering n cities (mirrors engine.Payment).
const PAYMENT = [10, 22, 33, 44, 54, 64, 73, 82, 90, 98, 105, 112, 118, 124, 129, 134, 138, 142, 145, 148, 150];
const income = (n) => PAYMENT[Math.max(0, Math.min(n, PAYMENT.length - 1))];

// planPower mirrors engine.PlanPowering for live feedback; the engine
// re-checks the choice when it is sent.
function planPower(p, fire, hybridCoal) {
  const need = [0, 0, 0, 0];
  let hybrid = 0;
  let cap = 0;
  for (const n of fire) {
    const pl = plants[n];
    cap += pl.cities;
    if (pl.kind === 4) hybrid += pl.input;
    else if (pl.kind < 4) need[pl.kind] += pl.input;
  }
  const hc = Math.max(0, Math.min(hybridCoal, hybrid));
  need[0] += hc;
  need[1] += hybrid - hc;
  const short = RES.filter((_, k) => need[k] > p.res[k]).map((r) => r.name.toLowerCase());
  return { need, hybrid, hc, cap, powered: Math.min(cap, p.cities.length), short };
}

function panelPower(panel, p) {
  const sug = view.suggest || { fired: [], hybridCoal: 0 };
  if (ui.fire === null) {
    ui.fire = new Set(sug.fired);
    ui.hcoal = sug.hybridCoal;
  }
  panel.append(h('p', { class: 'hint', text: 'Choose which plants to run. Fuel you don\'t burn stays for next round.' }));
  if (!p.plants.length) {
    panel.append(h('p', { class: 'muted', text: 'You have no plants to run.' }));
  }
  panel.append(h('div', { class: 'plants' }, p.plants.map((n) => plantCard(n, {
    selected: ui.fire.has(n),
    onclick: () => {
      if (ui.fire.has(n)) ui.fire.delete(n); else ui.fire.add(n);
      renderPanel();
    },
  }))));
  const plan = planPower(p, [...ui.fire], ui.hcoal);
  ui.hcoal = plan.hc;
  if (plan.hybrid > 0) {
    const label = h('span', { class: 'amount small-amount', text: plan.hc + ' 🪨 + ' + (plan.hybrid - plan.hc) + ' 🛢️' });
    panel.append(h('div', { class: 'stepper' },
      h('span', { class: 'muted small', text: 'Hybrid fuel' }),
      h('button', { class: 'btn icon', type: 'button', 'aria-label': 'Less coal', disabled: plan.hc === 0, onclick: () => { ui.hcoal--; renderPanel(); } }, '−🪨'),
      label,
      h('button', { class: 'btn icon', type: 'button', 'aria-label': 'More coal', disabled: plan.hc === plan.hybrid, onclick: () => { ui.hcoal++; renderPanel(); } }, '+🪨')));
  }
  const used = RES.map((r, k) => (plan.need[k] ? r.icon + plan.need[k] : '')).filter(Boolean).join(' ');
  const have = RES.map((r, k) => (p.res[k] ? r.icon + p.res[k] : '')).filter(Boolean).join(' ') || 'none';
  panel.append(h('p', { class: 'small' },
    h('strong', { text: 'Powers ' + plan.powered + ' of your ' + p.cities.length + ' cities → $' + income(plan.powered) }),
    h('br'),
    h('span', { class: 'muted', text: 'Burns: ' + (used || 'nothing') + ' · You have: ' + have })));
  if (plan.short.length) panel.append(h('p', { class: 'warn', text: 'Not enough ' + plan.short.join(' and ') + ' for that choice.' }));
  else if (!p.cities.length && ui.fire.size) panel.append(h('p', { class: 'warn', text: 'You have no cities yet, so running plants only wastes fuel.' }));
  else if (plan.cap > p.cities.length && ui.fire.size) panel.append(h('p', { class: 'warn', text: 'These plants can power ' + plan.cap + ' cities but you only have ' + p.cities.length + ': some fuel is wasted.' }));
  panel.append(h('div', { class: 'actions' },
    h('button', { class: 'btn', type: 'button', onclick: () => { ui.fire = new Set(sug.fired); ui.hcoal = sug.hybridCoal; renderPanel(); } }, 'Best'),
    h('button', { class: 'btn', type: 'button', onclick: () => { ui.fire = new Set(); renderPanel(); } }, 'None')));
  panel.append(h('div', { class: 'actions' },
    h('button', {
      class: 'btn primary', type: 'button', disabled: plan.short.length > 0,
      onclick: () => act({ type: 'power', fire: [...ui.fire], hybridCoal: plan.hc }),
    }, ui.fire.size ? 'Run ' + ui.fire.size + ' plant' + (ui.fire.size > 1 ? 's' : '') + ' · earn $' + income(plan.powered) : 'Run nothing · earn $' + income(0))));
}

function panelBuild(panel, p) {
  const costs = view.buildCosts || [];
  const g = game();
  const cap = p.plants.reduce((s, n) => s + plants[n].cities, 0);
  panel.append(h('p', { class: 'hint', text: 'Tap a glowing city on the map to build. You have ' + p.cities.length + ' cities; your plants can power up to ' + cap + '.' }));
  if (ui.city !== null && costs[ui.city] >= 0) {
    const cost = costs[ui.city];
    panel.append(h('div', { class: 'actions' },
      h('button', { class: 'btn primary', type: 'button', disabled: cost > p.money, onclick: () => act({ type: 'build', city: ui.city }) },
        'Build in ' + board.cities[ui.city].name + ' · $' + cost)));
  }
  const cheapest = costs.map((c, i) => [c, i]).filter(([c]) => c >= 0 && c <= p.money).sort((a, b) => a[0] - b[0]).slice(0, 4);
  if (cheapest.length) {
    panel.append(h('div', { class: 'quick' }, h('span', { class: 'muted small', text: 'Cheapest: ' }),
      cheapest.map(([c, i]) => h('button', { class: 'btn small' + (ui.city === i ? ' selected' : ''), type: 'button', onclick: () => selectCity(i) }, board.cities[i].name + ' $' + c))));
  } else {
    panel.append(h('p', { class: 'muted', text: 'Nothing affordable to build.' }));
  }
  panel.append(h('div', { class: 'actions' }, h('button', { class: 'btn', type: 'button', onclick: () => act({ type: 'done' }) }, 'Done building')));
  if (g.step < 3) panel.append(h('p', { class: 'muted small', text: 'Cities hold ' + g.step + ' house' + (g.step > 1 ? 's' : '') + ' this step.' }));
}

function selectCity(i) {
  ui.city = i;
  renderMap();
  renderPanel();
}

// ---------- map ----------
const mapRefs = { cities: [], regions: [] };
const vb = { x: 0, y: 0, w: 640, h: 940 };

function buildMap() {
  const svg = $('map');
  clear(svg);
  const regionLayer = s('g', { class: 'regions' });
  board.regionNames.forEach((name, r) => {
    const g = s('g', { class: 'region' });
    const pts = board.cities.filter((c) => c.region === r);
    for (const c of pts) g.append(s('circle', { cx: c.x, cy: c.y, r: 62, fill: REGION_TINT[r] }));
    const cx = pts.reduce((a, c) => a + c.x, 0) / pts.length;
    const cy = pts.reduce((a, c) => a + c.y, 0) / pts.length;
    g.append(s('text', { x: cx, y: cy + 4, class: 'region-name', 'text-anchor': 'middle' }, name.toUpperCase()));
    regionLayer.append(g);
    mapRefs.regions[r] = g;
  });
  svg.append(regionLayer);
  const edges = s('g', { class: 'edges' });
  const labels = s('g', { class: 'edge-labels' });
  for (const e of board.edges) {
    const a = board.cities[e.a];
    const b = board.cities[e.b];
    edges.append(s('line', { x1: a.x, y1: a.y, x2: b.x, y2: b.y, 'data-a': e.a, 'data-b': e.b }));
    const mx = (a.x + b.x) / 2;
    const my = (a.y + b.y) / 2;
    labels.append(s('g', { 'data-a': e.a, 'data-b': e.b }, s('circle', { cx: mx, cy: my, r: 9 }), s('text', { x: mx, y: my + 3.5, 'text-anchor': 'middle' }, String(e.cost))));
  }
  svg.append(edges, labels);
  const cityLayer = s('g', { class: 'cities' });
  board.cities.forEach((c, i) => {
    const slots = [0, 1, 2].map((k) => {
      const ang = (-90 + k * 120) * Math.PI / 180;
      return s('circle', { cx: c.x + Math.cos(ang) * 8, cy: c.y + Math.sin(ang) * 8, r: 6, class: 'slot' });
    });
    const ring = s('circle', { cx: c.x, cy: c.y, r: 17, class: 'ring' });
    const cost = s('text', { x: c.x, y: c.y - 22, class: 'cost', 'text-anchor': 'middle' });
    const g = s('g', { class: 'city', 'data-city': i },
      ring, s('circle', { cx: c.x, cy: c.y, r: 15, class: 'base' }), slots,
      s('text', { x: c.x, y: c.y + 29, class: 'label', 'text-anchor': 'middle' }, c.name), cost);
    cityLayer.append(g);
    mapRefs.cities[i] = { g, slots, cost };
  });
  svg.append(cityLayer);
  wireMapGestures(svg);
  setViewBox();
}

function renderMap() {
  if (!view) return;
  const g = game();
  const active = new Set();
  board.cities.forEach((c, i) => { if (g.regions.includes(c.region)) active.add(i); });
  mapRefs.regions.forEach((el, r) => el.classList.toggle('off', !g.regions.includes(r)));
  for (const el of $('map').querySelectorAll('[data-a]')) {
    const on = active.has(Number(el.dataset.a)) && active.has(Number(el.dataset.b));
    el.classList.toggle('off', !on);
  }
  const buildable = g.phase === 'build' && isHumanTurn() ? view.buildCosts || [] : [];
  board.cities.forEach((c, i) => {
    const ref = mapRefs.cities[i];
    ref.g.classList.toggle('off', !active.has(i));
    const owners = g.owners[i] || [];
    ref.slots.forEach((sl, k) => {
      sl.style.fill = owners[k] !== undefined ? COLORS[owners[k]] : '';
      sl.classList.toggle('blocked', k >= g.step && owners[k] === undefined);
    });
    const cost = buildable[i];
    const can = cost !== undefined && cost >= 0;
    ref.g.classList.toggle('can', can && cost <= me().money);
    ref.g.classList.toggle('pricey', can && cost > me().money);
    ref.g.classList.toggle('picked', ui.city === i);
    ref.cost.textContent = can ? '$' + cost : '';
  });
}

function onCityTap(i) {
  if (!view) return;
  const g = game();
  if (g.phase === 'build' && isHumanTurn() && view.buildCosts && view.buildCosts[i] >= 0) {
    selectCity(i);
    return;
  }
  const owners = (g.owners[i] || []).map(pname);
  const c = board.cities[i];
  toast(c.name + ' (' + board.regionNames[c.region] + ')' + (owners.length ? ': ' + owners.join(', ') : ': empty'));
}

function setViewBox() { $('map').setAttribute('viewBox', vb.x + ' ' + vb.y + ' ' + vb.w + ' ' + vb.h); }

function fitMap() {
  if (!view) { Object.assign(vb, { x: 0, y: 0, w: 640, h: 940 }); setViewBox(); return; }
  const g = game();
  const pts = board.cities.filter((c) => g.regions.includes(c.region));
  const minX = Math.min(...pts.map((c) => c.x)) - 50;
  const maxX = Math.max(...pts.map((c) => c.x)) + 50;
  const minY = Math.min(...pts.map((c) => c.y)) - 50;
  const maxY = Math.max(...pts.map((c) => c.y)) + 50;
  Object.assign(vb, { x: minX, y: minY, w: maxX - minX, h: maxY - minY });
  setViewBox();
}

function toSvg(svg, x, y) {
  const m = svg.getScreenCTM();
  if (!m) return { x: vb.x, y: vb.y };
  const pt = new DOMPoint(x, y).matrixTransform(m.inverse());
  return { x: pt.x, y: pt.y };
}

function zoomAt(factor, cx, cy) {
  const nw = Math.max(150, Math.min(1100, vb.w * factor));
  const k = nw / vb.w;
  vb.x = cx - (cx - vb.x) * k;
  vb.y = cy - (cy - vb.y) * k;
  vb.w = nw;
  vb.h = vb.h * k;
  setViewBox();
}

function wireMapGestures(svg) {
  const pts = new Map();
  let moved = 0;
  let pinch = null;
  svg.addEventListener('pointerdown', (e) => {
    svg.setPointerCapture(e.pointerId);
    pts.set(e.pointerId, { x: e.clientX, y: e.clientY });
    if (pts.size === 1) moved = 0;
    if (pts.size === 2) {
      const [a, b] = [...pts.values()];
      pinch = { d: Math.hypot(a.x - b.x, a.y - b.y) };
    }
  });
  svg.addEventListener('pointermove', (e) => {
    const prev = pts.get(e.pointerId);
    if (!prev) return;
    const cur = { x: e.clientX, y: e.clientY };
    pts.set(e.pointerId, cur);
    if (pts.size === 1) {
      const a = toSvg(svg, prev.x, prev.y);
      const b = toSvg(svg, cur.x, cur.y);
      vb.x -= b.x - a.x;
      vb.y -= b.y - a.y;
      moved += Math.abs(cur.x - prev.x) + Math.abs(cur.y - prev.y);
      setViewBox();
    } else if (pts.size === 2 && pinch) {
      const [a, b] = [...pts.values()];
      const d = Math.hypot(a.x - b.x, a.y - b.y);
      if (d > 0) {
        const mid = toSvg(svg, (a.x + b.x) / 2, (a.y + b.y) / 2);
        zoomAt(pinch.d / d, mid.x, mid.y);
        pinch.d = d;
      }
      moved = 999;
    }
  });
  const end = (e) => {
    const had = pts.has(e.pointerId);
    pts.delete(e.pointerId);
    if (pts.size < 2) pinch = null;
    if (had && pts.size === 0 && moved < 10 && e.type === 'pointerup') {
      const hit = document.elementFromPoint(e.clientX, e.clientY);
      const city = hit && hit.closest && hit.closest('[data-city]');
      if (city) onCityTap(Number(city.dataset.city));
    }
  };
  svg.addEventListener('pointerup', end);
  svg.addEventListener('pointercancel', end);
  svg.addEventListener('wheel', (e) => {
    e.preventDefault();
    const c = toSvg(svg, e.clientX, e.clientY);
    zoomAt(e.deltaY > 0 ? 1.15 : 1 / 1.15, c.x, c.y);
  }, { passive: false });
}

// ---------- wiring ----------
function setTab(tab) {
  ui.tab = tab;
  for (const b of document.querySelectorAll('#tabs .tab')) b.classList.toggle('active', b.dataset.tab === tab);
  $('mapView').classList.toggle('hidden', tab !== 'map');
  $('marketView').classList.toggle('hidden', tab !== 'market');
  $('logView').classList.toggle('hidden', tab !== 'log');
  if (view) {
    if (tab === 'market') renderMarket();
    if (tab === 'log') renderLog();
  }
}

function wireUI() {
  $('count').addEventListener('change', () => { prefs.seats = mergeSeats(); renderSeats(); });
  $('start').addEventListener('click', startGame);
  $('resume').addEventListener('click', resumeGame);
  $('howto').addEventListener('click', showHelp);
  $('menuBtn').addEventListener('click', showMenu);
  for (const b of document.querySelectorAll('#tabs .tab')) b.addEventListener('click', () => setTab(b.dataset.tab));
  const center = () => ({ x: vb.x + vb.w / 2, y: vb.y + vb.h / 2 });
  $('zoomIn').addEventListener('click', () => { const c = center(); zoomAt(1 / 1.3, c.x, c.y); });
  $('zoomOut').addEventListener('click', () => { const c = center(); zoomAt(1.3, c.x, c.y); });
  $('zoomFit').addEventListener('click', fitMap);
}

function mergeSeats() {
  const all = prefs.seats && prefs.seats.length === 6 ? prefs.seats.slice() : defaultSeats();
  readSeats().forEach((st, i) => { all[i] = st; });
  return all;
}

boot();
