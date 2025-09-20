package controller

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"goconnect/internal/config"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

// Her node'a benzersiz /32 IP ataması yapan basit fonksiyon
func allocateIP(networkID, nodeID string, s *State) string {
	// 100.NETID.0.HOSTID/32
	// NETID: networkID'nin son 2 hanesinin hex değeri
	// HOSTID: nodeID'nin son 2 hanesinin hex değeri
	netPart := byte(64)
	if len(networkID) > 1 {
		fmt.Sscanf(networkID[len(networkID)-2:], "%x", &netPart)
	}
	hostPart := byte(2)
	if len(nodeID) > 1 {
		fmt.Sscanf(nodeID[len(nodeID)-2:], "%x", &hostPart)
	}
	ip := fmt.Sprintf("100.%d.0.%d/32", netPart, hostPart)
	// Çakışma kontrolü
	for _, m := range s.Members[networkID] {
		if m.IP == ip {
			hostPart++
			ip = fmt.Sprintf("100.%d.0.%d/32", netPart, hostPart)
		}
	}
	return ip
}

type Handler struct {
	store     *Store
	seq       atomic.Uint64
	token     string
	adminPass string
}

func NewHandler(store *Store) *Handler {
	h := &Handler{store: store}
	h.adminPass = strings.TrimSpace(os.Getenv("CONTROLLER_ADMIN_PASSWORD"))
	// Load token from ProgramData secrets path; fallback to relative path
	h.token = readToken()
	if strings.TrimSpace(h.token) == "" {
		// Auto-generate on first run
		t := genToken()
		_ = writeToken(t)
		h.token = t
	}
	return h
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Admin endpoints: protected by admin password or loopback-only
	if strings.HasPrefix(r.URL.Path, "/admin/") {
		// Enforce loopback-only for all /admin/*
		if !isLoopbackRequest(r) {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte("forbidden"))
			return
		}
		// If a password is configured, also require Basic Auth locally
		if h.adminPass != "" {
			u, p, ok := r.BasicAuth()
			if !ok || u != "admin" || p != h.adminPass {
				w.Header().Set("WWW-Authenticate", "Basic realm=controller")
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte("unauthorized"))
				return
			}
		}
		h.handleAdmin(w, r)
		return
	}
	// Bearer token auth for all endpoints
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") || strings.TrimPrefix(auth, "Bearer ") != h.token {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("unauthorized"))
		return
	}
	switch {
	case r.Method == "GET" && r.URL.Path == "/api/controller/networks":
		h.handleListNetworks(w, r)
	case r.Method == "POST" && r.URL.Path == "/api/controller/networks":
		h.handleCreateNetwork(w, r)
	case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/api/controller/networks/") &&
		!strings.HasSuffix(r.URL.Path, "/join") && !strings.HasSuffix(r.URL.Path, "/snapshot") && !strings.HasSuffix(r.URL.Path, "/chat"):
		h.handleDeleteNetwork(w, r)
	// Ağlara katılma
	// /api/controller/networks/{id}/join
	case r.Method == "POST" && len(r.URL.Path) > 28 && r.URL.Path[:28] == "/api/controller/networks/" && r.URL.Path[len(r.URL.Path)-5:] == "/join":
		h.handleJoinNetwork(w, r)
	// Snapshot (üyeler + chat)
	// /api/controller/networks/{id}/snapshot
	case r.Method == "GET" && len(r.URL.Path) > 32 && r.URL.Path[:28] == "/api/controller/networks/" && r.URL.Path[len(r.URL.Path)-9:] == "/snapshot":
		h.handleSnapshot(w, r)
	// Chat gönderme
	// /api/controller/networks/{id}/chat
	case r.Method == "POST" && len(r.URL.Path) > 28 && r.URL.Path[:28] == "/api/controller/networks/" && r.URL.Path[len(r.URL.Path)-5:] == "/chat":
		h.handlePostChat(w, r)
	case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/admin/visibility"):
		h.handleSetVisibility(w, r)
	case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/admin/secret"):
		h.handleRotateSecret(w, r)
	case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/admin/kick"):
		h.handleKick(w, r)
	case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/admin/ban"):
		h.handleBan(w, r)
	case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/admin/unban"):
		h.handleUnban(w, r)
	case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/admin/approve"):
		h.handleApprove(w, r)
	case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/admin/reject"):
		h.handleReject(w, r)
	default:
		h.notFound(w)
	}
}

