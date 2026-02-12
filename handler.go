package main

import (
	"context"
	"io"
	"log"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type CdnHandler struct {
	s3Client  *s3.Client
	presigner *s3.PresignClient
	config    *Config
}

func NewCdnHandler(cfg *Config) (*CdnHandler, error) {
	s3Client := s3.New(s3.Options{
		Region:       cfg.S3Region,
		BaseEndpoint: aws.String(cfg.S3Endpoint),
		Credentials: credentials.NewStaticCredentialsProvider(
			cfg.S3AccessKey,
			cfg.S3SecretKey,
			"",
		),
	})

	return &CdnHandler{
		s3Client:  s3Client,
		presigner: s3.NewPresignClient(s3Client),
		config:    cfg,
	}, nil
}

func (h *CdnHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Janky way of doing health checks, but I don't care
	if r.URL.Path == "/health" {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("we are gaming"))
		return
	}

	// Determine mode based on host
	host := strings.ToLower(r.Host)
	isDirect := strings.HasPrefix(host, "s3.")
	isStream := strings.HasPrefix(host, "cdn.")

	if !isDirect && !isStream {
		http.Error(w, "Invalid host", http.StatusBadRequest)
		return
	}

	// Clean and validate path
	objectKey := strings.TrimPrefix(r.URL.Path, "/")
	if objectKey == "" {
		http.Error(w, "No path specified", http.StatusBadRequest)
		return
	}

	// Prevent path traversal
	objectKey = path.Clean(objectKey)
	if strings.HasPrefix(objectKey, "..") || strings.Contains(objectKey, "/../") {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	if h.config.AllowedPrefix != "" {
		// Strip the prefix if already present to avoid double-prefixing
		objectKey = strings.TrimPrefix(objectKey, h.config.AllowedPrefix)
		// Apply allowed prefix
		objectKey = h.config.AllowedPrefix + objectKey
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	presignOptions := s3.WithPresignExpires(h.config.PresignExpiry)
	objectInput := &s3.GetObjectInput{
		Bucket: aws.String(h.config.S3BucketName),
		Key:    aws.String(objectKey),
	}

	// Generate presigned URL
	presignedReq, err := h.presigner.PresignGetObject(ctx, objectInput, presignOptions)
	if err != nil {
		log.Printf("Failed to presign URL for %s: %v", objectKey, err)
		http.Error(w, "Failed to generate URL", http.StatusInternalServerError)
		return
	}

	// Log incoming request
	log.Printf("%s %s %s [%s] (%s)", r.Method, host, r.URL.Path, r.RemoteAddr, r.UserAgent())

	if isDirect {
		// Redirect to presigned URL, if using s3.<domain>
		http.Redirect(w, r, presignedReq.URL, http.StatusTemporaryRedirect)
		return
	}

	// Fallback to streaming mode, if using cdn.<domain>
	h.streamObject(ctx, w, r, presignedReq.URL, objectKey)
}

func (h *CdnHandler) streamObject(
	ctx context.Context,
	w http.ResponseWriter,
	r *http.Request,
	presignedURL string,
	objectKey string,
) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, presignedURL, nil)
	if err != nil {
		log.Printf("Failed to create request for %s: %v", objectKey, err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	// Forward range header for partial content support
	if rangeHeader := r.Header.Get("Range"); rangeHeader != "" {
		req.Header.Set("Range", rangeHeader)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("Failed to fetch %s: %v", objectKey, err)
		http.Error(w, "Failed to fetch object", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Handle S3 errors
	if resp.StatusCode == http.StatusNotFound {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	if resp.StatusCode >= 400 {
		http.Error(w, "Upstream error", resp.StatusCode)
		return
	}

	// Forward relevant headers
	forwardHeaders := []string{
		"Content-Type",
		"Content-Length",
		"Content-Range",
		"Accept-Ranges",
		"ETag",
		"Last-Modified",
		"Cache-Control",
	}
	for _, header := range forwardHeaders {
		if v := resp.Header.Get(header); v != "" {
			w.Header().Set(header, v)
		}
	}
	w.WriteHeader(resp.StatusCode)

	// Skip body for HEAD requests
	if r.Method == http.MethodHead {
		return
	}

	// Stream the body
	_, err = io.Copy(w, resp.Body)
	if err != nil {
		log.Printf("Error streaming %s: %v", objectKey, err)
	}
}
