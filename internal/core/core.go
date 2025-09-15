package core

import (
	"context"
	"errors"
	"sync"
	"time"

	gtun "goconnect/internal/tun"
)

type ServiceState string

const (
	StateStopped ServiceState = "stopped"
	StateRunning ServiceState = "running"
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
	Port          int
	MTU           int
	LogLevel      string
	Language      string
	Autostart     bool
	ControllerURL string
	RelayURLs     []string
	UDPPort       int
	Peers         []string
	StunServers   []string
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
	s.serviceState = StateRunning
	dev := s.tunDev
	joinedNetworks := hasJoinedNetwork(s.networks)
	s.mu.Unlock()

	tunUp := false
	var tunErr error
	if dev != nil {
		if err := dev.Up(); err != nil {
			tunErr = err
		} else {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			tunErr = dev.LoopbackTest(ctx)
			cancel()
			tunUp = tunErr == nil && dev.IsUp()
		}
	} else {
		tunErr = errors.New("no TUN device configured")
	}

	s.mu.Lock()
	s.tunUp = tunUp
	if tunErr != nil {
		s.tunErr = tunErr.Error()
	} else {
		s.tunErr = ""
	}
	s.controllerUp = tunUp && joinedNetworks
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

func hasJoinedNetwork(n []Network) bool {
	for _, net := range n {
		if net.Joined {
			return true
		}
	}
	return false
}
