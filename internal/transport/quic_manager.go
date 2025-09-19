package transport

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"encoding/pem"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	quic "github.com/quic-go/quic-go"
)

type PeerInfo struct {
	Address  string
	RTTms    int
	Relay    bool
	P2P      bool
	LastSeen time.Time
}

type Manager struct {
	addr            string
	stunServers     []string
	mu              sync.RWMutex
	peers           map[string]*PeerInfo
	publicEndpoint  string
	natUpdate       func(string)
	ln              *quic.Listener
	stopCh          chan struct{}
	identity        tls.Certificate
	caCert          *x509.Certificate
	clientTLSConfig *tls.Config
	serverTLSConfig *tls.Config
}

const (
	dialInitialBackoff = time.Second
	dialMaxBackoff     = 30 * time.Second
	maxDialFailures    = 20
	tlsServerName      = "goconnect"
)

func NewManager(udpAddr string, stunServers []string, secretsDir string, trusted []string) (*Manager, error) {
	servers := append([]string(nil), stunServers...)
	identity, ca, err := loadOrCreateManagerIdentity(secretsDir)
	if err != nil {
		return nil, err
	}

	serverTLS, clientTLS, err := newTLSConfigs(identity, ca, secretsDir, trusted)
	if err != nil {
		return nil, err
	}

	return &Manager{
		addr:            udpAddr,
		stunServers:     servers,
		peers:           map[string]*PeerInfo{},
		identity:        identity,
		caCert:          ca,
		clientTLSConfig: clientTLS,
		serverTLSConfig: serverTLS,
	}, nil
}

func (m *Manager) Start(peers []string) error {
	if m.ln != nil {
		return nil
	}
	udpAddr, err := net.ResolveUDPAddr("udp", m.addr)
	if err != nil {
		return err
	}
	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return err
	}
	ln, err := quic.Listen(conn, m.serverTLSConfig, &quic.Config{})
	if err != nil {
		return err
	}
	m.ln = ln
	m.stopCh = make(chan struct{})
	go m.acceptLoop()
	if len(peers) > 0 {
		go m.dialLoop(peers)
	}
	if len(m.stunServers) > 0 {
		go m.stunLoop()
	}
	return nil
}

func (m *Manager) Stop() error {
	if m.ln != nil {
		_ = m.ln.Close()
		m.ln = nil
	}
	if m.stopCh != nil {
		close(m.stopCh)
		m.stopCh = nil
	}
	return nil
}

// BoundAddr returns the local UDP address that the QUIC listener is bound to.
// It returns nil if the manager has not started listening yet.
func (m *Manager) BoundAddr() net.Addr {
	if m.ln == nil {
		return nil
	}
	return m.ln.Addr()
}

func (m *Manager) acceptLoop() {
	for {
		sess, err := m.ln.Accept(context.Background())
		if err != nil {
			return
		}
		go func() {
			s := sess
			defer s.CloseWithError(0, "bye")
			m.touchPeer(s.RemoteAddr().String())
			str, err := s.AcceptStream(context.Background())
			if err != nil {
				return
			}
			buf := make([]byte, 1024)
			for {
				n, err := str.Read(buf)
				if err != nil {
					return
				}
				m.touchPeer(s.RemoteAddr().String())
				_, _ = str.Write(buf[:n])
			}
		}()
	}
}

func (m *Manager) dialLoop(peers []string) {
	stop := m.stopCh
	delay := dialInitialBackoff
	failures := 0
	for {
		for _, p := range peers {
			if stop == nil {
				return
			}
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			start := time.Now()
			tlsConf := m.clientTLSConfig.Clone()
			tlsConf.ServerName = tlsServerName
			sess, err := quic.DialAddr(ctx, p, tlsConf, &quic.Config{})
			if err == nil {
				str, err2 := sess.OpenStreamSync(ctx)
				if err2 == nil {
					_, _ = str.Write([]byte("ping"))
					buf := make([]byte, 4)
					_, _ = str.Read(buf)
					rtt := int(time.Since(start).Milliseconds())
					m.updateRTT(p, rtt)
				}
				_ = sess.CloseWithError(0, "done")
				failures = 0
				delay = dialInitialBackoff
			} else {
				failures++
				cancel()
				if failures >= maxDialFailures {
					return
				}
				select {
				case <-stop:
					return
				case <-time.After(delay):
				}
				delay *= 2
				if delay > dialMaxBackoff {
					delay = dialMaxBackoff
				}
				continue
			}
			cancel()
			if stop != nil {
				select {
				case <-stop:
					return
				default:
				}
			}
			select {
			case <-time.After(time.Second):
			case <-stop:
				return
			}
		}
		if stop != nil {
			select {
			case <-stop:
				return
			default:
			}
		}
	}
}

