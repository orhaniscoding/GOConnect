function activateSection(section) {
  if (!section) {
    section = "dashboard";
  }
  const link = document.querySelector(`.sidebar .navlink[data-section="${section}"]`);
  document.querySelectorAll('.sidebar .navlink').forEach((a) => a.classList.remove('active'));
  if (link) {
    link.classList.add('active');
  }
  document.querySelectorAll('.section').forEach((s) => s.classList.remove('active'));
  const sectionEl = document.getElementById(`section-${section}`);
  if (sectionEl) {
    sectionEl.classList.add('active');
  }
  const titleKey = `nav.${section}`;
  document.getElementById('section-title').innerText = t(titleKey);
}
let I18N = {};
let CSRF = "";
const I18N_VERSION = "20250918-1"; // bump to force cache bust when i18n files change

async function init() {
  bindSidebarNav();
  applyHashNavigation();
  window.addEventListener("hashchange", applyHashNavigation);
  await loadI18n();
  applyHashNavigation();
  await loadStatus();
  bindActions();
  connectSSE();
  await loadNetworks();
  await loadPeers();
  await loadSettings();
  setInterval(async () => {
    await loadStatus();
    await loadPeers();
  }, 4000);
}

function t(key) {
  return I18N[key] || key;
}

function applyTranslations() {
  document.querySelectorAll("[data-i18n]").forEach((el) => {
    el.textContent = t(el.dataset.i18n);
  });
  document.querySelectorAll("[data-i18n-placeholder]").forEach((el) => {
    el.placeholder = t(el.dataset.i18nPlaceholder);
  });
  document.querySelectorAll("[data-i18n-option]").forEach((el) => {
    el.textContent = t(el.dataset.i18nOption);
  });
  const active = document.querySelector(".sidebar .navlink.active");
  if (active) {
    document.getElementById("section-title").innerText = t(`nav.${active.dataset.section}`);
  }
  document.title = t("app.title");
}


function bindSidebarNav() {
  document.querySelectorAll(".sidebar .navlink").forEach((link) => {
    link.addEventListener("click", (e) => {
      e.preventDefault();
      const target = link.dataset.section;
      window.location.hash = `#${target}`;
      activateSection(target);
    });
  });
}

function applyHashNavigation() {
  const section = window.location.hash ? window.location.hash.substring(1) : "dashboard";
  activateSection(section);
}


async function loadI18n() {
  const lang = window._goc_lang || "en";
  // cache-busting query param to ensure updated keys (like nav.chat) are fetched
  const res = await fetch(`/i18n/${lang}.json?v=${encodeURIComponent(I18N_VERSION)}`, { credentials: "same-origin" });
  I18N = await res.json();
  applyTranslations();
  document.getElementById("app-title").innerText = t("app.title");
}

async function loadStatus() {
  try {
    const res = await fetch("/api/status", { credentials: "same-origin" });
    const data = await res.json();
    CSRF = data.csrf_token || CSRF;
  const previousLang = window._goc_lang || "en";
  const nextLang = data.i18n || data.language || previousLang;
    const languageChanged = nextLang !== previousLang;
    window._goc_lang = nextLang;
    // Reflect bearer status (if present)
    if (typeof data.bearer_set === 'boolean') {
      const el = document.getElementById('bearer-meta');
      if (el) {
        el.textContent = data.bearer_set ? (t('settings.bearerSet') || 'Bearer token set') : (t('settings.bearerNotSet') || 'Bearer token not set');
      }
    }
    // Reload translations when backend language changes.
    if (languageChanged) {
      await loadI18n();
    }
  setBadge("svc-state", data.service_state);
  setBadge("tun-state", data.tun_state);
  setBadge("ctrl-state", data.controller);
  updatePublicEndpoint(data.public_endpoint);
  updateTunSelf(data.tun_error);
  // Tray state removed
  } catch (err) {
    console.error("status", err);
  }
}

function setBadge(id, state) {
  const el = document.getElementById(id);
  if (!el) return;
  const key = `status.${state}`;
  const text = I18N[key] ? t(key) : t(state);
  el.textContent = text || state;
  el.className = `badge ${state || "unknown"}`;
  if (state === 'degraded' || state === 'error') {
    el.classList.add('error');
  }
}

function updatePublicEndpoint(value) {
  const el = document.getElementById("public-endpoint");
  if (!el) return;
  if (value) {
    el.textContent = value;
    el.className = "badge info";
  } else {
    el.textContent = t("dashboard.none");
    el.className = "badge stopped";
  }
}

function updateTunSelf(err) {
  const el = document.getElementById("tun-self");
  if (!el) return;
  if (err) {
    el.textContent = err;
    el.className = "badge error";
  } else {
    el.textContent = t("dashboard.tunOk");
    el.className = "badge running";
  }
}

// updateTrayState removed

import { renderNetworkSettingsPanel } from "./networkSettingsPanel.js";
import { renderMemberPreferencesPanel } from "./memberPreferencesPanel.js";
import { renderEffectivePolicyPanel } from "./effectivePolicyPanel.js";

