//go:build windows

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	gtun "goconnect/internal/tun"
)

func main() {
	var doLoopback bool
	var timeout time.Duration
	flag.BoolVar(&doLoopback, "loopback", false, "Attempt UDP loopback test (requires address configured and admin)")
	flag.DurationVar(&timeout, "timeout", 2*time.Second, "Timeout for loopback test")
	flag.Parse()

	d := gtun.New()
	if err := d.Up(); err != nil {
		log.Fatalf("tun up: %v", err)
	}
	defer d.Down()
	fmt.Println("TUN device is up (wintun or stub)")

	if doLoopback {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		if err := d.LoopbackTest(ctx); err != nil {
			log.Fatalf("loopback: %v", err)
		}
		fmt.Println("loopback OK")
	}
}
