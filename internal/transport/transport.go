package transport

// TODO v1.2: Add QUIC (quic-go) skeleton, STUN/ICE (pion) for NAT traversal.

type Transport interface {
    Start() error
    Stop() error
}

type stub struct{}

func New() Transport { return &stub{} }
func (s *stub) Start() error { return nil }
func (s *stub) Stop() error  { return nil }

