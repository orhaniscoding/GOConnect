package core

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	gtun "goconnect/internal/tun"
)

type ServiceState string

const (
	StateStopped  ServiceState = "stopped"
	StateRunning  ServiceState = "running"
	StateDegraded ServiceState = "degraded"
	StateError    ServiceState = "error"
)

type Network struct {
	ID          string
	Name        string
	Description string
	Joined      bool
	Address     string
}

type Peer struct {
	ID       string
	Relay    bool
	P2P      bool
	RTTms    int
	LastSeen time.Time
}

type Settings struct {
	Port             int
	MTU              int
	LogLevel         string
	Language         string
	Autostart        bool
	ControllerURL    string
	RelayURLs        []string
	UDPPort          int
	Peers            []string
	StunServers      []string
	TrustedPeerCerts []string
}

type State struct {
	mu             sync.RWMutex
	serviceState   ServiceState
	tunUp          bool
	tunErr         string
	controllerUp   bool
	networks       []Network
	peers          []Peer
	settings       Settings
	publicEndpoint string
	tunDev         gtun.Device
}

func NewState(initial Settings) *State {
	return &State{serviceState: StateStopped, settings: initial}
}

func (s *State) Snapshot() (ServiceState, bool, bool, []Network, []Peer, Settings) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	nets := append([]Network(nil), s.networks...)
	peers := append([]Peer(nil), s.peers...)
	return s.serviceState, s.tunUp, s.controllerUp, nets, peers, s.settings
}

func (s *State) SetSettings(cfg Settings) {
	s.mu.Lock()
	s.settings = cfg
	s.mu.Unlock()
}

func (s *State) SetTunDevice(d gtun.Device) {
	s.mu.Lock()
	s.tunDev = d
	s.mu.Unlock()
}

func (s *State) Start() {
	s.mu.Lock()
	if s.serviceState == StateRunning {
		s.mu.Unlock()
		return
	}
	s.serviceState = StateRunning // Optimistic state
	s.tunErr = ""
	dev := s.tunDev
	joined := hasJoinedNetwork(s.networks)
	s.mu.Unlock()

	tunUp := false
	var tunErr error
	if dev != nil {
		if err := dev.Up(); err != nil {
			tunErr = err
		} else {
			tunUp = true
			if joined {
				// Give the interface a moment to be ready
				time.Sleep(100 * time.Millisecond)
				if err := dev.LoopbackTest(context.Background()); err != nil {
					tunErr = fmt.Errorf("loopback test failed: %w", err)
				}
			}
		}
	} else {
		tunErr = errors.New("device not available")
	}

	s.mu.Lock()
	s.tunUp = tunUp
	if tunErr != nil {
		s.tunErr = tunErr.Error()
		s.serviceState = StateDegraded // Downgrade state on error
	}
	s.controllerUp = tunUp && joined
	s.mu.Unlock()
}

func (s *State) Stop() {
	s.mu.Lock()
	s.serviceState = StateStopped
	dev := s.tunDev
	s.mu.Unlock()

	if dev != nil {
		_ = dev.Down()
	}

	s.mu.Lock()
	s.tunUp = false
	s.tunErr = ""
	s.controllerUp = false
	s.publicEndpoint = ""
	s.mu.Unlock()
}

func (s *State) Restart() { s.Stop(); s.Start() }

func (s *State) SetNetworks(n []Network) {
	s.mu.Lock()
	s.networks = append([]Network(nil), n...)
	s.controllerUp = s.tunUp && hasJoinedNetwork(s.networks)
	s.mu.Unlock()
}

func (s *State) SetPeers(p []Peer) {
	s.mu.Lock()
	s.peers = append([]Peer(nil), p...)
	s.mu.Unlock()
}

func (s *State) SetControllerUp(ok bool) {
	s.mu.Lock()
	s.controllerUp = ok
	s.mu.Unlock()
}

func (s *State) SetPublicEndpoint(ep string) {
	s.mu.Lock()
	s.publicEndpoint = ep
	s.controllerUp = s.tunUp && hasJoinedNetwork(s.networks)
	s.mu.Unlock()
}

func (s *State) PublicEndpoint() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.publicEndpoint
}

func (s *State) TunError() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.tunErr
}

// Tray status tracking removed.

func hasJoinedNetwork(n []Network) bool {
	for _, net := range n {
		if net.Joined {
			return true
		}
	}
	return false
}
