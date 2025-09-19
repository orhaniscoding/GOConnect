package core

import (
	"context"
	"sync"
	"testing"
	"time"
)

type fakeTun struct {
	up  bool
	rch chan []byte
	wch chan []byte
}

func (f *fakeTun) Up() error   { f.up = true; return nil }
func (f *fakeTun) Down() error { f.up = false; return nil }
func (f *fakeTun) IsUp() bool  { return f.up }
func (f *fakeTun) Read(b []byte) (int, error) {
	pkt := <-f.rch
	if pkt == nil {
		time.Sleep(10 * time.Millisecond)
		return 0, nil
	}
	n := copy(b, pkt)
	return n, nil
}
func (f *fakeTun) Write(b []byte) (int, error) {
	f.wch <- append([]byte(nil), b...)
	return len(b), nil
}

type fakeTransport struct {
	send chan []byte
	recv chan []byte
}

func (t *fakeTransport) SendPacket(b []byte) error   { t.recv <- append([]byte(nil), b...); return nil }
func (t *fakeTransport) RecvPacket() ([]byte, error) { return <-t.send, nil }

func TestForwarderHappyPath(t *testing.T) {
	tun := &fakeTun{rch: make(chan []byte, 16), wch: make(chan []byte, 16)}
	tr := &fakeTransport{send: make(chan []byte, 16), recv: make(chan []byte, 16)}

	ctx := context.Background()
	f := NewForwarder(ctx, tun, tr, 1280, CoreForwarderConfig{BufferPackets: 8, MaxFrameBytes: 2048, ShutdownTimeout: time.Second}, nil)
	f.Start()

	// TUN -> Transport
	msg1 := []byte("hello")
	tun.rch <- msg1
	select {
	case out := <-tr.recv:
		if string(out) != string(msg1) {
			t.Fatalf("t2x mismatch: got %q want %q", out, msg1)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout t2x")
	}

	// Transport -> TUN
	msg2 := []byte("world")
	tr.send <- msg2
	select {
	case out := <-tun.wch:
		if string(out) != string(msg2) {
			t.Fatalf("x2t mismatch: got %q want %q", out, msg2)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout x2t")
	}

	_ = f.Stop()
}

func TestForwarderShutdown(t *testing.T) {
	tun := &fakeTun{rch: make(chan []byte, 1), wch: make(chan []byte, 1)}
	tr := &fakeTransport{send: make(chan []byte, 1), recv: make(chan []byte, 1)}

	f := NewForwarder(context.Background(), tun, tr, 1280, CoreForwarderConfig{BufferPackets: 4, MaxFrameBytes: 1024, ShutdownTimeout: time.Second}, nil)
	f.Start()

	// Start background traffic
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			tun.rch <- []byte("x")
		}
	}()

	// Stop and ensure it finishes within timeout
	if err := f.Stop(); err != nil {
		t.Fatalf("stop error: %v", err)
	}
	wg.Wait()
}
