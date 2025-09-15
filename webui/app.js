let I18N = {};
let CSRF = "";

async function init() {
  bindSidebarNav();
  await loadStatus();
  await loadI18n();
  bindActions();
  connectSSE();
  await loadNetworks();
  await loadPeers();
  await loadSettings();
  setInterval(async () => { await loadStatus(); await loadPeers(); }, 4000);
}

function t(key){ return I18N[key] || key }

function bindSidebarNav(){
  document.querySelectorAll('.sidebar .navlink').forEach(link => {
    link.addEventListener('click', (e) => {
      e.preventDefault();
      document.querySelectorAll('.sidebar .navlink').forEach(a => a.classList.remove('active'));
      link.classList.add('active');
      const target = link.dataset.section;
      document.querySelectorAll('.section').forEach(s => s.classList.remove('active'));
      document.getElementById('section-' + target).classList.add('active');
      const title = target.charAt(0).toUpperCase() + target.slice(1);
      document.getElementById('section-title').innerText = title;
    });
  })
}

async function loadI18n(){
  const lang = window._goc_lang || 'en';
  const res = await fetch(`/i18n/${lang}.json`, {credentials:'same-origin'});
  I18N = await res.json();
  document.getElementById('app-title').innerText = t('app.title');
}

async function loadStatus(){
  const res = await fetch('/api/status', {credentials:'same-origin'});
  // Extract CSRF from cookie
  CSRF = getCookie('goc_csrf') || CSRF;
  const data = await res.json();
  window._goc_lang = data.i18n;
  setBadge('svc-state', data.service_state);
  setBadge('tun-state', data.tun_state);
  setBadge('ctrl-state', data.controller);
}

function setBadge(id, key){
  const el = document.getElementById(id);
  el.innerText = key;
  el.className = 'badge ' + key;
}

async function loadNetworks(){
  const res = await fetch('/api/networks');
  const data = await res.json();
  const ul = document.getElementById('networks-list');
  ul.innerHTML = '';
  data.networks.forEach(n => {
    const li = document.createElement('li');
    li.innerText = `${n.name || n.Name} (${n.id || n.ID})` + (n.joined || n.Joined ? ' [joined]' : '');
    ul.appendChild(li);
  });
}

async function loadPeers(){
  const res = await fetch('/api/peers');
  const data = await res.json();
  const tbody = document.querySelector('#peers-table tbody');
  if (!tbody) return;
  tbody.innerHTML = '';
  data.peers.forEach(p => {
    const tr = document.createElement('tr');
    const mode = (p.P2P ? 'p2p' : (p.Relay ? 'relay' : 'n/a'));
    tr.innerHTML = `<td>${p.ID||p.Address||''}</td><td>${mode}</td><td>${p.RTTms||0} ms</td><td>${formatTime(p.LastSeen)}</td>`;
    tbody.appendChild(tr);
  });
}

function formatTime(v){
  try { const d = new Date(v); return isNaN(d) ? '' : d.toLocaleString(); } catch(e){ return '' }
}

async function loadSettings(){
  const res = await fetch('/api/settings');
  const s = await res.json();
  document.getElementById('port').value = s.Port;
  document.getElementById('mtu').value = s.MTU;
  document.getElementById('log_level').value = s.LogLevel;
  document.getElementById('language').value = s.Language || 'en';
  document.getElementById('autostart').checked = s.Autostart;
  document.getElementById('controller_url').value = s.ControllerURL || '';
  document.getElementById('relay_urls').value = (s.RelayURLs||[]).join(',');
  document.getElementById('udp_port').value = s.UDPPort || 45820;
  document.getElementById('peers').value = (s.Peers||[]).join(',');
}

function bindActions(){
  document.getElementById('btn-start').onclick = () => post('/api/service/start');
  document.getElementById('btn-stop').onclick = () => post('/api/service/stop');
  document.getElementById('btn-restart').onclick = () => post('/api/service/restart');
  document.getElementById('btn-diag').onclick = async () => {
    await post('/api/diag/run');
  };
  document.getElementById('btn-exit').onclick = async () => {
    await post('/api/exit');
  };
  document.getElementById('settings-form').addEventListener('submit', async (e) => {
    e.preventDefault();
    const body = {
      port: parseInt(document.getElementById('port').value, 10),
      mtu: parseInt(document.getElementById('mtu').value, 10),
      log_level: document.getElementById('log_level').value,
      language: document.getElementById('language').value,
      autostart: document.getElementById('autostart').checked,
      controller_url: document.getElementById('controller_url').value,
      relay_urls: document.getElementById('relay_urls').value.split(',').map(s => s.trim()).filter(Boolean),
      udp_port: parseInt(document.getElementById('udp_port').value, 10),
      peers: document.getElementById('peers').value.split(',').map(s => s.trim()).filter(Boolean),
    };
    await put('/api/settings', body);
    await loadI18n();
  });
}

async function post(path){
  await fetch(path, {method:'POST', headers:{'X-CSRF-Token': CSRF, 'Content-Type':'application/json'}})
}

async function put(path, body){
  await fetch(path, {method:'PUT', headers:{'X-CSRF-Token': CSRF, 'Content-Type':'application/json'}, body: JSON.stringify(body)})
}

function connectSSE(){
  const logEl = document.getElementById('log-stream');
  const es = new EventSource('/api/logs/stream');
  es.onmessage = (e) => {
    const line = `[${new Date().toLocaleTimeString()}] ${e.data}`;
    logEl.textContent += line + "\n";
    logEl.scrollTop = logEl.scrollHeight;
  };
}

function getCookie(name){
  const m = document.cookie.match(new RegExp('(^| )' + name + '=([^;]+)'));
  if (m) return m[2];
}

document.addEventListener('DOMContentLoaded', init);
