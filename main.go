package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	cfg, err := LoadConfig("config.json")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if cfg.S3AccessKey == "" || cfg.S3SecretKey == "" || cfg.S3BucketName == "" {
		log.Fatal("S3 access key, secret key & bucket name are required")
	}

	handler, err := NewCdnHandler(cfg)
	if err != nil {
		log.Fatalf("Failed to create handler: %v", err)
	}

	server := &http.Server{
		Addr:    cfg.ListenAddress,
		Handler: handler.Router(),
	}
	log.Printf("CDN is listening on %s ...", cfg.ListenAddress)

	// Run server in a separate goroutine to allow for graceful shutdown
	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- server.ListenAndServe()
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-serverErrors:
		// Server encountered an error, log it & exit
		if err != nil && err != http.ErrServerClosed {
			closeHandler(handler)
			log.Fatalf("Server error: %v", err)
		}
	case <-ctx.Done():
		// Received shutdown signal, proceed to shutdown
	}

	shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownContext); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}
	if err := handler.Close(shutdownContext); err != nil {
		log.Printf("Failed to close handler: %v", err)
	}
}

func closeHandler(handler *CdnHandler) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := handler.Close(ctx); err != nil {
		log.Printf("Failed to close handler: %v", err)
	}
}
