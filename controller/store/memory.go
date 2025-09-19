package store

import (
	"goconnect/controller/models"
	"sync"
)

type InMemoryStore struct {
	mu       sync.RWMutex
	settings map[string]*models.NetworkSettings
	prefs    map[string]map[string]*models.MembershipPreferences // networkID -> nodeID -> prefs
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		settings: make(map[string]*models.NetworkSettings),
		prefs:    make(map[string]map[string]*models.MembershipPreferences),
	}
}

func (s *InMemoryStore) GetNetworkSettings(networkID string) (*models.NetworkSettings, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.settings[networkID]
	return v, ok
}

func (s *InMemoryStore) SetNetworkSettings(networkID string, ns *models.NetworkSettings) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.settings[networkID] = ns
}

func (s *InMemoryStore) GetMembershipPreferences(networkID, nodeID string) (*models.MembershipPreferences, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.prefs[networkID] == nil {
		return nil, false
	}
	v, ok := s.prefs[networkID][nodeID]
	return v, ok
}

func (s *InMemoryStore) SetMembershipPreferences(networkID, nodeID string, mp *models.MembershipPreferences) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.prefs[networkID] == nil {
		s.prefs[networkID] = make(map[string]*models.MembershipPreferences)
	}
	s.prefs[networkID][nodeID] = mp
}
