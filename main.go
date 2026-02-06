package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/n0madic/ssh-mcp/internal/config"
	"github.com/n0madic/ssh-mcp/internal/server"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	cfg, err := config.Parse()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		log.Printf("Received signal %v, shutting down...", sig)
		cancel()
	}()

	srv, err := server.New(ctx, cfg)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	if err := srv.Run(ctx); err != nil {
		log.Fatalf("Server error: %v", err)
		os.Exit(1)
	}
}