async function loadNetworks() {
  try {
    const res = await fetch("/api/networks");
    const data = await res.json();
    const networks = Array.isArray(data.networks) ? data.networks : [];
    const ul = document.getElementById("networks-list");
    ul.innerHTML = "";
    const detailsPanel = document.getElementById("network-details-panel");
    detailsPanel.innerHTML = "";
    if (networks.length === 0) {
      const li = document.createElement("li");
      li.className = "network-item empty";
      li.textContent = t("dashboard.none");
      ul.appendChild(li);
      return;
    }
    networks.forEach((net) => {
      const id = net.ID || net.id || "";
      const name = net.Name || net.name || id;
      const description = net.Description || net.description || "";
      const address = net.Address || net.address || "";
      const joined = Boolean(net.Joined || net.joined);

      const li = document.createElement("li");
      li.className = "network-item";

      const info = document.createElement("div");
      info.className = "network-info";
      const textParts = [name, `(${id})`];
      // Show NET badge if there's a per-network token for this network
      const ctrl = window._goc_controller_url || '';
      const hasNetToken = ctrl ? Boolean(localStorage.getItem(`goc_owner_token::${ctrl}::${id}`)) : false;
      if (address) {
        textParts.push(address);
      }
      info.textContent = textParts.join(" ");
      if (hasNetToken) {
        const b = document.createElement('span');
        b.className = 'badge info';
        b.style.marginLeft = '6px';
        b.textContent = 'NET';
        b.title = t('owner.badgeNetTooltip') || 'Per-network owner token is set';
        info.appendChild(b);
      }
      if (description) {
        const desc = document.createElement("div");
        desc.className = "network-desc";
        desc.textContent = description;
        info.appendChild(desc);
      }

      const actions = document.createElement("div");
      actions.className = "network-actions";
      const status = document.createElement("span");
      status.className = joined ? "badge running" : "badge stopped";
      status.textContent = joined ? t("status.connected") : t("status.disconnected");
      const btn = document.createElement("button");
      btn.type = "button";
      if (joined) {
        btn.textContent = t("networks.leave");
        btn.addEventListener("click", async () => {
          await leaveNetwork(id);
        });
      } else {
        btn.textContent = t("networks.join");
        btn.addEventListener("click", async () => {
          await joinNetwork({ id, name, description });
        });
      }
      actions.append(status, btn);

      li.append(info, actions);
      li.addEventListener("click", () => showNetworkDetails(id, joined));
      ul.appendChild(li);
    });
    // Populate chat network select with joined networks that have chat allowed (we don't yet know AllowChat here; populate all joined for now)
    const select = document.getElementById('chat-network-select');
    if (select) {
      const current = select.value;
      select.innerHTML = '';
      networks.filter(n=>n.Joined||n.joined).forEach(n => {
        const opt = document.createElement('option');
        opt.value = n.ID || n.id;
        opt.textContent = (n.Name || n.name || n.ID || n.id);
        select.appendChild(opt);
      });
      if (current) {
        select.value = current;
      }
      if (!select.value && select.options.length>0) {
        select.selectedIndex = 0;
      }
      if (select.value) {
        connectChatStream(select.value);
        refreshChatMessages(select.value);
      }
    }
  } catch (err) {
    console.error("networks", err);
  }
}

async function showNetworkDetails(networkId, joined) {
  const detailsPanel = document.getElementById("network-details-panel");
  detailsPanel.innerHTML = "<div>Loading...</div>";
  try {
    // Fetch settings, preferences, and effective policy in parallel
    const [settingsRes, prefsRes, policyRes] = await Promise.all([
      fetch(`/api/v1/networks/${networkId}/settings`),
      fetch(`/api/v1/networks/${networkId}/me/preferences`),
      fetch(`/api/v1/networks/${networkId}/effective?node=me`)
    ]);
    const [settings, prefs, policy] = await Promise.all([
      settingsRes.json(),
      prefsRes.json(),
      policyRes.json()
    ]);
    detailsPanel.innerHTML = "";
    // Owner panel (show if user is owner, here we show for all joined for demo)
    if (joined) {
      const controllerMode = settings.ControllerManaged || settings.controllerManaged;
      detailsPanel.appendChild(renderNetworkSettingsPanel(settings, async (updated) => {
        const resp = await fetch(`/api/v1/networks/${networkId}/settings`, {
          method: "PUT",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(updated)
        });
        if (resp.status === 409) {
          alert("Settings conflict: data changed, reloading.");
        } else if (!resp.ok) {
          const txt = await resp.text();
          alert("Settings update failed: " + txt);
        }
        await loadNetworks();
        showNetworkDetails(networkId, joined);
      }));
      detailsPanel.appendChild(renderMemberPreferencesPanel(prefs, async (updated) => {
        const resp = await fetch(`/api/v1/networks/${networkId}/me/preferences`, {
          method: "PUT",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(updated)
        });
        if (resp.status === 409) {
          alert("Preferences conflict: data changed, reloading.");
        } else if (!resp.ok) {
          const txt = await resp.text();
          alert("Preferences update failed: " + txt);
        }
        await loadNetworks();
        showNetworkDetails(networkId, joined);
      }, controllerMode));
      detailsPanel.appendChild(renderEffectivePolicyPanel(policy));
      // Owner Tools: show when a Controller URL is configured
      if (isControllerConfigured()) {
        const ownerPanel = await renderOwnerToolsPanel(networkId);
        detailsPanel.appendChild(ownerPanel);
      }
    } else {
      detailsPanel.innerHTML = "<div>Not a member of this network.</div>";
    }
  } catch (err) {
    detailsPanel.innerHTML = `<div>Error loading details: ${err.message || err}</div>`;
  }
}

// Helper for proxying to controller with CSRF and owner token headers
// If options._networkId is provided, prefer per-network token: goc_owner_token::<controller>::<networkId>
async function controllerFetch(path, options = {}) {
  const ctrl = window._goc_controller_url || '';
  const nid = options._networkId || '';
  let ownerToken = '';
  if (ctrl && nid) {
    ownerToken = localStorage.getItem(`goc_owner_token::${ctrl}::${nid}`) || '';
  }
  if (!ownerToken) {
    ownerToken = localStorage.getItem(ctrl ? `goc_owner_token::${ctrl}` : 'goc_owner_token') || '';
  }
  const headers = Object.assign({}, options.headers || {}, { 'X-CSRF-Token': CSRF });
  if (ownerToken) headers['X-Owner-Token'] = ownerToken;
  const res = await fetch(path, {
    credentials: 'same-origin',
    ...options,
    headers,
  });
  return res;
}

function isControllerConfigured() {
  return Boolean(window._goc_controller_url);
}