func (m *Manager) stunLoop() {
	ticker := time.NewTicker(45 * time.Second)
	defer ticker.Stop()
	stop := m.stopCh
	for {
		m.probeSTUN()
		select {
		case <-ticker.C:
			continue
		case <-stop:
			return
		}
	}
}

func (m *Manager) probeSTUN() {
	for _, server := range m.stunServers {
		if server == "" {
			continue
		}
		ep, err := queryPublicEndpoint(server)
		if err != nil {
			continue
		}
		m.setPublicEndpoint(ep)
		return
	}
}

func (m *Manager) touchPeer(addr string) {
	m.mu.Lock()
	pi := m.peers[addr]
	if pi == nil {
		pi = &PeerInfo{Address: addr}
		m.peers[addr] = pi
	}
	pi.LastSeen = time.Now()
	m.mu.Unlock()
}

func (m *Manager) updateRTT(addr string, rtt int) {
	m.mu.Lock()
	pi := m.peers[addr]
	if pi == nil {
		pi = &PeerInfo{Address: addr}
		m.peers[addr] = pi
	}
	pi.RTTms = rtt
	pi.P2P = true
	pi.LastSeen = time.Now()
	m.mu.Unlock()
}

func (m *Manager) SnapshotPeers() []PeerInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]PeerInfo, 0, len(m.peers))
	for _, p := range m.peers {
		out = append(out, *p)
	}
	return out
}

func (m *Manager) setPublicEndpoint(ep string) {
	m.mu.Lock()
	if m.publicEndpoint == ep {
		cb := m.natUpdate
		m.mu.Unlock()
		if cb != nil && ep == "" {
			cb(ep)
		}
		return
	}
	m.publicEndpoint = ep
	cb := m.natUpdate
	m.mu.Unlock()
	if cb != nil {
		cb(ep)
	}
}

func (m *Manager) PublicEndpoint() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.publicEndpoint
}

func (m *Manager) SetNATUpdateFn(fn func(string)) {
	m.mu.Lock()
	m.natUpdate = fn
	ep := m.publicEndpoint
	m.mu.Unlock()
	if fn != nil && ep != "" {
		fn(ep)
	}
}

func (m *Manager) GetIdentity() tls.Certificate {
	return m.identity
}

func loadOrCreateManagerIdentity(secretsDir string) (tls.Certificate, *x509.Certificate, error) {
	caCert, caKey, err := loadOrCreateCA(secretsDir)
	if err != nil {
		return tls.Certificate{}, nil, fmt.Errorf("failed to load/create CA: %w", err)
	}

	hostCert, hostKey, err := loadOrCreateHostCert(secretsDir, caCert, caKey)
	if err != nil {
		return tls.Certificate{}, nil, fmt.Errorf("failed to load/create host certificate: %w", err)
	}

	// PEM encode host cert
	hostCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: hostCert.Raw})

	// PEM encode host key
	privBytes, err := x509.MarshalECPrivateKey(hostKey)
	if err != nil {
		return tls.Certificate{}, nil, fmt.Errorf("failed to marshal host key: %w", err)
	}
	hostKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privBytes})

	tlsCert, err := tls.X509KeyPair(hostCertPEM, hostKeyPEM)
	if err != nil {
		return tls.Certificate{}, nil, fmt.Errorf("failed to create tls key pair: %w", err)
	}
	tlsCert.Leaf = hostCert

	return tlsCert, caCert, nil
}

