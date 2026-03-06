package main

import (
	"context"
	"errors"
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
	router    http.Handler
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

	handler := &CdnHandler{
		s3Client:  s3Client,
		presigner: s3.NewPresignClient(s3Client),
		config:    cfg,
	}
	handler.router = handler.Routes()
	return handler, nil
}

func (h *CdnHandler) Router() http.Handler {
	return h.router
}

func (h *CdnHandler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/health", h.StatusRoutes())
	mux.Handle("/admin/", h.AdminRoutes())
	mux.Handle("/", h.ObjectRoutes())
	return h.logRequests(mux)
}

func (h *CdnHandler) ObjectRoutes() http.Handler {
	return http.HandlerFunc(h.HandleDownloadRequest)
}

func (h *CdnHandler) StatusRoutes() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/health", http.HandlerFunc(h.HandleHealth))
	return mux
}

func (h *CdnHandler) AdminRoutes() http.Handler {
	mux := http.NewServeMux()
	// TODO: Implement file management capabilities
	return mux
}

func (h *CdnHandler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("we are gaming"))
}

func (h *CdnHandler) HandleDownloadRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	host := strings.ToLower(r.Host)
	isDirect := strings.HasPrefix(host, "s3.")
	isStream := strings.HasPrefix(host, "cdn.")

	if !isDirect && !isStream {
		http.Error(w, "Invalid host", http.StatusBadRequest)
		return
	}

	objectKey, err := h.objectKeyFromRequestPath(r.URL.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	presignOptions := s3.WithPresignExpires(h.config.PresignExpiry.Duration)
	objectInput := &s3.GetObjectInput{
		Bucket: aws.String(h.config.S3BucketName),
		Key:    aws.String(objectKey),
	}

	presignedReq, err := h.presigner.PresignGetObject(ctx, objectInput, presignOptions)
	if err != nil {
		log.Printf("Failed to presign URL for %s: %v", objectKey, err)
		http.Error(w, "Failed to generate URL", http.StatusInternalServerError)
		return
	}

	if isDirect {
		http.Redirect(w, r, presignedReq.URL, http.StatusTemporaryRedirect)
		return
	}

	h.streamObject(ctx, w, r, presignedReq.URL, objectKey)
}

func (h *CdnHandler) objectKeyFromRequestPath(requestPath string) (string, error) {
	objectKey := strings.TrimPrefix(requestPath, "/")
	if objectKey == "" {
		return "", errors.New("No path specified")
	}

	objectKey = path.Clean(objectKey)
	if strings.HasPrefix(objectKey, "..") || strings.Contains(objectKey, "/../") {
		return "", errors.New("Invalid path")
	}

	if h.config.AllowedPrefix != "" {
		objectKey = strings.TrimPrefix(objectKey, h.config.AllowedPrefix)
		objectKey = h.config.AllowedPrefix + objectKey
	}

	return objectKey, nil
}

func (h *CdnHandler) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s %s [%s] (%s)", r.Method, r.Host, r.URL.Path, r.RemoteAddr, r.UserAgent())
		next.ServeHTTP(w, r)
	})
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

	if resp.StatusCode == http.StatusNotFound {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	if resp.StatusCode >= 400 {
		http.Error(w, "Upstream error", resp.StatusCode)
		return
	}

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

	if r.Method == http.MethodHead {
		// HEAD requests only return headers, no body
		return
	}

	_, err = io.Copy(w, resp.Body)
	if err != nil {
		log.Printf("Error streaming %s: %v", objectKey, err)
	}
}