// Build the Owner Tools panel: members + pending requests + admin actions
async function renderOwnerToolsPanel(networkId) {
  const card = document.createElement('div');
  card.className = 'card';
  const ctrl = window._goc_controller_url || '';
  const netTokenKey = ctrl ? `goc_owner_token::${ctrl}::${networkId}` : '';
  const ctrlTokenKey = ctrl ? `goc_owner_token::${ctrl}` : 'goc_owner_token';
  let hasNetToken = netTokenKey ? Boolean(localStorage.getItem(netTokenKey)) : false;
  let hasCtrlToken = Boolean(localStorage.getItem(ctrlTokenKey));
  let metaText = hasNetToken ? (t('owner.tokenSourceNetwork') || 'Using per-network owner token')
                : hasCtrlToken ? (t('owner.tokenSourceController') || 'Using controller-level owner token')
                : (t('owner.tokenSourceNone') || 'Owner token not set');
  const collapseKey = ctrl ? `goc_owner_tools_collapsed::${ctrl}::${networkId}` : `goc_owner_tools_collapsed::${networkId}`;
  const initialCollapsed = localStorage.getItem(collapseKey) === '1';
  card.innerHTML = `
    <h3 style="display:flex;align-items:center;gap:8px;">
      <button type="button" id="owner-toggle" aria-expanded="${!initialCollapsed}" style="font-size:12px;line-height:18px;">${initialCollapsed ? '+' : '–'}</button>
      <span>${t('owner.title') || 'Owner Tools'}</span>
    </h3>
    <div class="meta" id="owner-token-meta">${metaText} ${hasNetToken ? '<span class="badge info" style="margin-left:6px">NET</span>' : ''}</div>
    <div class="row" style="gap:8px;flex-wrap:wrap;align-items:center;">
      <button type="button" id="owner-refresh">${t('owner.refresh') || 'Refresh Snapshot'}</button>
      <button type="button" id="owner-public">${t('owner.makePublic') || 'Make Public'}</button>
      <button type="button" id="owner-private">${t('owner.makePrivate') || 'Make Private'}</button>
      <input type="password" id="owner-new-secret" placeholder="${t('owner.newSecretPlaceholder') || 'New join secret'}" style="min-width:200px;">
      <button type="button" id="owner-rotate">${t('owner.rotateSecret') || 'Rotate Secret'}</button>
      <button type="button" id="owner-delete" style="margin-left:auto;">${t('owner.delete') || 'Delete Network'}</button>
    </div>
    <div class="row" style="gap:8px;flex-wrap:wrap;align-items:center;margin-top:6px;">
      <input type="password" id="owner-network-token" placeholder="${t('owner.networkTokenLabel') || 'Owner token for this network'}" style="min-width:240px;">
      <button type="button" id="owner-save-network-token">${t('owner.saveNetworkToken') || 'Save Network Token'}</button>
      <button type="button" id="owner-clear-network-token">${t('owner.clearNetworkToken') || 'Clear Network Token'}</button>
    </div>
    <div id="owner-flags" class="meta" style="margin-top:6px;"></div>
    <div id="owner-snapshot" class="owner-snapshot" style="margin-top:10px;"></div>
  `;

  const snapEl = card.querySelector('#owner-snapshot');

  async function loadSnapshot() {
    snapEl.innerHTML = `<div>${t('owner.loading') || 'Loading snapshot...'}</div>`;
    try {
  const res = await controllerFetch(`/api/controller/networks/${networkId}/snapshot`, {_networkId: networkId});
      if (!res.ok) {
        const text = await res.text();
        throw new Error(`${res.status} ${res.statusText}${text?`: ${text}`:''}`);
      }
      const data = await res.json();
      renderSnapshot(data);
    } catch (e) {
      snapEl.innerHTML = `<div class="error">${t('owner.snapshotFailed') || 'Snapshot failed'}: ${e.message || e}</div>`;
    }
  }

  function renderSnapshot(snap) {
    const members = Array.isArray(snap.members) ? snap.members : [];
    const reqs = Array.isArray(snap.requests) ? snap.requests : [];
    const bans = Array.isArray(snap.bans) ? snap.bans : [];
    const PAGE = 50;
    const flagsEl = card.querySelector('#owner-flags');
    if (flagsEl) {
      const vis = (snap.visible ? t('owner.flagPublic') || 'Public' : t('owner.flagPrivate') || 'Private');
      const appr = (snap.requireApproval ? t('owner.flagApprovalOn') || 'Approval Required' : t('owner.flagApprovalOff') || 'Auto-Join');
      flagsEl.innerHTML = `<span class="badge ${snap.visible ? 'running':'stopped'}">${vis}</span> <span class="badge info">${appr}</span>`;
    }
    // Search inputs
    const searchRow = document.createElement('div');
    searchRow.className = 'row';
    searchRow.style.gap = '8px';
    searchRow.innerHTML = `
      <input id="owner-search-members" placeholder="${t('owner.searchMembers') || 'Search members'}" style="flex:1;min-width:220px;">
      <input id="owner-search-requests" placeholder="${t('owner.searchRequests') || 'Search requests'}" style="flex:1;min-width:220px;">
      <input id="owner-search-bans" placeholder="${t('owner.searchBans') || 'Search bans'}" style="flex:1;min-width:220px;">
    `;

    const membersTable = document.createElement('table');
    membersTable.className = 'table';
    membersTable.innerHTML = `
      <thead><tr><th>${t('owner.node') || 'Node'}</th><th>${t('owner.nickname') || 'Nickname'}</th><th>${t('owner.ip') || 'IP'}</th><th>${t('owner.actions') || 'Actions'}</th></tr></thead>
      <tbody></tbody>
    `;
    const mtbody = membersTable.querySelector('tbody');
    let memPage = 0;
    const memRender = () => {
      mtbody.innerHTML = '';
      const q = (document.getElementById('owner-search-members')?.value || '').toLowerCase();
      const filtered = q ? members.filter(m => (`${m.nodeId||m.NodeID||''} ${m.nickname||m.Nickname||''} ${m.ip||m.IP||''}`).toLowerCase().includes(q)) : members;
      const start = memPage * PAGE;
      const slice = filtered.slice(start, start + PAGE);
      slice.forEach(m => {
        const tr = document.createElement('tr');
        tr.innerHTML = `
          <td>${m.nodeId || m.NodeID || ''}</td>
          <td>${m.nickname || m.Nickname || ''}</td>
          <td>${m.ip || m.IP || ''}</td>
          <td>
            <button type="button" data-act="kick" data-id="${m.nodeId || m.NodeID || ''}">${t('owner.kick') || 'Kick'}</button>
            <button type="button" data-act="ban" data-id="${m.nodeId || m.NodeID || ''}">${t('owner.ban') || 'Ban'}</button>
          </td>`;
        mtbody.appendChild(tr);
      });
      memPager.textContent = `${t('owner.page') || 'Page'} ${memPage+1} / ${Math.max(1, Math.ceil(filtered.length / PAGE))}`;
      prevBtn.disabled = memPage === 0;
      nextBtn.disabled = start + PAGE >= filtered.length;
    };
    const memPager = document.createElement('span');
    const prevBtn = document.createElement('button'); prevBtn.type='button'; prevBtn.textContent = t('owner.prev') || 'Prev'; prevBtn.onclick=()=>{ if (memPage>0){memPage--; memRender();}};
    const nextBtn = document.createElement('button'); nextBtn.type='button'; nextBtn.textContent = t('owner.next') || 'Next'; nextBtn.onclick=()=>{ memPage++; memRender(); };
    const memControls = document.createElement('div'); memControls.className='row'; memControls.style.gap='8px'; memControls.append(prevBtn, nextBtn, memPager);
    // initial render
    memRender();

    const reqTable = document.createElement('table');
    reqTable.className = 'table';
    reqTable.innerHTML = `
      <thead><tr><th>${t('owner.request') || 'Request'}</th><th>${t('owner.nickname') || 'Nickname'}</th><th>${t('owner.created') || 'Created'}</th><th>${t('owner.actions') || 'Actions'}</th></tr></thead>
      <tbody></tbody>
    `;
    const rtbody = reqTable.querySelector('tbody');
    let reqPage = 0;
    const reqRender = () => {
      rtbody.innerHTML = '';
      const q = (document.getElementById('owner-search-requests')?.value || '').toLowerCase();
      const filtered = q ? reqs.filter(r => (`${r.id||r.ID||''} ${r.nickname||r.Nickname||''}`).toLowerCase().includes(q)) : reqs;
      const start = reqPage * PAGE;
      const slice = filtered.slice(start, start + PAGE);
      slice.forEach(r => {
        const tr = document.createElement('tr');
        const ts = r.createdAt || r.CreatedAt || 0;
        const created = ts ? new Date(ts*1000).toLocaleString() : '';
        tr.innerHTML = `
          <td>${r.id || r.ID || ''}</td>
          <td>${r.nickname || r.Nickname || ''}</td>
          <td>${created}</td>
          <td>
            <button type="button" data-act="approve" data-id="${r.id || r.ID || ''}">${t('owner.approve') || 'Approve'}</button>
            <button type="button" data-act="reject" data-id="${r.id || r.ID || ''}">${t('owner.reject') || 'Reject'}</button>
          </td>`;
        rtbody.appendChild(tr);
      });
      reqPager.textContent = `${t('owner.page') || 'Page'} ${reqPage+1} / ${Math.max(1, Math.ceil(filtered.length / PAGE))}`;
      prevReq.disabled = reqPage === 0;
      nextReq.disabled = start + PAGE >= filtered.length;
    };
    const reqPager = document.createElement('span');
    const prevReq = document.createElement('button'); prevReq.type='button'; prevReq.textContent=t('owner.prev')||'Prev'; prevReq.onclick=()=>{ if (reqPage>0){reqPage--; reqRender();}};
    const nextReq = document.createElement('button'); nextReq.type='button'; nextReq.textContent=t('owner.next')||'Next'; nextReq.onclick=()=>{ reqPage++; reqRender(); };
    const reqControls = document.createElement('div'); reqControls.className='row'; reqControls.style.gap='8px'; reqControls.append(prevReq, nextReq, reqPager);
    reqRender();

    // Bans table
    const bansTable = document.createElement('table');
    bansTable.className = 'table';
    bansTable.innerHTML = `
      <thead><tr><th>${t('owner.banId') || 'Banned Node'}</th><th>${t('owner.actions') || 'Actions'}</th></tr></thead>
      <tbody></tbody>
    `;
    const btbody = bansTable.querySelector('tbody');
    let banPage = 0;
    const banRender = () => {
      btbody.innerHTML = '';
      const q = (document.getElementById('owner-search-bans')?.value || '').toLowerCase();
      const filtered = q ? bans.filter(x => (`${x}`).toLowerCase().includes(q)) : bans;
      const start = banPage * PAGE;
      const slice = filtered.slice(start, start + PAGE);
      slice.forEach(b => {
        const tr = document.createElement('tr');
        tr.innerHTML = `
          <td>${b}</td>
          <td>
            <button type="button" data-act="unban" data-id="${b}">${t('owner.unban') || 'Unban'}</button>
          </td>`;
        btbody.appendChild(tr);
      });
      banPager.textContent = `${t('owner.page') || 'Page'} ${banPage+1} / ${Math.max(1, Math.ceil(filtered.length / PAGE))}`;
      prevBan.disabled = banPage === 0;
      nextBan.disabled = start + PAGE >= filtered.length;
    };
    const banPager = document.createElement('span');
    const prevBan = document.createElement('button'); prevBan.type='button'; prevBan.textContent=t('owner.prev')||'Prev'; prevBan.onclick=()=>{ if (banPage>0){banPage--; banRender();}};
    const nextBan = document.createElement('button'); nextBan.type='button'; nextBan.textContent=t('owner.next')||'Next'; nextBan.onclick=()=>{ banPage++; banRender(); };
    const banControls = document.createElement('div'); banControls.className='row'; banControls.style.gap='8px'; banControls.append(prevBan, nextBan, banPager);
    banRender();

  snapEl.innerHTML = '';
  // Top search row with live filtering
  snapEl.appendChild(searchRow);
  const wrap1 = document.createElement('div');
  const h1 = document.createElement('h4');
  h1.textContent = t('owner.members') || 'Members';
  wrap1.appendChild(h1);
  wrap1.appendChild(membersTable);
  wrap1.appendChild(memControls);
  const wrap2 = document.createElement('div');
  const h2 = document.createElement('h4');
  h2.textContent = t('owner.pending') || 'Pending Join Requests';
  wrap2.appendChild(h2);
  wrap2.appendChild(reqTable);
  wrap2.appendChild(reqControls);
  const wrap3 = document.createElement('div');
  const h3 = document.createElement('h4');
  h3.textContent = t('owner.bans') || 'Bans';
  wrap3.appendChild(h3);
  wrap3.appendChild(bansTable);
  wrap3.appendChild(banControls);
  snapEl.append(wrap1, wrap2, wrap3);

    // live re-filtering without re-fetching
    searchRow.addEventListener('input', () => {
      memPage = 0; reqPage = 0; banPage = 0;
      memRender(); reqRender(); banRender();
    });

    // bind actions on the two tables
    snapEl.querySelectorAll('button[data-act]')?.forEach(btn => {
      btn.addEventListener('click', async () => {
        const act = btn.getAttribute('data-act');
        const id = btn.getAttribute('data-id');
        try {
          if (act === 'kick') {
            await controllerFetch(`/api/controller/networks/${networkId}/admin/kick`, {method:'POST', body: JSON.stringify({nodeId: id}), headers:{'Content-Type':'application/json'}, _networkId: networkId});
          } else if (act === 'ban') {
            await controllerFetch(`/api/controller/networks/${networkId}/admin/ban`, {method:'POST', body: JSON.stringify({nodeId: id}), headers:{'Content-Type':'application/json'}, _networkId: networkId});
          } else if (act === 'approve') {
            await controllerFetch(`/api/controller/networks/${networkId}/admin/approve`, {method:'POST', body: JSON.stringify({requestId: id}), headers:{'Content-Type':'application/json'}, _networkId: networkId});
          } else if (act === 'reject') {
            await controllerFetch(`/api/controller/networks/${networkId}/admin/reject`, {method:'POST', body: JSON.stringify({requestId: id}), headers:{'Content-Type':'application/json'}, _networkId: networkId});
          } else if (act === 'unban') {
            await controllerFetch(`/api/controller/networks/${networkId}/admin/unban`, {method:'POST', body: JSON.stringify({nodeId: id}), headers:{'Content-Type':'application/json'}, _networkId: networkId});
          }
          toast(t('owner.actionSuccess') || 'Action completed', 'success');
          await loadSnapshot();
        } catch (e) {
          toast(`${t('owner.actionFailed') || 'Action failed'}: ${e.message || e}`, 'error');
        }
      });
    });
  }

  // Top-level action bindings
  card.querySelector('#owner-refresh').addEventListener('click', loadSnapshot);
  // search is handled locally via searchRow listener in renderSnapshot
  card.querySelector('#owner-public').addEventListener('click', async () => {
    try {
      await controllerFetch(`/api/controller/networks/${networkId}/admin/visibility`, {method:'POST', headers:{'Content-Type':'application/json'}, body: JSON.stringify({visible:true}), _networkId: networkId});
      await loadSnapshot();
    } catch (e) { toast(`${t('owner.makePublicFailed') || 'Make public failed'}: ${e.message || e}`, 'error'); }
  });
  card.querySelector('#owner-private').addEventListener('click', async () => {
    try {
      await controllerFetch(`/api/controller/networks/${networkId}/admin/visibility`, {method:'POST', headers:{'Content-Type':'application/json'}, body: JSON.stringify({visible:false}), _networkId: networkId});
      await loadSnapshot();
    } catch (e) { toast(`${t('owner.makePrivateFailed') || 'Make private failed'}: ${e.message || e}`, 'error'); }
  });
  card.querySelector('#owner-rotate').addEventListener('click', async () => {
    const val = card.querySelector('#owner-new-secret').value.trim();
    if (!val) { alert(t('owner.enterSecret') || 'Enter a new secret'); return; }
    try {
      await controllerFetch(`/api/controller/networks/${networkId}/admin/secret`, {method:'POST', headers:{'Content-Type':'application/json'}, body: JSON.stringify({joinSecret: val}), _networkId: networkId});
      card.querySelector('#owner-new-secret').value = '';
      await loadSnapshot();
    } catch (e) { toast(`${t('owner.rotateFailed') || 'Rotate secret failed'}: ${e.message || e}`, 'error'); }
  });
  card.querySelector('#owner-delete').addEventListener('click', async () => {
    if (!confirm(t('owner.deleteConfirm') || 'Delete this network? This cannot be undone.')) return;
    try {
      const res = await controllerFetch(`/api/controller/networks/${networkId}`, {method:'DELETE'});
      if (!res.ok && res.status !== 204) {
        const txt = await res.text();
        throw new Error(`${res.status} ${res.statusText}${txt?`: ${txt}`:''}`);
      }
      // after delete, refresh networks list
      toast(t('owner.actionSuccess') || 'Action completed', 'success');
      await loadNetworks();
    } catch (e) { toast(`${t('owner.actionFailed') || 'Action failed'}: ${e.message || e}`, 'error'); }
  });

  // collapse/expand behavior + lazy load
  const toggleBtn = card.querySelector('#owner-toggle');
  const rows = card.querySelectorAll('.row');
  const flagsEl = card.querySelector('#owner-flags');
  function setCollapsed(collapsed) {
    rows.forEach(r => { r.style.display = collapsed ? 'none' : ''; });
    if (flagsEl) flagsEl.style.display = collapsed ? 'none' : '';
    if (snapEl) snapEl.style.display = collapsed ? 'none' : '';
    if (toggleBtn) {
      toggleBtn.textContent = collapsed ? '+' : '–';
      toggleBtn.setAttribute('aria-expanded', String(!collapsed));
    }
  }
  setCollapsed(initialCollapsed);
  if (toggleBtn) {
    toggleBtn.addEventListener('click', async () => {
      const collapsed = toggleBtn.getAttribute('aria-expanded') === 'false';
      const nextCollapsed = !collapsed;
      setCollapsed(nextCollapsed);
      localStorage.setItem(collapseKey, nextCollapsed ? '1' : '0');
      if (!nextCollapsed && snapEl && !snapEl.hasChildNodes()) {
        await loadSnapshot();
      }
    });
  }
  if (!initialCollapsed) {
    loadSnapshot();
  }
  // Initialize per-network token field
  const tokenInput = card.querySelector('#owner-network-token');
  const metaEl = card.querySelector('#owner-token-meta');
  if (tokenInput) {
    tokenInput.value = netTokenKey ? (localStorage.getItem(netTokenKey) || '') : '';
  }
  const saveBtn = card.querySelector('#owner-save-network-token');
  if (saveBtn) {
    saveBtn.addEventListener('click', () => {
      if (!netTokenKey) return;
      const v = (tokenInput.value || '').trim();
      if (v) {
        localStorage.setItem(netTokenKey, v);
        toast(t('owner.tokenSaved') || 'Owner token saved for this network', 'success');
        // dynamic meta update without reload
        hasNetToken = true; hasCtrlToken = Boolean(localStorage.getItem(ctrlTokenKey));
        metaText = hasNetToken ? (t('owner.tokenSourceNetwork') || 'Using per-network owner token')
                  : hasCtrlToken ? (t('owner.tokenSourceController') || 'Using controller-level owner token')
                  : (t('owner.tokenSourceNone') || 'Owner token not set');
  if (metaEl) metaEl.innerHTML = `${metaText} <span class="badge info" style="margin-left:6px" title="${t('owner.badgeNetTooltip') || 'Per-network owner token is set'}">NET</span>`;
      }
    });
  }
  const clearBtn = card.querySelector('#owner-clear-network-token');
  if (clearBtn) {
    clearBtn.addEventListener('click', () => {
      if (!netTokenKey) return;
      localStorage.removeItem(netTokenKey);
      if (tokenInput) tokenInput.value = '';
      toast(t('owner.tokenCleared') || 'Owner token cleared for this network', 'success');
      // dynamic meta update without reload
      hasNetToken = false; hasCtrlToken = Boolean(localStorage.getItem(ctrlTokenKey));
      metaText = hasNetToken ? (t('owner.tokenSourceNetwork') || 'Using per-network owner token')
                : hasCtrlToken ? (t('owner.tokenSourceController') || 'Using controller-level owner token')
                : (t('owner.tokenSourceNone') || 'Owner token not set');
  if (metaEl) metaEl.innerHTML = metaText + (hasNetToken ? ` <span class="badge info" style="margin-left:6px" title="${t('owner.badgeNetTooltip') || 'Per-network owner token is set'}">NET</span>` : '');
    });
  }
  return card;
}

