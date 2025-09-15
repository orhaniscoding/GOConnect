package core

import (
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
}

type Peer struct {
    ID        string
    Relay     bool
    P2P       bool
    RTTms     int
    LastSeen  time.Time
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
}

type State struct {
    mu            sync.RWMutex
    serviceState  ServiceState
    tunUp         bool
    controllerUp  bool
    networks      []Network
    peers         []Peer
    settings      Settings
    tunDev        gtun.Device
}

func NewState(initial Settings) *State {
    return &State{serviceState: StateStopped, settings: initial}
}

func (s *State) Snapshot() (ServiceState, bool, bool, []Network, []Peer, Settings) {
    s.mu.RLock(); defer s.mu.RUnlock()
    return s.serviceState, s.tunUp, s.controllerUp, append([]Network(nil), s.networks...), append([]Peer(nil), s.peers...), s.settings
}

func (s *State) SetSettings(cfg Settings) { s.mu.Lock(); s.settings = cfg; s.mu.Unlock() }

func (s *State) SetTunDevice(d gtun.Device) { s.mu.Lock(); s.tunDev = d; s.mu.Unlock() }

func (s *State) Start() {
    s.mu.Lock()
    s.serviceState = StateRunning
    // Bring up TUN if available
    if s.tunDev != nil {
        _ = s.tunDev.Up()
        s.tunUp = s.tunDev.IsUp()
    } else {
        s.tunUp = false
    }
    s.controllerUp = false // controller stub
    s.mu.Unlock()
}

func (s *State) Stop() {
    s.mu.Lock()
    s.serviceState = StateStopped
    if s.tunDev != nil { _ = s.tunDev.Down() }
    s.tunUp = false
    s.controllerUp = false
    s.mu.Unlock()
}

func (s *State) Restart() { s.Stop(); s.Start() }

func (s *State) SetNetworks(n []Network) { s.mu.Lock(); s.networks = n; s.mu.Unlock() }
func (s *State) SetPeers(p []Peer)       { s.mu.Lock(); s.peers = p; s.mu.Unlock() }
