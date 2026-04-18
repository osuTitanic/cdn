package main

import "net/http"

func (h *CdnHandler) AdminRoutes() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/session", adminRouteHandler(http.MethodGet, h.HandleAdminSession))
	mux.Handle("/files", adminRouteHandler(http.MethodGet, h.HandleAdminList))

	// /files/{key} for upload & delete
	mux.Handle("/files/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			h.HandleAdminUpload(w, r)
		case http.MethodDelete:
			h.HandleAdminDelete(w, r)
		default:
			writeAdminError(w, http.StatusMethodNotAllowed, "bad_request", "method not allowed")
		}
	}))

	return h.adminAuthenticationMiddleware(mux)
}
