// UI panel for member preferences (My Preferences)
// This is a stub for integration into the main app.js logic.
export function renderMemberPreferencesPanel(prefs, onSave) {
  const panel = document.createElement('div');
  panel.className = 'card member-preferences-panel';
  panel.innerHTML = `
    <h3>My Preferences</h3>
    <div class="meta">Version: <span class="mp-version">${prefs.version || prefs.Version || 0}</span></div>
    <form id="member-preferences-form" class="form-grid">
      <label><span>Local Share Enabled</span> <input type="checkbox" id="local_share_enabled" ${prefs.LocalShareEnabled ? 'checked' : ''}></label>
      <label><span>Advertise Services</span> <input type="checkbox" id="advertise_services" ${prefs.AdvertiseServices ? 'checked' : ''}></label>
      <label><span>Allow Incoming P2P</span> <input type="checkbox" id="allow_incoming_p2p" ${prefs.AllowIncomingP2P ? 'checked' : ''}></label>
    <label><span>Chat Enabled</span> <input type="checkbox" id="chat_enabled" ${prefs.ChatEnabled ? 'checked' : ''} ${controllerMode ? 'disabled' : ''}></label>
      <label><span>Alias</span> <input type="text" id="alias" value="${prefs.Alias||''}"></label>
      <label><span>Notes</span> <input type="text" id="notes" value="${prefs.Notes||''}"></label>
      <div class="row"><button type="submit">Save</button></div>
    </form>
  `;
  panel.querySelector('#member-preferences-form').onsubmit = (e) => {
    e.preventDefault();
    const updated = {
      Version: prefs.version || prefs.Version || 0,
      LocalShareEnabled: panel.querySelector('#local_share_enabled').checked,
      AdvertiseServices: panel.querySelector('#advertise_services').checked,
      AllowIncomingP2P: panel.querySelector('#allow_incoming_p2p').checked,
  ChatEnabled: panel.querySelector('#chat_enabled').checked,
      Alias: panel.querySelector('#alias').value,
      Notes: panel.querySelector('#notes').value,
    };
    onSave(updated);
  };
  return panel;
}
