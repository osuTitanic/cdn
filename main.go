package main

import (
	"log"
	"net/http"
	"os"
)

func main() {
	cfg, err := LoadConfig("config.json")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
		os.Exit(1)
	}

	if cfg.S3AccessKey == "" || cfg.S3SecretKey == "" || cfg.S3BucketName == "" {
		log.Fatal("S3 access key, secret key & bucket name are required")
		os.Exit(1)
	}

	handler, err := NewCdnHandler(cfg)
	if err != nil {
		log.Fatalf("Failed to create handler: %v", err)
		os.Exit(1)
	}

	log.Printf("CDN is listening on %s ...", cfg.ListenAddress)

	if err := http.ListenAndServe(cfg.ListenAddress, handler); err != nil {
		log.Fatalf("Server error: %v", err)
		os.Exit(1)
	}
}