// Toast notifications
function ensureToastContainer() {
  if (document.getElementById('toast-container')) return;
  const div = document.createElement('div');
  div.id = 'toast-container';
  div.style.position = 'fixed';
  div.style.right = '16px';
  div.style.bottom = '16px';
  div.style.zIndex = '9999';
  document.body.appendChild(div);
}

function toast(message, type = 'info') {
  ensureToastContainer();
  const div = document.createElement('div');
  div.textContent = message;
  div.style.marginTop = '8px';
  div.style.padding = '10px 12px';
  div.style.borderRadius = '4px';
  div.style.fontSize = '13px';
  div.style.color = '#fff';
  div.style.boxShadow = '0 2px 8px rgba(0,0,0,0.4)';
  div.style.transition = 'opacity 0.3s';
  div.style.opacity = '1';
  if (type === 'success') div.style.background = '#2e7d32';
  else if (type === 'error') div.style.background = '#c62828';
  else div.style.background = '#424242';
  document.getElementById('toast-container').appendChild(div);
  setTimeout(() => { div.style.opacity = '0'; }, 2800);
  setTimeout(() => { div.remove(); }, 3200);
}

async function joinNetwork(payload) {
  try {
    await postJSON("/api/networks/join", payload);
    await loadNetworks();
    await loadStatus();
  } catch (err) {
    console.error("network join", err);
    alert(`Join failed: ${err.message || err}`);
  }
}

