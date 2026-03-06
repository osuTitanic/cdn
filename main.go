package main

import (
	"log"
	"net/http"
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

	httpServer := &http.Server{
		Addr:    cfg.ListenAddress,
		Handler: handler.Router(),
	}
	log.Printf("CDN is listening on %s ...", cfg.ListenAddress)

	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}
}
