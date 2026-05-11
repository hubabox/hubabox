//go:build !windows

package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	log.SetPrefix("hubabox ")
	log.SetFlags(log.LstdFlags)

	if err := run(ctx); err != nil {
		log.Fatal(err)
	}
}