// --- Admin helpers ---
func isLoopbackRequest(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func (h *Handler) adminAuthorized(r *http.Request) bool {
	// If adminPass set, require Basic Auth admin:<pass> from anywhere
	if h.adminPass != "" {
		u, p, ok := r.BasicAuth()
		return ok && u == "admin" && p == h.adminPass
	}
	// Otherwise allow only loopback
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func genToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func secretsPath() string {
	_, _, secrets := config.Paths()
	return secrets
}

func tokenFilePath() string {
	return filepath.Join(secretsPath(), "controller_token.txt")
}

func readToken() string {
	p := tokenFilePath()
	if b, err := os.ReadFile(p); err == nil {
		return strings.TrimSpace(string(b))
	}
	// fallback to legacy relative path
	if b, err := ioutil.ReadFile("secrets/controller_token.txt"); err == nil {
		return strings.TrimSpace(string(b))
	}
	return ""
}

func writeToken(tok string) error {
	if err := os.MkdirAll(secretsPath(), 0o755); err != nil {
		return err
	}
	return os.WriteFile(tokenFilePath(), []byte(strings.TrimSpace(tok)), 0o600)
}

func clearToken() {
	_ = os.Remove(tokenFilePath())
}

func (h *Handler) handleAdmin(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/admin/token":
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		status := "(set)"
		if strings.TrimSpace(h.token) == "" {
			status = "(not set)"
		}
		// Show token value inline with a copy button; only loopback can access this page.
		// Security: page is loopback-only (and optionally Basic Auth) enforced in ServeHTTP.
		tok := h.token
		if strings.TrimSpace(tok) == "" {
			tok = ""
		}
		fmt.Fprintf(w, `<!doctype html><html><head><meta charset="utf-8"><title>GOConnect Controller Admin</title>
		<style>body{font-family:Segoe UI,Arial,sans-serif;background:#0e1116;color:#e6e6e6;padding:20px} .row{display:flex;gap:8px;align-items:center;margin:8px 0} input{background:#0b0d12;border:1px solid #1f2430;color:#e6e6e6;border-radius:6px;padding:8px;min-width:360px} button{background:#263043;border:1px solid #33425b;color:#e6e6e6;border-radius:6px;padding:8px 12px;cursor:pointer}</style>
		<script>async function copy(){const el=document.getElementById('token'); el.select(); el.setSelectionRange(0,99999); try{await navigator.clipboard.writeText(el.value);}catch(e){document.execCommand('copy');} const info=document.getElementById('copied'); if(info){info.textContent='Copied'; setTimeout(()=>info.textContent='',1500);} }</script>
		</head><body>
		<h2>Controller Token</h2>
		<p>Status: %s</p>
		<div class="row"><input id="token" type="text" value="%s" placeholder="(empty)"><button type="button" onclick="copy()">Copy</button><span id="copied" style="font-size:12px;color:#9aa4b5"></span></div>
		<form method="post" action="/admin/token/regenerate"><button type="submit">Regenerate</button></form>
		<form method="post" action="/admin/token/clear" onsubmit="return confirm('Clear token? Agents will be blocked until updated.');"><button type="submit">Clear</button></form>
		</body></html>`, status, tok)
		return
	case r.Method == http.MethodGet && r.URL.Path == "/admin/token/status":
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"set": strings.TrimSpace(h.token) != ""})
		return
	case r.Method == http.MethodPost && r.URL.Path == "/admin/token/regenerate":
		t := genToken()
		if err := writeToken(t); err != nil {
			http.Error(w, "write failed", 500)
			return
		}
		h.token = t
		http.Redirect(w, r, "/admin/token", http.StatusSeeOther)
		return
	case r.Method == http.MethodPost && r.URL.Path == "/admin/token/clear":
		clearToken()
		h.token = ""
		http.Redirect(w, r, "/admin/token", http.StatusSeeOther)
		return
	default:
		http.NotFound(w, r)
		return
	}
}

