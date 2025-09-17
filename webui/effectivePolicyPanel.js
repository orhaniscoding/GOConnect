// UI panel for effective policy summary
// This is a stub for integration into the main app.js logic.
export function renderEffectivePolicyPanel(policy) {
  const panel = document.createElement('div');
  panel.className = 'card effective-policy-panel';
  panel.innerHTML = `
    <h3>Effective Policy</h3>
    <ul class="policy-list">
      <li><b>Mode:</b> ${policy.Mode || ''}</li>
      <li><b>Allow All:</b> ${policy.AllowAll ? 'Yes' : 'No'}</li>
      <li><b>File Sharing:</b> ${policy.AllowFileShare ? 'Enabled' : 'Disabled'}</li>
      <li><b>Service Discovery:</b> ${policy.AllowServiceDisc ? 'Enabled' : 'Disabled'}</li>
      <li><b>Peer Ping:</b> ${policy.AllowPeerPing ? 'Enabled' : 'Disabled'}</li>
      <li><b>Relay Fallback:</b> ${policy.AllowRelayFallback ? 'Enabled' : 'Disabled'}</li>
      <li><b>Broadcast:</b> ${policy.AllowBroadcast ? 'Enabled' : 'Disabled'}</li>
      <li><b>IPv6:</b> ${policy.AllowIPv6 ? 'Enabled' : 'Disabled'}</li>
      <li><b>MTU:</b> ${policy.MTU || ''}</li>
      <li><b>Default DNS:</b> ${(policy.DefaultDNS||[]).join(', ')}</li>
      <li><b>Game Profile:</b> ${policy.GameProfile || ''}</li>
      <li><b>Require Encryption:</b> ${policy.RequireEncryption ? 'Yes' : 'No'}</li>
      <li><b>Idle Disconnect (min):</b> ${policy.IdleDisconnectMin || ''}</li>
    </ul>
  `;
  return panel;
}
