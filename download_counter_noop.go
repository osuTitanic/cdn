package main

import "context"

// NoopDownloadCounter is used for when download tracking is disabled for all access keys
type NoopDownloadCounter struct{}

func NewNoopDownloadCounter() *NoopDownloadCounter {
	return &NoopDownloadCounter{}
}

func (NoopDownloadCounter) Increment(string) {}

func (NoopDownloadCounter) Reset(string) {}

func (NoopDownloadCounter) Close(context.Context) error {
	return nil
}

func (NoopDownloadCounter) Counts(keys []string) map[string]uint64 {
	counts := make(map[string]uint64, len(keys))
	for _, key := range keys {
		counts[key] = 0
	}
	return counts
}
