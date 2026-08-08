//go:build windows

package main

import (
	"context"
	"fmt"
	"log"
	"os/signal"
	"syscall"
	"time"

	"github.com/kros/hubabox/internal/config"
	"golang.org/x/sys/windows/svc"
)

const winServiceName = "HubaBox"

func main() {
	log.SetPrefix("hubabox ")
	log.SetFlags(log.LstdFlags)

	isSvc, err := svc.IsWindowsService()
	if err != nil {
		log.Fatalf("svc check: %v", err)
	}
	if isSvc {
		// Services have no console: stderr is discarded, so log to a file
		// under the data dir before doing anything that can fail.
		setupFileLogging(config.ResolveDataDir(), false)
		if err := svc.Run(winServiceName, &hubaboxService{}); err != nil {
			log.Fatal(err)
		}
		return
	}

	// Interactive (double-clicked exe or terminal): keep console output and
	// also tee to the log file, so errors survive the window closing.
	setupFileLogging(config.ResolveDataDir(), true)
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	if err := run(ctx); err != nil {
		log.Fatal(err)
	}
}

type hubaboxService struct{}

func (*hubaboxService) Execute(args []string, requests <-chan svc.ChangeRequest, changes chan<- svc.Status) (svcSpecificErr bool, exitCode uint32) {
	const accept = svc.AcceptStop | svc.AcceptShutdown
	changes <- svc.Status{State: svc.StartPending}
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- fmt.Errorf("hubabox panicked: %v", r)
			}
		}()
		done <- run(ctx)
	}()

	changes <- svc.Status{State: svc.Running, Accepts: accept}

	shutdownGrace := func() {
		cancel()
		shutdownWait := time.NewTimer(20 * time.Second)
		select {
		case <-shutdownWait.C:
			log.Printf("service: shutdown wait timed out")
		case err := <-done:
			if err != nil {
				log.Printf("service: run ended: %v", err)
			}
		}
		shutdownWait.Stop()
	}

	for {
		select {
		case req := <-requests:
			switch req.Cmd {
			case svc.Interrogate:
				changes <- req.CurrentStatus
			case svc.Stop, svc.Shutdown:
				changes <- svc.Status{State: svc.StopPending}
				shutdownGrace()
				changes <- svc.Status{State: svc.Stopped}
				return false, 0
			default:
				changes <- svc.Status{State: svc.Running, Accepts: accept}
			}
		case err := <-done:
			if err != nil {
				// Report a real failure to the SCM (services.msc shows a
				// service-specific exit code); details are in hubabox.log.
				log.Printf("service: run exited with error: %v", err)
				cancel()
				changes <- svc.Status{State: svc.Stopped}
				return false, 1
			}
			cancel()
			changes <- svc.Status{State: svc.Stopped}
			return false, 0
		}
	}
}
