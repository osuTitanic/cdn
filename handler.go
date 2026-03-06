package main

import (
	"log"
	"net/http"

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
	mux.Handle("/", h.CdnRoutes())
	return h.logRequests(mux)
}

func (h *CdnHandler) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s %s [%s] (%s)", r.Method, r.Host, r.URL.Path, r.RemoteAddr, r.UserAgent())
		next.ServeHTTP(w, r)
	})
}
