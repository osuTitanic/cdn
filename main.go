package main

import (
	"log"
	"net/http"
)

func main() {
	cfg := LoadConfig()
	if cfg.S3AccessKey == "" || cfg.S3SecretKey == "" || cfg.S3BucketName == "" {
		log.Fatal("S3_ACCESS_KEY, S3_SECRET_KEY, and S3_BUCKET are required")
	}

	handler, err := NewCdnHandler(cfg)
	if err != nil {
		log.Fatalf("Failed to create handler: %v", err)
	}

	log.Printf("S3 proxy listening on %s ...", cfg.ListenAddress)

	if err := http.ListenAndServe(cfg.ListenAddress, handler); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
