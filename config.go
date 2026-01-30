package main

import (
	"os"
	"time"
)

type Config struct {
	S3Endpoint  string
	S3Region    string
	S3AccessKey string
	S3SecretKey string
	S3BucketName  string

	AllowedPrefix string        // Only serve files under this prefix, e.g. "public/", empty for no restriction
	PresignExpiry time.Duration // How long pre-signed URLs are valid
	ListenAddress string
}

func LoadConfig() *Config {
	return &Config{
		S3Endpoint:    getEnv("S3_ENDPOINT", "https://s3.eu-central-1.wasabisys.com"),
		S3Region:      getEnv("S3_REGION", "eu-central-1"),
		S3AccessKey:   getEnv("S3_ACCESS_KEY", ""),
		S3SecretKey:   getEnv("S3_SECRET_KEY", ""),
		S3BucketName:    getEnv("S3_BUCKET", ""),
		AllowedPrefix: getEnv("ALLOWED_PREFIX", ""),
		ListenAddress: getEnv("LISTEN_ADDRESS", ":6969"),
		PresignExpiry: 15 * time.Minute,
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