func newTLSConfigs(identity tls.Certificate, caCert *x509.Certificate, secretsDir string, trusted []string) (*tls.Config, *tls.Config, error) {
	certPool := x509.NewCertPool()
	certPool.AddCert(caCert)
	// Merge trusted PEMs
	for _, entry := range trusted {
		e := strings.TrimSpace(entry)
		if e == "" {
			continue
		}
		if strings.Contains(e, "BEGIN CERTIFICATE") {
			// treat as inline PEM
			if !certPool.AppendCertsFromPEM([]byte(e)) {
				return nil, nil, fmt.Errorf("failed to append trusted inline PEM")
			}
			continue
		}
		// treat as path (resolve relative to secretsDir)
		path := e
		if !filepath.IsAbs(path) {
			path = filepath.Join(secretsDir, path)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to read trusted cert %s: %w", e, err)
		}
		if !certPool.AppendCertsFromPEM(b) {
			return nil, nil, fmt.Errorf("failed to append trusted cert %s", e)
		}
	}

	serverTLSConfig := &tls.Config{
		Certificates: []tls.Certificate{identity},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    certPool,
		NextProtos:   []string{tlsServerName},
	}

	clientTLSConfig := &tls.Config{
		Certificates: []tls.Certificate{identity},
		RootCAs:      certPool,
		NextProtos:   []string{tlsServerName},
		ServerName:   tlsServerName, // SNI
	}

	return serverTLSConfig, clientTLSConfig, nil
}

func (m *Manager) OnNATInfo(f func(s string)) {
	m.natUpdate = f
}

func queryPublicEndpoint(server string) (string, error) {
	conn, err := net.DialTimeout("udp", server, 3*time.Second)
	if err != nil {
		return "", err
	}
	defer conn.Close()

	req, txID := buildStunRequest()
	if _, err := conn.Write(req); err != nil {
		return "", err
	}

	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	buf := make([]byte, 512)
	n, err := conn.Read(buf)
	if err != nil {
		return "", err
	}
	ep, err := parseStunResponse(buf[:n], txID)
	if err != nil {
		return "", err
	}
	return ep, nil
}

func buildStunRequest() ([]byte, [12]byte) {
	buf := make([]byte, 20)
	binary.BigEndian.PutUint16(buf[0:2], 0x0001)
	binary.BigEndian.PutUint16(buf[2:4], 0)
	binary.BigEndian.PutUint32(buf[4:8], 0x2112A442)
	var txID [12]byte
	if _, err := rand.Read(txID[:]); err != nil {
		panic(err)
	}
	copy(buf[8:], txID[:])
	return buf, txID
}

func parseStunResponse(resp []byte, txID [12]byte) (string, error) {
	if len(resp) < 20 {
		return "", fmt.Errorf("stun: short response")
	}
	if binary.BigEndian.Uint16(resp[0:2]) != 0x0101 {
		return "", fmt.Errorf("stun: unexpected message type")
	}
	if !equalBytes(resp[8:20], txID[:]) {
		return "", fmt.Errorf("stun: transaction mismatch")
	}
	length := int(binary.BigEndian.Uint16(resp[2:4]))
	attrs := resp[20:]
	if length < len(attrs) {
		attrs = attrs[:length]
	}
	const xorMappedAddress = 0x0020
	magic := uint32(0x2112A442)
	offset := 0
	for offset+4 <= len(attrs) {
		attrType := binary.BigEndian.Uint16(attrs[offset : offset+2])
		attrLen := int(binary.BigEndian.Uint16(attrs[offset+2 : offset+4]))
		offset += 4
		if offset+attrLen > len(attrs) {
			break
		}
		value := attrs[offset : offset+attrLen]
		offset += (attrLen + 3) &^ 3
		if attrType != xorMappedAddress || len(value) < 8 {
			continue
		}
		family := value[1]
		if family != 0x01 {
			continue
		}
		port := binary.BigEndian.Uint16(value[2:4]) ^ uint16(magic>>16)
		rawIP := binary.BigEndian.Uint32(value[4:8]) ^ magic
		ipBytes := make([]byte, 4)
		binary.BigEndian.PutUint32(ipBytes, rawIP)
		return fmt.Sprintf("%s:%d", net.IP(ipBytes).String(), port), nil
	}
	return "", fmt.Errorf("stun: no xor mapped address")
}

func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
