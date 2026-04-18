package main

import "net/http"

func (h *CdnHandler) HandleAdminSession(w http.ResponseWriter, r *http.Request) {
	accessKey, ok := accessKeyFromContext(r.Context())
	if !ok {
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "admin session context is missing")
		return
	}

	writeAdminJson(w, http.StatusOK, adminSessionResponse{
		Name:        accessKey.Name,
		Prefixes:    accessKey.Prefixes,
		Permissions: accessKey.Permissions,
	})
}
