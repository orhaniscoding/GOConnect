// UI panel for network owner: settings (allow_all, mode, etc.)
// This is a stub for integration into the main app.js logic.

export function renderNetworkSettingsPanel(settings, onSave) {
  const panel = document.createElement('div');
  panel.className = 'card network-settings-panel';
  const controllerManaged = settings.ControllerManaged || settings.controllerManaged;
  let controllerBadge = '';
  if (controllerManaged) {
    controllerBadge = '<span class="badge controller">Controller Managed</span>';
  }
  panel.innerHTML = `
    <h3>Network Settings ${controllerBadge}</h3>
    <div class="meta">Version: <span class="ns-version">${settings.version || settings.Version || 0}</span></div>
    <form id="network-settings-form" class="form-grid">
      <label><span>Allow All</span> <input type="checkbox" id="allow_all" ${settings.AllowAll ? 'checked' : ''}></label>
      <label><span>Mode</span>
        <select id="mode">
          <option value="default" ${settings.Mode === 'default' ? 'selected' : ''}>Default</option>
          <option value="lan" ${settings.Mode === 'lan' ? 'selected' : ''}>LAN</option>
          <option value="game" ${settings.Mode === 'game' ? 'selected' : ''}>Game</option>
        </select>
      </label>
      <label><span>File Sharing</span> <input type="checkbox" id="allow_file_share" ${settings.AllowFileShare ? 'checked' : ''}></label>
      <label><span>Service Discovery</span> <input type="checkbox" id="allow_service_discovery" ${settings.AllowServiceDisc ? 'checked' : ''}></label>
      <label><span>Peer Ping</span> <input type="checkbox" id="allow_peer_ping" ${settings.AllowPeerPing ? 'checked' : ''}></label>
      <label><span>Relay Fallback</span> <input type="checkbox" id="allow_relay_fallback" ${settings.AllowRelayFallback ? 'checked' : ''}></label>
      <label><span>Broadcast</span> <input type="checkbox" id="allow_broadcast" ${settings.AllowBroadcast ? 'checked' : ''}></label>
      <label><span>IPv6</span> <input type="checkbox" id="allow_ipv6" ${settings.AllowIPv6 ? 'checked' : ''}></label>
      <label><span>Allow Chat</span> <input type="checkbox" id="allow_chat" ${settings.AllowChat ? 'checked' : ''} ${controllerManaged ? 'disabled' : ''}></label>
      <label><span>MTU Override</span> <input type="number" id="mtu_override" value="${settings.MTUOverride || ''}"></label>
      <label><span>Default DNS</span> <input type="text" id="default_dns" value="${(settings.DefaultDNS||[]).join(',')}"></label>
      <label><span>Game Profile</span> <input type="text" id="game_profile" value="${settings.GameProfile||''}"></label>
      <label><span>Require Encryption</span> <input type="checkbox" id="require_encryption" ${settings.RequireEncryption ? 'checked' : ''}></label>
      <label><span>Restrict New Members</span> <input type="checkbox" id="restrict_new_members" ${settings.RestrictNewMembers ? 'checked' : ''}></label>
      <label><span>Idle Disconnect (min)</span> <input type="number" id="idle_disconnect_minutes" value="${settings.IdleDisconnectMin||''}"></label>
      <div class="row"><button type="submit">Save</button></div>
    </form>
  `;
  panel.querySelector('#network-settings-form').onsubmit = (e) => {
    e.preventDefault();
    const updated = {
      Version: settings.version || settings.Version || 0,
      AllowAll: panel.querySelector('#allow_all').checked,
      Mode: panel.querySelector('#mode').value,
      AllowFileShare: panel.querySelector('#allow_file_share').checked,
      AllowServiceDisc: panel.querySelector('#allow_service_discovery').checked,
      AllowPeerPing: panel.querySelector('#allow_peer_ping').checked,
      AllowRelayFallback: panel.querySelector('#allow_relay_fallback').checked,
      AllowBroadcast: panel.querySelector('#allow_broadcast').checked,
      AllowIPv6: panel.querySelector('#allow_ipv6').checked,
      AllowChat: panel.querySelector('#allow_chat').checked,
      MTUOverride: parseInt(panel.querySelector('#mtu_override').value, 10) || 0,
      DefaultDNS: panel.querySelector('#default_dns').value.split(',').map(s=>s.trim()).filter(Boolean),
      GameProfile: panel.querySelector('#game_profile').value,
      RequireEncryption: panel.querySelector('#require_encryption').checked,
      RestrictNewMembers: panel.querySelector('#restrict_new_members').checked,
      IdleDisconnectMin: parseInt(panel.querySelector('#idle_disconnect_minutes').value, 10) || 0,
    };
    onSave(updated);
  };
  return panel;
}
