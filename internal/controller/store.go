package controller

import (
	"encoding/json"
	"os"
	"sync"
)

type Network struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Description     string `json:"description"`
	JoinSecret      string `json:"joinSecret,omitempty"`
	AllowChat       bool   `json:"allowChat"`
	OwnerToken      string `json:"ownerToken,omitempty"`
	Visible         bool   `json:"visible"`
	RequireApproval bool   `json:"requireApproval"`
}

type Member struct {
	NodeID      string `json:"nodeId"`
	Nickname    string `json:"nickname"`
	IP          string `json:"ip"`
	LastSeen    int64  `json:"lastSeen"`
	ChatEnabled bool   `json:"chatEnabled"`
}

type ChatMessage struct {
	Timestamp int64  `json:"timestamp"`
	Nickname  string `json:"nickname"`
	Message   string `json:"message"`
}

type State struct {
	Networks map[string]*Network                `json:"networks"`
	Members  map[string]map[string]*Member      `json:"members"`  // networkID -> nodeID -> Member
	Chats    map[string][]*ChatMessage          `json:"chats"`    // networkID -> messages
	Bans     map[string]map[string]bool         `json:"bans"`     // networkID -> nodeID -> banned
	Requests map[string]map[string]*JoinRequest `json:"requests"` // networkID -> requestID -> request
}

// JoinRequest represents a pending approval to join a network.
type JoinRequest struct {
	ID        string `json:"id"`
	Nickname  string `json:"nickname"`
	CreatedAt int64  `json:"createdAt"`
}

type Store struct {
	mu    sync.Mutex
	file  string
	state *State
}

func NewStore(file string) *Store {
	return &Store{
		file: file,
		state: &State{
			Networks: map[string]*Network{},
			Members:  map[string]map[string]*Member{},
			Chats:    map[string][]*ChatMessage{},
			Bans:     map[string]map[string]bool{},
			Requests: map[string]map[string]*JoinRequest{},
		},
	}
}

func (s *Store) Load() error {
	f, err := os.Open(s.file)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewDecoder(f).Decode(s.state)
}

func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := os.Create(s.file)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(s.state)
}

func (s *Store) State() *State {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}
