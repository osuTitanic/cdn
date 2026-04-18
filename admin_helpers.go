package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

const adminOperationTimeout = 30 * time.Second

type apiError interface {
	ErrorCode() string
}

func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}

	var apiError apiError
	if !errors.As(err, &apiError) {
		return false
	}

	switch apiError.ErrorCode() {
	case "NoSuchKey", "NotFound", "NoSuchVersion":
		return true
	default:
		return false
	}
}

func adminRouteHandler(method string, next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != method {
			writeAdminError(w, http.StatusMethodNotAllowed, "bad_request", "method not allowed")
			return
		}
		next(w, r)
	})
}

func (h *CdnHandler) adminAuthenticationMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, err := parseBearerToken(r.Header.Get("Authorization"))
		if err != nil {
			writeAdminError(w, http.StatusUnauthorized, "unauthorized", err.Error())
			return
		}

		accessKey, ok := h.keys[token]
		if !ok {
			writeAdminError(w, http.StatusUnauthorized, "unauthorized", "invalid access key")
			return
		}
		ctx := context.WithValue(r.Context(), "accessKey", accessKey)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func parseBearerToken(header string) (string, error) {
	if header == "" {
		return "", errors.New("missing authorization header")
	}
	tokenType, token, ok := strings.Cut(header, " ")

	if !ok || !strings.EqualFold(tokenType, "Bearer") || strings.TrimSpace(token) == "" {
		return "", errors.New("invalid authorization header")
	}

	return strings.TrimSpace(token), nil
}

func accessKeyFromContext(ctx context.Context) (AccessKey, bool) {
	accessKey, ok := ctx.Value("accessKey").(AccessKey)
	return accessKey, ok
}

func writeAdminJson(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(payload)
}

func writeAdminError(w http.ResponseWriter, statusCode int, code string, message string) {
	writeAdminJson(w, statusCode, adminErrorResponse{
		Error:   code,
		Message: message,
	})
}

func normalizeObjectKey(candidate string) (string, error) {
	candidate = strings.TrimSpace(strings.TrimPrefix(candidate, "/"))
	if candidate == "" {
		return "", errors.New("object key is required")
	}
	cleaned := path.Clean(candidate)

	if cleaned == "." || cleaned == "" || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", errors.New("invalid object key")
	}
	return cleaned, nil
}

func normalizePrefix(candidate string) (string, error) {
	candidate = strings.TrimSpace(candidate)
	if candidate == "" || candidate == "/" {
		return "", nil
	}

	candidate = strings.TrimPrefix(candidate, "/")
	cleaned := path.Clean(candidate)
	if cleaned == "." {
		return "", nil
	}
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", errors.New("invalid prefix")
	}

	if !strings.HasSuffix(cleaned, "/") {
		cleaned += "/"
	}

	return cleaned, nil
}

func objectKeyFromRequestPath(requestPath string) (string, error) {
	rawKey := strings.TrimPrefix(requestPath, "/files/")
	if rawKey == requestPath {
		return "", errors.New("invalid admin file path")
	}

	unescaped, err := url.PathUnescape(rawKey)
	if err != nil {
		return "", errors.New("invalid object key")
	}

	return normalizeObjectKey(unescaped)
}

func requirePermission(accessKey AccessKey, permission string) error {
	if accessKey.HasPermission(permission) {
		return nil
	}
	return errors.New(permission + " permission is required for this path")
}

func timeoutContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, adminOperationTimeout)
}