// Ağlara katılma endpointi
func (h *Handler) handleJoinNetwork(w http.ResponseWriter, r *http.Request) {
	id := extractID(r.URL.Path, "/join")
	if id == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	var req struct {
		Nickname    string `json:"nickname"`
		JoinSecret  string `json:"joinSecret"`
		ChatEnabled bool   `json:"chatEnabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	s := h.store.State()
	netw := s.Networks[id]
	if netw == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if netw.JoinSecret != "" && netw.JoinSecret != req.JoinSecret {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	if netw.RequireApproval {
		s := h.store.State()
		if s.Requests[id] == nil {
			s.Requests[id] = map[string]*JoinRequest{}
		}
		rid := h.nextID()
		s.Requests[id][rid] = &JoinRequest{ID: rid, Nickname: req.Nickname, CreatedAt: time.Now().Unix()}
		_ = h.store.Save()
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("pending"))
		return
	}
	if s.Members[id] == nil {
		s.Members[id] = map[string]*Member{}
	}
	nodeID := h.nextID()
	ip := allocateIP(id, nodeID, s)
	member := &Member{
		NodeID:      nodeID,
		Nickname:    req.Nickname,
		IP:          ip,
		LastSeen:    time.Now().Unix(),
		ChatEnabled: req.ChatEnabled,
	}
	s.Members[id][nodeID] = member
	_ = h.store.Save()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(member)
}

// Snapshot endpointi (üyeler + chat)
func (h *Handler) handleSnapshot(w http.ResponseWriter, r *http.Request) {
	id := extractID(r.URL.Path, "/snapshot")
	if id == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	s := h.store.State()
	members := []*Member{}
	for _, m := range s.Members[id] {
		members = append(members, m)
	}
	// Chat için since parametresi
	since := int64(0)
	if v := r.URL.Query().Get("since"); v != "" {
		if t, err := parseInt64(v); err == nil {
			since = t
		}
	}
	chats := []*ChatMessage{}
	for _, msg := range s.Chats[id] {
		if msg.Timestamp > since {
			chats = append(chats, msg)
		}
	}
	// Pending requests (for owner UI)
	reqs := []*JoinRequest{}
	for _, jr := range s.Requests[id] {
		reqs = append(reqs, jr)
	}
	// Bans list (node IDs)
	bans := []string{}
	for nodeID, banned := range s.Bans[id] {
		if banned {
			bans = append(bans, nodeID)
		}
	}
	// Network flags for UI badges
	netw := s.Networks[id]
	vis := false
	reqAppr := false
	if netw != nil {
		vis = netw.Visible
		reqAppr = netw.RequireApproval
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(struct {
		Members         []*Member      `json:"members"`
		Chats           []*ChatMessage `json:"chats"`
		Requests        []*JoinRequest `json:"requests"`
		Bans            []string       `json:"bans"`
		Visible         bool           `json:"visible"`
		RequireApproval bool           `json:"requireApproval"`
	}{members, chats, reqs, bans, vis, reqAppr})
}

// Chat gönderme endpointi
func (h *Handler) handlePostChat(w http.ResponseWriter, r *http.Request) {
	id := extractID(r.URL.Path, "/chat")
	if id == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	var req struct {
		Nickname string `json:"nickname"`
		Message  string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	s := h.store.State()
	if s.Chats[id] == nil {
		s.Chats[id] = []*ChatMessage{}
	}
	msg := &ChatMessage{
		Timestamp: time.Now().Unix(),
		Nickname:  req.Nickname,
		Message:   req.Message,
	}
	s.Chats[id] = append(s.Chats[id], msg)
	// Son 200 mesajı tut
	if len(s.Chats[id]) > 200 {
		s.Chats[id] = s.Chats[id][len(s.Chats[id])-200:]
	}
	_ = h.store.Save()
	w.WriteHeader(http.StatusNoContent)
}

// Yardımcılar
func extractID(path, suffix string) string {
	// /api/controller/networks/{id}/suffix
	parts := len("/api/controller/networks/")
	end := len(path) - len(suffix)
	if end <= parts {
		return ""
	}
	return path[parts:end]
}

func parseInt64(s string) (int64, error) {
	var i int64
	_, err := fmt.Sscan(s, &i)
	return i, err
}

func (h *Handler) handleListNetworks(w http.ResponseWriter, r *http.Request) {
	nets := []*Network{}
	for _, n := range h.store.State().Networks {
		if !n.Visible {
			continue
		}
		nets = append(nets, n)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(nets)
}

func (h *Handler) handleCreateNetwork(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name            string `json:"name"`
		Description     string `json:"description"`
		JoinSecret      string `json:"joinSecret"`
		AllowChat       bool   `json:"allowChat"`
		Visible         bool   `json:"visible"`
		RequireApproval bool   `json:"requireApproval"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	id := h.nextID()
	net := &Network{
		ID:              id,
		Name:            req.Name,
		Description:     req.Description,
		JoinSecret:      req.JoinSecret,
		AllowChat:       req.AllowChat,
		Visible:         req.Visible,
		RequireApproval: req.RequireApproval,
	}
	// Set owner from Authorization token
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		net.OwnerToken = strings.TrimPrefix(auth, "Bearer ")
	}
	// Owner token from header (preferred)
	if v := r.Header.Get("X-Owner-Token"); strings.TrimSpace(v) != "" {
		net.OwnerToken = strings.TrimSpace(v)
	}
	h.store.State().Networks[id] = net
	_ = h.store.Save()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(net)
}