async function leaveNetwork(id) {
  try {
    await postJSON("/api/networks/leave", { id });
    await loadNetworks();
    await loadStatus();
  } catch (err) {
    console.error("network leave", err);
    alert(`Leave failed: ${err.message || err}`);
  }
}

async function loadPeers() {
  try {
    const res = await fetch("/api/peers");
    const data = await res.json();
    const peers = Array.isArray(data.peers) ? data.peers : [];
    const tbody = document.querySelector("#peers-table tbody");
    if (!tbody) return;
    tbody.innerHTML = "";
    peers.forEach((p) => {
      const tr = document.createElement("tr");
      const modeKey = p.P2P ? "mode.p2p" : p.Relay ? "mode.relay" : "mode.none";
      tr.innerHTML = `<td>${p.ID || p.Address || ""}</td><td>${t(modeKey)}</td><td>${p.RTTms || 0} ms</td><td>${formatTime(p.LastSeen)}</td>`;
      tbody.appendChild(tr);
    });
  } catch (err) {
    console.error("peers", err);
  }
}

function formatTime(v) {
  try {
    const d = new Date(v);
    return Number.isNaN(d.getTime()) ? "" : d.toLocaleString();
  } catch (e) {
    return "";
  }
}

async function loadSettings() {
  try {
    const res = await fetch("/api/settings");
    const s = await res.json();
    window._goc_controller_url = s.ControllerURL || "";
    document.getElementById("port").value = s.Port;
    document.getElementById("mtu").value = s.MTU;
    document.getElementById("log_level").value = s.LogLevel;
    document.getElementById("language").value = s.Language || "en";
    document.getElementById("autostart").checked = s.Autostart;
    document.getElementById("controller_url").value = s.ControllerURL || "";
    document.getElementById("relay_urls").value = (s.RelayURLs || []).join(",");
    document.getElementById("udp_port").value = s.UDPPort || 45820;
    document.getElementById("peers").value = (s.Peers || []).join(",");
    document.getElementById("stun_servers").value = (s.StunServers || []).join(",");
  const tpc = (s.TrustedPeerCerts || []).join("\n\n");
  // restore owner token (keyed by controller URL)
  const tokenKey = window._goc_controller_url ? `goc_owner_token::${window._goc_controller_url}` : 'goc_owner_token';
  const ownerToken = localStorage.getItem(tokenKey) || '';
    const ownerEl = document.getElementById('owner_token');
    if (ownerEl) ownerEl.value = ownerToken;
    const tpcEl = document.getElementById("trusted_peer_certs");
    if (tpcEl) tpcEl.value = tpc;
    // update bearer meta from settings response, if present
    if (typeof s.bearer_set === 'boolean') {
      const el = document.getElementById('bearer-meta');
      if (el) {
        el.textContent = s.bearer_set ? (t('settings.bearerSet') || 'Bearer token set') : (t('settings.bearerNotSet') || 'Bearer token not set');
      }
    }
  } catch (err) {
    console.error("settings load", err);
  }
}

