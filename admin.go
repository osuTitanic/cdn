package main

import "net/http"

func (h *CdnHandler) AdminRoutes() http.Handler {
	mux := http.NewServeMux()
	// TODO: Implement file management capabilities
	return mux
}
