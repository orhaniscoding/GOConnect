package transport

import (
	"testing"
	"time"
)

// Test basic send/recv over a loopback QUICTransport: start listener and dial itself via 127.0.0.1.
func TestQUICTransport_RoundTrip(t *testing.T) {
	// Use a random high port; quic-go will bind on Start
	addr := "127.0.0.1:0"
	// Share a secrets dir so both sides trust the same CA
	sec := t.TempDir()
	tr, err := NewQUICTransport(addr, []string{}, sec, nil, nil)
	if err != nil {
		t.Fatalf("new transport: %v", err)
	}

	// We don't have a peer to dial; instead, we start another transport as client dialing server's addr.
	// First, start server manager to discover bound addr.
	if err := tr.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(func() { _ = tr.Stop() })

	// Wait briefly for listener to bind and expose addr via UDP
	time.Sleep(200 * time.Millisecond)

	// Snapshot: Manager listener exposes no direct method to get addr; we reuse configured udpAddr.
	// For test, construct a client-only transport to dial the server at the configured port if non-zero.
	// Recreate with a specific localhost:port if tr.mgr.ln != nil.
	if tr.mgr.BoundAddr() == nil {
		t.Fatalf("listener not ready")
	}
	la := tr.mgr.BoundAddr().String()

	cli, err := NewQUICTransport("127.0.0.1:0", []string{la}, sec, nil, nil)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	if err := cli.Start(); err != nil {
		t.Fatalf("client start: %v", err)
	}
	t.Cleanup(func() { _ = cli.Stop() })

	// Wait for client's dialer to establish a stream
	deadline := time.Now().Add(3 * time.Second)
	for {
		cli.mu.RLock()
		ready := cli.stream != nil
		cli.mu.RUnlock()
		if ready {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("client stream not established")
		}
		time.Sleep(20 * time.Millisecond)
	}

	// Send a packet from client to server and ensure we receive echo back on client
	msg := []byte("hello")
	if err := cli.SendPacket(msg); err != nil {
		t.Fatalf("send: %v", err)
	}

	select {
	case got := <-cli.recvCh:
		if string(got) != string(msg) {
			t.Fatalf("mismatch: got %q want %q", got, msg)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for recv")
	}
}