// --- Admin & helper methods ---
func (h *Handler) isOwner(r *http.Request, n *Network) bool {
	v := strings.TrimSpace(r.Header.Get("X-Owner-Token"))
	return n.OwnerToken != "" && v == n.OwnerToken
}

func (h *Handler) handleDeleteNetwork(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/controller/networks/")
	s := h.store.State()
	net := s.Networks[id]
	if net == nil {
		h.notFound(w)
		return
	}
	if !h.isOwner(r, net) {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	delete(s.Networks, id)
	delete(s.Members, id)
	delete(s.Chats, id)
	delete(s.Bans, id)
	delete(s.Requests, id)
	_ = h.store.Save()
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) handleSetVisibility(w http.ResponseWriter, r *http.Request) {
	id := extractAdminID(r.URL.Path, "/admin/visibility")
	s := h.store.State()
	net := s.Networks[id]
	if net == nil {
		h.notFound(w)
		return
	}
	if !h.isOwner(r, net) {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	var req struct {
		Visible bool `json:"visible"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(400)
		return
	}
	net.Visible = req.Visible
	_ = h.store.Save()
	w.WriteHeader(204)
}

func (h *Handler) handleRotateSecret(w http.ResponseWriter, r *http.Request) {
	id := extractAdminID(r.URL.Path, "/admin/secret")
	s := h.store.State()
	net := s.Networks[id]
	if net == nil {
		h.notFound(w)
		return
	}
	if !h.isOwner(r, net) {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	var req struct {
		JoinSecret string `json:"joinSecret"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(400)
		return
	}
	net.JoinSecret = strings.TrimSpace(req.JoinSecret)
	_ = h.store.Save()
	w.WriteHeader(204)
}

func (h *Handler) handleKick(w http.ResponseWriter, r *http.Request) {
	id := extractAdminID(r.URL.Path, "/admin/kick")
	s := h.store.State()
	net := s.Networks[id]
	if net == nil {
		h.notFound(w)
		return
	}
	if !h.isOwner(r, net) {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	var req struct {
		NodeID string `json:"nodeId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.NodeID) == "" {
		w.WriteHeader(400)
		return
	}
	if s.Members[id] != nil {
		delete(s.Members[id], req.NodeID)
	}
	_ = h.store.Save()
	w.WriteHeader(204)
}

func (h *Handler) handleBan(w http.ResponseWriter, r *http.Request) {
	id := extractAdminID(r.URL.Path, "/admin/ban")
	s := h.store.State()
	net := s.Networks[id]
	if net == nil {
		h.notFound(w)
		return
	}
	if !h.isOwner(r, net) {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	var req struct {
		NodeID string `json:"nodeId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.NodeID) == "" {
		w.WriteHeader(400)
		return
	}
	if s.Bans[id] == nil {
		s.Bans[id] = map[string]bool{}
	}
	s.Bans[id][req.NodeID] = true
	if s.Members[id] != nil {
		delete(s.Members[id], req.NodeID)
	}
	_ = h.store.Save()
	w.WriteHeader(204)
}

func (h *Handler) handleUnban(w http.ResponseWriter, r *http.Request) {
	id := extractAdminID(r.URL.Path, "/admin/unban")
	s := h.store.State()
	net := s.Networks[id]
	if net == nil {
		h.notFound(w)
		return
	}
	if !h.isOwner(r, net) {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	var req struct {
		NodeID string `json:"nodeId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.NodeID) == "" {
		w.WriteHeader(400)
		return
	}
	if s.Bans[id] != nil {
		delete(s.Bans[id], req.NodeID)
	}
	_ = h.store.Save()
	w.WriteHeader(204)
}

func (h *Handler) handleApprove(w http.ResponseWriter, r *http.Request) {
	id := extractAdminID(r.URL.Path, "/admin/approve")
	s := h.store.State()
	net := s.Networks[id]
	if net == nil {
		h.notFound(w)
		return
	}
	if !h.isOwner(r, net) {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	var req struct {
		RequestID string `json:"requestId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.RequestID) == "" {
		w.WriteHeader(400)
		return
	}
	jrMap := s.Requests[id]
	if jrMap == nil {
		h.notFound(w)
		return
	}
	jr, ok := jrMap[req.RequestID]
	if !ok {
		h.notFound(w)
		return
	}
	if s.Members[id] == nil {
		s.Members[id] = map[string]*Member{}
	}
	nodeID := h.nextID()
	ip := allocateIP(id, nodeID, s)
	s.Members[id][nodeID] = &Member{NodeID: nodeID, Nickname: jr.Nickname, IP: ip, LastSeen: time.Now().Unix(), ChatEnabled: true}
	delete(s.Requests[id], req.RequestID)
	_ = h.store.Save()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"nodeId": nodeID, "ip": ip})
}

func (h *Handler) handleReject(w http.ResponseWriter, r *http.Request) {
	id := extractAdminID(r.URL.Path, "/admin/reject")
	s := h.store.State()
	net := s.Networks[id]
	if net == nil {
		h.notFound(w)
		return
	}
	if !h.isOwner(r, net) {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	var req struct {
		RequestID string `json:"requestId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.RequestID) == "" {
		w.WriteHeader(400)
		return
	}
	if s.Requests[id] != nil {
		delete(s.Requests[id], req.RequestID)
	}
	_ = h.store.Save()
	w.WriteHeader(204)
}

func extractAdminID(path, suffix string) string {
	base := "/api/controller/networks/"
	if !strings.HasPrefix(path, base) || !strings.HasSuffix(path, suffix) {
		return ""
	}
	mid := strings.TrimSuffix(strings.TrimPrefix(path, base), suffix)
	if strings.HasSuffix(mid, "/admin") {
		mid = strings.TrimSuffix(mid, "/admin")
	}
	mid = strings.TrimSuffix(mid, "/")
	parts := strings.Split(mid, "/")
	if len(parts) >= 1 {
		return parts[0]
	}
	return ""
}

func (h *Handler) notFound(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNotFound)
	_, _ = w.Write([]byte("not found"))
}

func (h *Handler) nextID() string {
	return time.Now().Format("20060102150405") + "-" + strconv.FormatUint(h.seq.Add(1), 10)
}