function bindActions() {
  document.getElementById("btn-start").onclick = async () => {
    try {
      await post("/api/service/start");
    } catch (err) {
      console.error("service start", err);
      alert(`Start failed: ${err.message || err}`);
    }
  };
  document.getElementById("btn-stop").onclick = async () => {
    try {
      await post("/api/service/stop");
    } catch (err) {
      console.error("service stop", err);
      alert(`Stop failed: ${err.message || err}`);
    }
  };
  document.getElementById("btn-restart").onclick = async () => {
    try {
      await post("/api/service/restart");
    } catch (err) {
      console.error("service restart", err);
      alert(`Restart failed: ${err.message || err}`);
    }
  };
  document.getElementById("btn-diag").onclick = async () => {
    try {
      const res = await postJSON("/api/diag/run", {});
      const diag = await res.json();
      const networks = Array.isArray(diag.networks) ? diag.networks : [];
      const joined = networks.filter((n) => n.Joined || n.joined).length;
      const lines = [
        `${t("dashboard.tunSelf")}: ${diag.tun_ok ? t("status.ok") : diag.tun_error || t("status.error")}`,
        `${t("dashboard.publicEndpoint")}: ${diag.public_endpoint || t("dashboard.none")}`,
        `${t("nav.networks")}: ${joined}/${networks.length}`,
      ];
      alert(`${t("diag.result")}\n${lines.join("\n")}`);
    } catch (err) {
      console.error("diagnostics", err);
      alert(`Diagnostics failed: ${err.message || err}`);
    }
  };
  document.getElementById("btn-exit").onclick = async () => {
    try {
      await post("/api/exit");
    } catch (err) {
      console.error("service exit", err);
      alert(`Shutdown failed: ${err.message || err}`);
    }
  };
  // Token management buttons
  const btnRegen = document.getElementById('btn-token-regenerate');
  if (btnRegen) {
    btnRegen.addEventListener('click', async () => {
      try {
        const res = await fetch('/api/token/regenerate', {method:'POST', credentials:'same-origin', headers:{'X-CSRF-Token': CSRF}});
        if (!res.ok) {
          const txt = await res.text();
          throw new Error(`${res.status} ${res.statusText}${txt?`: ${txt}`:''}`);
        }
        // Set UI meta and set cookie so Web UI requests pass auth without manual header
        document.cookie = `goc_bearer=; Max-Age=0; Path=/`; // clear old
        // Since backend does not return token (by design), ask user to copy from config if needed.
        // But we can keep flow by leaving cookie unset; subsequent calls will still fail unless token entered.
        // Show hint
        alert(t('settings.tokenRegenerated') || 'Token regenerated. Please copy the new token from config.yaml if you plan to use external tools. Web UI continues to work on this tab.');
        await loadStatus();
        await loadSettings();
      } catch (e) {
        alert(`Token regenerate failed: ${e.message || e}`);
      }
    });
  }
  const btnClear = document.getElementById('btn-token-clear');
  if (btnClear) {
    btnClear.addEventListener('click', async () => {
      try {
        const res = await fetch('/api/token/clear', {method:'POST', credentials:'same-origin', headers:{'X-CSRF-Token': CSRF}});
        if (!res.ok) {
          const txt = await res.text();
          throw new Error(`${res.status} ${res.statusText}${txt?`: ${txt}`:''}`);
        }
        document.cookie = `goc_bearer=; Max-Age=0; Path=/`;
        await loadStatus();
        await loadSettings();
      } catch (e) {
        alert(`Token clear failed: ${e.message || e}`);
      }
    });
  }
  const form = document.getElementById("settings-form");
  if (form) {
    form.addEventListener("submit", async (e) => {
      e.preventDefault();
      const ownerEl = document.getElementById('owner_token');
      const ownerToken = ownerEl ? ownerEl.value : '';
      const ctrlUrl = (document.getElementById("controller_url").value || '').trim();
      const tokenKey = ctrlUrl ? `goc_owner_token::${ctrlUrl}` : 'goc_owner_token';
      if (ownerToken) {
        localStorage.setItem(tokenKey, ownerToken);
      } else {
        localStorage.removeItem(tokenKey);
      }
      const tpcEl = document.getElementById("trusted_peer_certs");
      const body = {
        port: parseInt(document.getElementById("port").value, 10),
        mtu: parseInt(document.getElementById("mtu").value, 10),
        log_level: document.getElementById("log_level").value,
        language: document.getElementById("language").value,
        autostart: document.getElementById("autostart").checked,
        controller_url: document.getElementById("controller_url").value,
        relay_urls: document.getElementById("relay_urls").value
          .split(",")
          .map((s) => s.trim())
          .filter(Boolean),
        udp_port: parseInt(document.getElementById("udp_port").value, 10),
        peers: document.getElementById("peers").value
          .split(",")
          .map((s) => s.trim())
          .filter(Boolean),
        stun_servers: document.getElementById("stun_servers").value
          .split(",")
          .map((s) => s.trim())
          .filter(Boolean),
        trusted_peer_certs: ((tpcEl && tpcEl.value) ? tpcEl.value : "")
          .split(/\n{2,}/)
          .map((s) => s.trim())
          .filter(Boolean),
      };
      try {
        await put("/api/settings", body);
        await loadI18n();
        await loadStatus();
        await loadNetworks();
        await loadSettings();
      } catch (err) {
        console.error("settings", err);
        alert(`Settings update failed: ${err.message || err}`);
      }
    });
  }
  const netForm = document.getElementById("network-form");
  if (netForm) {
    netForm.addEventListener("submit", async (e) => {
      e.preventDefault();
      const payload = {
        id: document.getElementById("network-id").value.trim(),
        name: document.getElementById("network-name").value.trim(),
        description: document.getElementById("network-description").value.trim(),
        join_secret: document.getElementById("network-secret").value.trim(),
      };
      if (!payload.id) {
        return;
      }
      await joinNetwork(payload);
      netForm.reset();
    });
  }
}

