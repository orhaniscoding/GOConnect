package transport

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	quic "github.com/quic-go/quic-go"
)

// Transport is a simple packet-oriented interface used by the core forwarding loop.
// It provides a best-effort delivery over QUIC using a length-prefixed stream.
type Transport interface {
	Start() error
	Stop() error
	SendPacket([]byte) error
	RecvPacket() ([]byte, error)
}

// QUICTransport implements Transport using quic-go and the Manager's TLS model.
type QUICTransport struct {
	// config
	udpAddr     string
	peers       []string
	secretsDir  string
	trusted     []string
	stunServers []string

	// internals
	mu          sync.RWMutex
	started     bool
	mgr         *Manager
	stream      io.ReadWriteCloser
	closeSessFn func()
	sendCh      chan []byte
	recvCh      chan []byte
	stopCh      chan struct{}
	stopped     chan struct{}
}

// NewQUICTransport constructs a QUICTransport bound to udpAddr and prepared to dial peers.
func NewQUICTransport(udpAddr string, peers []string, secretsDir string, trusted []string, stunServers []string) (*QUICTransport, error) {
	mgr, err := NewManager(udpAddr, stunServers, secretsDir, trusted)
	if err != nil {
		return nil, err
	}
	return &QUICTransport{
		udpAddr:     udpAddr,
		peers:       append([]string(nil), peers...),
		secretsDir:  secretsDir,
		trusted:     append([]string(nil), trusted...),
		stunServers: append([]string(nil), stunServers...),
		mgr:         mgr,
		sendCh:      make(chan []byte, 256),
		recvCh:      make(chan []byte, 256),
		stopCh:      make(chan struct{}),
		stopped:     make(chan struct{}),
	}, nil
}

func (t *QUICTransport) Start() error {
	t.mu.Lock()
	if t.started {
		t.mu.Unlock()
		return nil
	}
	t.started = true
	t.mu.Unlock()

	// Start server side (listener, accept loop inside Manager)
	if err := t.mgr.Start(t.peers); err != nil {
		return err
	}

	go t.dialAndPump()
	return nil
}

func (t *QUICTransport) Stop() error {
	t.mu.Lock()
	if !t.started {
		t.mu.Unlock()
		return nil
	}
	t.started = false
	close(t.stopCh)
	t.mu.Unlock()

	// Close stream/session if present
	t.mu.Lock()
	if t.stream != nil {
		_ = t.stream.Close()
		t.stream = nil
	}
	if t.closeSessFn != nil {
		t.closeSessFn()
		t.closeSessFn = nil
	}
	t.mu.Unlock()

	_ = t.mgr.Stop()
	close(t.stopped)
	return nil
}

func (t *QUICTransport) SendPacket(pkt []byte) error {
	if len(pkt) == 0 {
		return nil
	}
	t.mu.RLock()
	started := t.started
	t.mu.RUnlock()
	if !started {
		return errors.New("transport not started")
	}
	select {
	case t.sendCh <- append([]byte(nil), pkt...):
		return nil
	case <-t.stopCh:
		return errors.New("transport stopped")
	case <-time.After(3 * time.Second):
		return errors.New("send timeout")
	}
}

func (t *QUICTransport) RecvPacket() ([]byte, error) {
	select {
	case b := <-t.recvCh:
		return b, nil
	case <-t.stopCh:
		return nil, errors.New("transport stopped")
	}
}

// dialAndPump maintains a client session/stream to the first reachable peer and pumps packets.
func (t *QUICTransport) dialAndPump() {
	defer func() {
		// signal stopped only when Stop() closes stopped
	}()
	var backoff = time.Second
	for {
		// Stop?
		select {
		case <-t.stopCh:
			return
		default:
		}

		peer := firstNonEmpty(t.peers)
		if peer == "" {
			time.Sleep(500 * time.Millisecond)
			continue
		}

		// Clone client TLS from Manager and dial
		clientTLS := t.mgr.clientTLSConfig.Clone()
		if clientTLS == nil {
			// fallback: build anew
			id := t.mgr.GetIdentity()
			srv, cli, err := newTLSConfigs(id, id.Leaf, t.secretsDir, t.trusted)
			_ = srv
			if err == nil {
				clientTLS = cli
			} else {
				time.Sleep(backoff)
				continue
			}
		}
		// Dial
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		sess, err := quic.DialAddr(ctx, peer, clientTLS, &quic.Config{})
		cancel()
		if err != nil {
			time.Sleep(backoff)
			backoff = min(backoff*2, 10*time.Second)
			continue
		}
		// Reset backoff upon success
		backoff = time.Second
		// Open a stream
		str, err := sess.OpenStreamSync(context.Background())
		if err != nil {
			_ = sess.CloseWithError(0, "stream open fail")
			continue
		}
		t.mu.Lock()
		t.stream = str
		t.closeSessFn = func() { _ = sess.CloseWithError(0, "stop") }
		t.mu.Unlock()

		// Start writers/readers
		var wg sync.WaitGroup
		wg.Add(2)
		// writer loop
		go func() {
			defer wg.Done()
			for {
				select {
				case <-t.stopCh:
					return
				case b := <-t.sendCh:
					// length-prefix
					if err := writeFrame(str, b); err != nil {
						return
					}
				}
			}
		}()
		// reader loop
		go func() {
			defer wg.Done()
			for {
				buf, err := readFrame(str)
				if err != nil {
					return
				}
				select {
				case t.recvCh <- buf:
				case <-t.stopCh:
					return
				}
			}
		}()

		// Wait for either stop or stream/session error
		finished := make(chan struct{})
		go func() { wg.Wait(); close(finished) }()
		select {
		case <-t.stopCh:
			_ = str.Close()
			_ = sess.CloseWithError(0, "stop")
			return
		case <-finished:
			_ = str.Close()
			_ = sess.CloseWithError(0, "eof")
			t.mu.Lock()
			t.stream = nil
			t.closeSessFn = nil
			t.mu.Unlock()
			// loop to redial
		}
	}
}

func writeFrame(w io.Writer, b []byte) error {
	if len(b) > 0xFFFF {
		return fmt.Errorf("frame too large: %d", len(b))
	}
	hdr := []byte{byte(len(b) >> 8), byte(len(b))}
	if _, err := w.Write(hdr); err != nil {
		return err
	}
	_, err := w.Write(b)
	return err
}

func readFrame(r io.Reader) ([]byte, error) {
	hdr := make([]byte, 2)
	if _, err := io.ReadFull(r, hdr); err != nil {
		return nil, err
	}
	n := int(hdr[0])<<8 | int(hdr[1])
	if n <= 0 {
		return nil, fmt.Errorf("invalid frame size")
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

func firstNonEmpty(ss []string) string {
	for _, s := range ss {
		if s != "" {
			return s
		}
	}
	return ""
}

func min(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

// Ensure we import net for lints even if unused in certain builds
var _ = net.IPv4len
