let I18N = {};
let CSRF = "";

async function init() {
  bindSidebarNav();
  await loadI18n();
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
      document.querySelectorAll(".sidebar .navlink").forEach((a) => a.classList.remove("active"));
      link.classList.add("active");
      const target = link.dataset.section;
      document.querySelectorAll(".section").forEach((s) => s.classList.remove("active"));
      document.getElementById(`section-${target}`).classList.add("active");
      document.getElementById("section-title").innerText = t(`nav.${target}`);
    });
  });
}

async function loadI18n() {
  const lang = window._goc_lang || "en";
  const res = await fetch(`/i18n/${lang}.json`, { credentials: "same-origin" });
  I18N = await res.json();
  applyTranslations();
  document.getElementById("app-title").innerText = t("app.title");
}

async function loadStatus() {
  try {
    const res = await fetch("/api/status", { credentials: "same-origin" });
    CSRF = getCookie("goc_csrf") || CSRF;
    const data = await res.json();
    window._goc_lang = data.i18n;
    setBadge("svc-state", data.service_state);
    setBadge("tun-state", data.tun_state);
    setBadge("ctrl-state", data.controller);
    updatePublicEndpoint(data.public_endpoint);
    updateTunSelf(data.tun_error);
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

async function loadNetworks() {
  try {
    const res = await fetch("/api/networks");
    const data = await res.json();
    const networks = Array.isArray(data.networks) ? data.networks : [];
    const ul = document.getElementById("networks-list");
    ul.innerHTML = "";
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
      if (address) {
        textParts.push(address);
      }
      info.textContent = textParts.join(" ");
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
      ul.appendChild(li);
    });
  } catch (err) {
    console.error("networks", err);
  }
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
  const form = document.getElementById("settings-form");
  if (form) {
    form.addEventListener("submit", async (e) => {
      e.preventDefault();
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
      };
      try {
        await put("/api/settings", body);
        await loadI18n();
        await loadStatus();
        await loadNetworks();
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
  const res = await fetch(path, {\n    method: "POST",\n    credentials: "same-origin",\n    headers: { "X-CSRF-Token": CSRF, "Content-Type": "application/json" },\n    body: JSON.stringify(body),\n  });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(`${res.status} ${res.statusText}${text ? `: ${text}` : ""}`);
  }
  return res;
}

async function put(path, body) {
  const res = await fetch(path, {\n    method: "PUT",\n    credentials: "same-origin",\n    headers: { "X-CSRF-Token": CSRF, "Content-Type": "application/json" },\n    body: JSON.stringify(body),\n  });
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

function getCookie(name) {
  const m = document.cookie.match(new RegExp(`(^| )${name}=([^;]+)`));
  if (m) return m[2];
}

document.addEventListener("DOMContentLoaded", init);