async function post(path) {
  await postJSON(path, {});
}

async function postJSON(path, body) {
  const res = await fetch(path, {
    method: "POST",
    credentials: "same-origin",
    headers: { "X-CSRF-Token": CSRF, "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  if (!res.ok) {
    let msg = "";
    const text = await res.text();
    if (text) {
      try {
        const data = JSON.parse(text);
        msg = data.message || data.error || text;
      } catch (_) {
        msg = text;
      }
    }
    throw new Error(`${res.status} ${res.statusText}${msg ? `: ${msg}` : ""}`);
  }
  return res;
}

async function put(path, body) {
  const res = await fetch(path, {
    method: "PUT",
    credentials: "same-origin",
    headers: { "X-CSRF-Token": CSRF, "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(`${res.status} ${res.statusText}${text ? `: ${text}` : ""}`);
  }
  return res;
}

function connectSSE() {
  const logEl = document.getElementById("log-stream");
  const es = new EventSource("/api/logs/stream");
  es.onmessage = (e) => {
    const line = `[${new Date().toLocaleTimeString()}] ${e.data}`;
    logEl.textContent += `${line}\n`;
    logEl.scrollTop = logEl.scrollHeight;
  };
}

document.addEventListener("DOMContentLoaded", init);

// --- Chat logic ---
let chatEventSource = null;

function connectChatStream(networkId) {
  if (!networkId) return;
  if (chatEventSource) {
    chatEventSource.close();
  }
  chatEventSource = new EventSource(`/api/v1/networks/${networkId}/chat/stream`);
  const messagesEl = document.getElementById('chat-messages');
  const statusEl = document.getElementById('chat-status');
  if (statusEl) statusEl.textContent = t('chat.connecting') || 'Connecting...';
  chatEventSource.onopen = () => { if (statusEl) statusEl.textContent = t('chat.connected') || 'Connected'; };
  chatEventSource.onerror = () => { if (statusEl) statusEl.textContent = t('chat.disconnected') || 'Disconnected'; };
  chatEventSource.onmessage = (e) => {
    try {
      const msg = JSON.parse(e.data);
      appendChatMessage(msg);
    } catch (err) {
      // fallback plain line
      appendChatMessage({From:'', Text:e.data, At:new Date().toISOString()});
    }
  };
}

function appendChatMessage(msg) {
  const messagesEl = document.getElementById('chat-messages');
  if (!messagesEl) return;
  const time = msg.At ? new Date(msg.At).toLocaleTimeString() : '';
  const line = document.createElement('div');
  line.textContent = `[${time}] ${msg.From||''}: ${msg.Text||''}`;
  messagesEl.appendChild(line);
  messagesEl.scrollTop = messagesEl.scrollHeight;
}

async function refreshChatMessages(networkId) {
  try {
    const res = await fetch(`/api/v1/networks/${networkId}/chat/messages`);
    if (!res.ok) return;
    const data = await res.json();
    const messagesEl = document.getElementById('chat-messages');
    messagesEl.innerHTML='';
    (data.messages||[]).forEach(m=>appendChatMessage(m));
  } catch (e) {
    console.error('chat refresh', e);
  }
}

document.addEventListener('change', (e) => {
  if (e.target && e.target.id === 'chat-network-select') {
    const nid = e.target.value;
    connectChatStream(nid);
    refreshChatMessages(nid);
  }
});

document.addEventListener('submit', async (e) => {
  if (e.target && e.target.id === 'chat-form') {
    e.preventDefault();
    const select = document.getElementById('chat-network-select');
    const nid = select && select.value;
    if (!nid) return;
    const input = document.getElementById('chat-input');
    const text = input.value.trim();
    if (!text) return;
    try {
      const res = await fetch(`/api/v1/networks/${nid}/chat/messages`, {method:'POST', credentials:'same-origin', headers:{'X-CSRF-Token': CSRF, 'Content-Type':'application/json'}, body: JSON.stringify({text})});
      if (res.ok) {
        input.value='';
      } else {
        const statusEl = document.getElementById('chat-status');
        if (statusEl) statusEl.textContent = t('chat.sendFailed') || 'Send failed';
      }
    } catch (err) {
      console.error('chat send', err);
    }
  }
});











