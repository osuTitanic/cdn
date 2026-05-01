package main

import (
	"context"
)

// DownloadCounter is an interface for tracking download counts
type DownloadCounter interface {
	Increment(key string)
	Counts(keys []string) map[string]uint64
	Reset(key string)
	Close(ctx context.Context) error
}

func NewDownloadCounterFromConfig(cfg *Config) (DownloadCounter, error) {
	if !configTracksDownloads(cfg) {
		return NewNoopDownloadCounter(), nil
	}
	return NewBBoltDownloadCounter(cfg.DownloadCountsPath, nil)
}

func configTracksDownloads(cfg *Config) bool {
	for _, accessKey := range cfg.AdminAccessKeys {
		if accessKey.TrackDownloads {
			return true
		}
	}
	return false
}
