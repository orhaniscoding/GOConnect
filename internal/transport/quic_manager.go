package transport

import (
    "context"
    "crypto/rand"
    "crypto/rsa"
    "crypto/tls"
    "crypto/x509"
    "encoding/pem"
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
    addr     string
    mu       sync.RWMutex
    peers    map[string]*PeerInfo
    ln       *quic.Listener
    stopCh   chan struct{}
}

func NewManager(udpAddr string) *Manager {
    return &Manager{addr: udpAddr, peers: map[string]*PeerInfo{}, stopCh: make(chan struct{})}
}

func (m *Manager) Start(peers []string) error {
    if m.ln != nil { return nil }
    tlsConf := generateTLS()
    udpAddr, err := net.ResolveUDPAddr("udp", m.addr)
    if err != nil { return err }
    conn, err := net.ListenUDP("udp", udpAddr)
    if err != nil { return err }
    ln, err := quic.Listen(conn, tlsConf, &quic.Config{})
    if err != nil { return err }
    m.ln = ln
    go m.acceptLoop()
    if len(peers) > 0 { go m.dialLoop(peers) }
    return nil
}

func (m *Manager) Stop() error {
    if m.ln != nil { _ = m.ln.Close(); m.ln = nil }
    close(m.stopCh)
    return nil
}

func (m *Manager) acceptLoop() {
    for {
        sess, err := m.ln.Accept(context.Background())
        if err != nil { return }
        go func(){
            s := sess
            defer s.CloseWithError(0, "bye")
            m.touchPeer(s.RemoteAddr().String())
            // Simple ping-pong
            str, err := s.AcceptStream(context.Background())
            if err != nil { return }
            buf := make([]byte, 1024)
            for {
                n, err := str.Read(buf)
                if err != nil { return }
                m.touchPeer(s.RemoteAddr().String())
                _, _ = str.Write(buf[:n])
            }
        }()
    }
}

func (m *Manager) dialLoop(peers []string) {
    tlsConf := &tls.Config{InsecureSkipVerify: true, NextProtos: []string{"goc/1"}}
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
            time.Sleep(2 * time.Second)
        }
        select { case <-m.stopCh: return; default: }
    }
}

func (m *Manager) touchPeer(addr string) {
    m.mu.Lock()
    pi := m.peers[addr]
    if pi == nil { pi = &PeerInfo{Address: addr}; m.peers[addr] = pi }
    pi.LastSeen = time.Now()
    m.mu.Unlock()
}

func (m *Manager) updateRTT(addr string, rtt int) {
    m.mu.Lock()
    pi := m.peers[addr]
    if pi == nil { pi = &PeerInfo{Address: addr}; m.peers[addr] = pi }
    pi.RTTms = rtt
    pi.P2P = true
    pi.LastSeen = time.Now()
    m.mu.Unlock()
}

func (m *Manager) SnapshotPeers() []PeerInfo {
    m.mu.RLock(); defer m.mu.RUnlock()
    out := make([]PeerInfo, 0, len(m.peers))
    for _, p := range m.peers { out = append(out, *p) }
    return out
}

func generateTLS() *tls.Config {
    // Self-signed for transport; real PKI will come later.
    tmpl := &x509.Certificate{
        SerialNumber: big.NewInt(time.Now().UnixNano()),
        NotBefore:    time.Now().Add(-time.Hour),
        NotAfter:     time.Now().Add(365 * 24 * time.Hour),
        KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
        ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
        IsCA:         true,
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
