// devvault-relay is the DevVault Anywhere relay server. Run it on any host
// with a public IP (VPS, cloud VM) or locally for development:
//
//	devvault-relay --addr :7921 --data-dir ./relay-data
//
// It prints the TLS certificate fingerprint on startup; agents and clients
// pin this fingerprint to prevent man-in-the-middle attacks.
package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"devvault/internal/relay"
)

func main() {
	addr := flag.String("addr", ":7921", "listen address")
	dataDir := flag.String("data-dir", "./relay-data", "data directory for the relay database and TLS certificate")
	verbose := flag.Bool("v", false, "verbose logging")
	flag.Parse()

	level := slog.LevelInfo
	if *verbose {
		level = slog.LevelDebug
	}
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level}))

	srv, err := relay.New(relay.Config{
		Addr:    *addr,
		DataDir: filepath.Clean(*dataDir),
		Logger:  log,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to start relay: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("DevVault Anywhere relay")
	fmt.Printf("  listening on : %s\n", srv.Addr())
	fmt.Printf("  data dir    : %s\n", filepath.Clean(*dataDir))
	fmt.Printf("  TLS fingerprint (SHA-256), pin this in agents/clients:\n    %s\n", srv.Fingerprint())
	fmt.Println("Agents connect with: devvault (in-app Settings) | devvault-cli --relay <host:port> --fingerprint <fp>")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Run() }()
	select {
	case err := <-errCh:
		if err != nil {
			log.Error("relay stopped", "err", err)
			os.Exit(1)
		}
	case <-stop:
		log.Info("shutting down")
		srv.Close()
	}
}
