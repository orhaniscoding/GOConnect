package core

import (
	"context"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// CoreForwarderConfig carries loop knobs for backpressure and shutdown.
type CoreForwarderConfig struct {
	BufferPackets   int
	MaxFrameBytes   int
	ShutdownTimeout time.Duration
}

// Forwarder connects TUN <-> Transport with bounded buffers and graceful shutdown.
type Forwarder struct {
	ctx    context.Context
	cancel context.CancelFunc

	tun gtunDevice
	tr  Transport

	log *log.Logger

	mtu             int
	bufPackets      int
	maxFrameBytes   int
	shutdownTimeout time.Duration

	t2x chan []byte // TUN -> Transport
	x2t chan []byte // Transport -> TUN

	wg sync.WaitGroup

	forwardedT2X uint64
	droppedT2X   uint64
	forwardedX2T uint64
	droppedX2T   uint64

	lastErrUnix atomic.Int64

	done chan struct{}
}

// local alias to make testing easier without import loops.
type gtunDevice interface {
	Up() error
	Down() error
	IsUp() bool
	Read([]byte) (int, error)
	Write([]byte) (int, error)
}

func NewForwarder(parent context.Context, tun gtunDevice, tr Transport, mtu int, cfg CoreForwarderConfig, lg *log.Logger) *Forwarder {
	if cfg.BufferPackets <= 0 {
		cfg.BufferPackets = 256
	}
	if cfg.MaxFrameBytes <= 0 {
		cfg.MaxFrameBytes = 65535
	}
	if cfg.ShutdownTimeout <= 0 {
		cfg.ShutdownTimeout = 3 * time.Second
	}
	ctx, cancel := context.WithCancel(parent)
	return &Forwarder{
		ctx:             ctx,
		cancel:          cancel,
		tun:             tun,
		tr:              tr,
		log:             lg,
		mtu:             mtu,
		bufPackets:      cfg.BufferPackets,
		maxFrameBytes:   cfg.MaxFrameBytes,
		shutdownTimeout: cfg.ShutdownTimeout,
		t2x:             make(chan []byte, cfg.BufferPackets),
		x2t:             make(chan []byte, cfg.BufferPackets),
		done:            make(chan struct{}),
	}
}

func (f *Forwarder) Start() {
	// TUN reader: dedicated helper goroutine does blocking Read and feeds a channel.
	tunReadCh := make(chan []byte, f.bufPackets)
	go func() {
		buf := make([]byte, f.maxFrameBytes)
		for {
			n, err := f.tun.Read(buf)
			if err != nil || n <= 0 {
				time.Sleep(10 * time.Millisecond)
				continue
			}
			pkt := make([]byte, n)
			copy(pkt, buf[:n])
			select {
			case tunReadCh <- pkt:
			default:
				// drop if channel full; controller will account
			}
		}
	}()
	f.wg.Add(1)
	go func() {
		defer f.wg.Done()
		for {
			select {
			case <-f.ctx.Done():
				return
			case pkt := <-tunReadCh:
				if pkt == nil {
					continue
				}
				select {
				case f.t2x <- pkt:
					atomic.AddUint64(&f.forwardedT2X, 1)
				default:
					atomic.AddUint64(&f.droppedT2X, 1)
					if f.log != nil {
						f.log.Printf("forwarder dir=t2x event=drop len=%d", len(pkt))
					}
				}
			}
		}
	}()

	// Transport reader: dedicated helper goroutine
	trReadCh := make(chan []byte, f.bufPackets)
	go func() {
		for {
			b, err := f.tr.RecvPacket()
			if err != nil || len(b) == 0 {
				time.Sleep(10 * time.Millisecond)
				continue
			}
			select {
			case trReadCh <- b:
			default:
			}
		}
	}()
	f.wg.Add(1)
	go func() {
		defer f.wg.Done()
		for {
			select {
			case <-f.ctx.Done():
				return
			case b := <-trReadCh:
				if b == nil {
					continue
				}
				select {
				case f.x2t <- b:
					atomic.AddUint64(&f.forwardedX2T, 1)
				default:
					atomic.AddUint64(&f.droppedX2T, 1)
					if f.log != nil {
						f.log.Printf("forwarder dir=x2t event=drop len=%d", len(b))
					}
				}
			}
		}
	}()

	// Transport writer (consume t2x)
	f.wg.Add(1)
	go func() {
		defer f.wg.Done()
		for {
			select {
			case <-f.ctx.Done():
				return
			case b := <-f.t2x:
				if b == nil {
					continue
				}
				// simple timeout loop to respect shutdown
				sendDone := make(chan struct{})
				go func() {
					_ = f.tr.SendPacket(b)
					close(sendDone)
				}()
				select {
				case <-f.ctx.Done():
					return
				case <-sendDone:
				case <-time.After(2 * time.Second):
					if f.log != nil {
						f.log.Printf("forwarder dir=t2x event=send_timeout len=%d", len(b))
					}
				}
			}
		}
	}()

	// TUN writer (consume x2t)
	f.wg.Add(1)
	go func() {
		defer f.wg.Done()
		for {
			select {
			case <-f.ctx.Done():
				return
			case b := <-f.x2t:
				if b == nil {
					continue
				}
				// Write and ignore short errors; we log and continue
				if _, err := f.tun.Write(b); err != nil {
					f.lastErrUnix.Store(time.Now().Unix())
					if f.log != nil {
						f.log.Printf("forwarder dir=x2t event=write_err err=%v", err)
					}
				}
			}
		}
	}()

	// watcher to close done when all goroutines finish after Stop
	f.wg.Add(1)
	go func() {
		defer f.wg.Done()
		// nothing, just wait in Stop then close done
		<-f.ctx.Done()
	}()
}

func (f *Forwarder) Stop() error {
	// Cancel context so goroutines can exit
	if f.cancel != nil {
		f.cancel()
	}
	// Attempt to unblock any potential Read by lowering the TUN
	if f.tun != nil {
		_ = f.tun.Down()
	}
	c := make(chan struct{})
	go func() { f.wg.Wait(); close(c) }()
	select {
	case <-c:
		close(f.done)
		return nil
	case <-time.After(f.shutdownTimeout):
		if f.log != nil {
			f.log.Printf("forwarder event=shutdown_timeout")
		}
		close(f.done)
		return context.DeadlineExceeded
	}
}

func (f *Forwarder) Done() <-chan struct{} { return f.done }

type ForwarderStats struct {
	ForwardedT2X uint64
	DroppedT2X   uint64
	ForwardedX2T uint64
	DroppedX2T   uint64
	LastErrUnix  int64
}

func (f *Forwarder) Stats() ForwarderStats {
	return ForwarderStats{
		ForwardedT2X: atomic.LoadUint64(&f.forwardedT2X),
		DroppedT2X:   atomic.LoadUint64(&f.droppedT2X),
		ForwardedX2T: atomic.LoadUint64(&f.forwardedX2T),
		DroppedX2T:   atomic.LoadUint64(&f.droppedX2T),
		LastErrUnix:  f.lastErrUnix.Load(),
	}
}
