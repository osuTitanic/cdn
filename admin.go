package main

import "net/http"

// In the future I want this to contain an API for file management
// There should be different "access keys" for various folder & permission configurations
// Example: Digitalfear117 has an access key for uploading & deleting files off of /public/digitalclient/*
// Optionally, I want to add a simple web ui that makes this whole process easier
// Another thing to consider is cloudflare upload filesize limits, which will require us to implement a chunked upload system

func (h *CdnHandler) AdminRoutes() http.Handler {
	mux := http.NewServeMux()
	// TODO: Implement file management capabilities
	return mux
}
