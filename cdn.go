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
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// This is the part of the project that made it exist in the first place: the s3 proxy
// My requirements for it were:
// - Provide a direct, streaming file download (cdn.titanic.sh)
// - Provide a redirect to pre-signed URLs (s3.titanic.sh)
// This way we ensure that old browsers can access the files without https support

func (h *CdnHandler) CdnRoutes() http.Handler {
	return http.HandlerFunc(h.HandleDownloadRequest)
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
