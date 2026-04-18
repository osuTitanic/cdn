package main

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"
)

type Config struct {
	S3Endpoint      string      `json:"s3_endpoint"`
	S3Region        string      `json:"s3_region"`
	S3AccessKey     string      `json:"s3_access_key"`
	S3SecretKey     string      `json:"s3_secret_key"`
	S3BucketName    string      `json:"s3_bucket_name"`
	AllowedPrefix   string      `json:"allowed_prefix"`
	PresignExpiry   Duration    `json:"presign_expiry"`
	ListenAddress   string      `json:"listen_address"`
	AdminAccessKeys []AccessKey `json:"admin_access_keys"`
}

type AccessKey struct {
	Name        string   `json:"name"`
	AccessKey   string   `json:"access_key"`
	Prefixes    []string `json:"prefixes"`
	Permissions []string `json:"permissions"`
}

func (key AccessKey) HasPermission(permission string) bool {
	return slices.Contains(key.Permissions, permission)
}

func (key AccessKey) AllowsPath(candidate string) bool {
	for _, allowedPrefix := range key.Prefixes {
		if allowedPrefix == "" || strings.HasPrefix(candidate, allowedPrefix) {
			return true
		}
	}
	return false
}

func LoadConfig(path string) (*Config, error) {
	cfg := &Config{
		S3Endpoint:      "https://s3.eu-central-1.wasabisys.com",
		S3Region:        "eu-central-1",
		S3AccessKey:     "",
		S3SecretKey:     "",
		S3BucketName:    "",
		AllowedPrefix:   "",
		PresignExpiry:   Duration{15 * time.Minute},
		ListenAddress:   ":6969",
		AdminAccessKeys: nil,
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return cfg, nil
}

type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("duration must be a string: %w", err)
	}

	parsed, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", s, err)
	}

	d.Duration = parsed
	return nil
}

func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}
