package main

import (
	"context"
	"encoding/binary"
	"sync"

	"go.etcd.io/bbolt"
)

const DownloadCounterBucketName = "download_counts"

// BBoltDownloadCounter implements DownloadCounter using a bbolt database
type BBoltDownloadCounter struct {
	db     *bbolt.DB
	bucket []byte

	closeOnce  sync.Once
	closeDone  chan struct{}
	closeError error
}

// NewBBoltDownloadCounter creates a new bbolt-backed download counter
func NewBBoltDownloadCounter(path string, options *bbolt.Options) (*BBoltDownloadCounter, error) {
	db, err := bbolt.Open(path, 0o600, options)
	if err != nil {
		return nil, err
	}

	counter := &BBoltDownloadCounter{
		db:        db,
		bucket:    []byte(DownloadCounterBucketName),
		closeDone: make(chan struct{}),
	}
	err = db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(counter.bucket)
		return err
	})
	if err != nil {
		db.Close()
		return nil, err
	}

	return counter, nil
}

// Increment increments the download count for the given key
func (c *BBoltDownloadCounter) Increment(key string) {
	c.db.Update(func(tx *bbolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists(c.bucket)
		if err != nil {
			return err
		}
		raw := bucket.Get([]byte(key))

		// We expect the value to be an 8-byte big-endian uint64
		// If it's not, we treat it as 0
		var count uint64
		if len(raw) == 8 {
			count = binary.BigEndian.Uint64(raw)
		}

		// Increment the count & write it back as big-endian uint64
		count++

		var buf [8]byte
		binary.BigEndian.PutUint64(buf[:], count)

		return bucket.Put([]byte(key), buf[:])
	})
}

// Set sets the download count for key to count
func (c *BBoltDownloadCounter) Set(key string, count uint64) {
	c.db.Update(func(tx *bbolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists(c.bucket)
		if err != nil {
			return err
		}

		// Write the count as an 8-byte big-endian uint64
		var buf [8]byte
		binary.BigEndian.PutUint64(buf[:], count)

		return bucket.Put([]byte(key), buf[:])
	})
}

// Counts returns the download counts for the given keys
func (c *BBoltDownloadCounter) Counts(keys []string) map[string]uint64 {
	result := make(map[string]uint64, len(keys))
	for _, key := range keys {
		result[key] = 0
	}

	c.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(c.bucket)
		if bucket == nil {
			return nil
		}

		for _, key := range keys {
			// Again, we expect the value to be an 8-byte big-endian uint64
			raw := bucket.Get([]byte(key))
			if len(raw) == 8 {
				result[key] = binary.BigEndian.Uint64(raw)
			}
		}
		return nil
	})
	return result
}

// Reset resets the download count for key back to 0
func (c *BBoltDownloadCounter) Reset(key string) {
	c.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(c.bucket)
		if bucket == nil {
			return nil
		}
		return bucket.Delete([]byte(key))
	})
}

// Close closes the underlying bbolt database
func (c *BBoltDownloadCounter) Close(ctx context.Context) error {
	c.closeOnce.Do(func() {
		go func() {
			c.closeError = c.db.Close()
			close(c.closeDone)
		}()
	})

	select {
	case <-c.closeDone:
		return c.closeError
	case <-ctx.Done():
		return ctx.Err()
	}
}
