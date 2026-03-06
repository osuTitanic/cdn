package main

import "net/http"

func (h *CdnHandler) StatusRoutes() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/health", http.HandlerFunc(h.HandleHealth))
	return mux
}

func (h *CdnHandler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("we are gaming"))
}
