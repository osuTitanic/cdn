package main

import (
	"context"
	"log"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type S3ObjectStore interface {
	ListObjectsV2(ctx context.Context, params *s3.ListObjectsV2Input, options ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
	PutObject(ctx context.Context, params *s3.PutObjectInput, options ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	DeleteObject(ctx context.Context, params *s3.DeleteObjectInput, options ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
}

type S3Presigner interface {
	PresignGetObject(ctx context.Context, params *s3.GetObjectInput, options ...func(*s3.PresignOptions)) (*v4.PresignedHTTPRequest, error)
}

type CdnHandler struct {
	keys   map[string]AccessKey
	config *Config
	router http.Handler

	client    S3ObjectStore
	presigner S3Presigner
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
		config:    cfg,
		keys:      make(map[string]AccessKey, len(cfg.AdminAccessKeys)),
		client:    s3Client,
		presigner: s3.NewPresignClient(s3Client),
	}
	for _, accessKey := range cfg.AdminAccessKeys {
		handler.keys[accessKey.AccessKey] = accessKey
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
	mux.Handle("/admin/", http.StripPrefix("/admin", h.AdminRoutes()))
	mux.Handle("/", h.CdnRoutes())
	return h.logRequests(mux)
}

func (h *CdnHandler) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s %s [%s] (%s)", r.Method, r.Host, r.URL.Path, r.RemoteAddr, r.UserAgent())
		next.ServeHTTP(w, r)
	})
}
