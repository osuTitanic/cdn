package main

import (
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func (h *CdnHandler) HandleAdminDelete(w http.ResponseWriter, r *http.Request) {
	accessKey, ok := accessKeyFromContext(r.Context())
	if !ok {
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "admin session context is missing")
		return
	}

	if err := requirePermission(accessKey, adminPermissionDelete); err != nil {
		writeAdminError(w, http.StatusForbidden, "forbidden", err.Error())
		return
	}

	objectKey, err := objectKeyFromRequestPath(r.URL.Path)
	if err != nil {
		writeAdminError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if !accessKey.AllowsPath(objectKey) {
		writeAdminError(w, http.StatusForbidden, "forbidden", "delete permission is required for this path")
		return
	}

	ctx, cancel := timeoutContext(r.Context())
	defer cancel()

	_, err = h.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(h.config.S3BucketName),
		Key:    aws.String(objectKey),
	})
	if err != nil && !isNotFoundError(err) {
		writeAdminError(w, http.StatusBadGateway, "storage_error", "failed to delete object")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
