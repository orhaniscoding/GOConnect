package controller

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
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
	store *Store
	seq   atomic.Uint64
	token string
}

func NewHandler(store *Store) *Handler {
	token := ""
	data, err := ioutil.ReadFile("secrets/controller_token.txt")
	if err == nil {
		token = strings.TrimSpace(string(data))
	}
	return &Handler{store: store, token: token}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
	default:
		h.notFound(w)
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
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(struct {
		Members []*Member      `json:"members"`
		Chats   []*ChatMessage `json:"chats"`
	}{members, chats})
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
		nets = append(nets, n)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(nets)
}

func (h *Handler) handleCreateNetwork(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		JoinSecret  string `json:"joinSecret"`
		AllowChat   bool   `json:"allowChat"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	id := h.nextID()
	net := &Network{
		ID:          id,
		Name:        req.Name,
		Description: req.Description,
		JoinSecret:  req.JoinSecret,
		AllowChat:   req.AllowChat,
	}
	h.store.State().Networks[id] = net
	_ = h.store.Save()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(net)
}

func (h *Handler) notFound(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNotFound)
	_, _ = w.Write([]byte("not found"))
}

func (h *Handler) nextID() string {
	return time.Now().Format("20060102150405") + "-" + strconv.FormatUint(h.seq.Add(1), 10)
}
