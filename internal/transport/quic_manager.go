package transport

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
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
	addr           string
	stunServers    []string
	mu             sync.RWMutex
	peers          map[string]*PeerInfo
	publicEndpoint string
	natUpdate      func(string)
	ln             *quic.Listener
	stopCh         chan struct{}
}

func NewManager(udpAddr string, stunServers []string) *Manager {
	servers := append([]string(nil), stunServers...)
	return &Manager{addr: udpAddr, stunServers: servers, peers: map[string]*PeerInfo{}}
}

func (m *Manager) Start(peers []string) error {
	if m.ln != nil {
		return nil
	}
	tlsConf := generateTLS()
	udpAddr, err := net.ResolveUDPAddr("udp", m.addr)
	if err != nil {
		return err
	}
	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return err
	}
	ln, err := quic.Listen(conn, tlsConf, &quic.Config{})
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
	tlsConf := &tls.Config{InsecureSkipVerify: true, NextProtos: []string{"goc/1"}}
	stop := m.stopCh
	for {
		for _, p := range peers {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			start := time.Now()
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
			}
			cancel()
			select {
			case <-time.After(2 * time.Second):
			case <-stop:
				return
			}
		}
		select {
		case <-stop:
			return
		default:
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
		addr, err := queryPublicEndpoint(server)
		if err != nil {
			continue
		}
		m.setPublicEndpoint(addr)
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
		offset += (attrLen + 3) &^ 3 // 32-bit alignment
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

func generateTLS() *tls.Config {
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	der, _ := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	cert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	pkcs8, _ := x509.MarshalPKCS8PrivateKey(key)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8})
	tlsCert, _ := tls.X509KeyPair(cert, keyPEM)
	return &tls.Config{Certificates: []tls.Certificate{tlsCert}, NextProtos: []string{"goc/1"}, InsecureSkipVerify: true}
}
