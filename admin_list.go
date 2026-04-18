package main

import (
	"net/http"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

const (
	defaultAdminListLimit = 100
	maxAdminListLimit     = 1000
)

func (h *CdnHandler) HandleAdminList(w http.ResponseWriter, r *http.Request) {
	accessKey, ok := accessKeyFromContext(r.Context())
	if !ok {
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "admin session context is missing")
		return
	}

	if err := requirePermission(accessKey, adminPermissionList); err != nil {
		writeAdminError(w, http.StatusForbidden, "forbidden", err.Error())
		return
	}

	prefixValues, ok := r.URL.Query()["prefix"]
	if !ok || len(prefixValues) == 0 {
		writeAdminError(w, http.StatusBadRequest, "bad_request", "prefix query parameter is required")
		return
	}

	prefix, err := normalizePrefix(prefixValues[0])
	if err != nil {
		writeAdminError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if !accessKey.AllowsPath(prefix) {
		writeAdminError(w, http.StatusForbidden, "forbidden", "list permission is not allowed for this prefix")
		return
	}
	limit := defaultAdminListLimit

	if rawLimit := r.URL.Query().Get("limit"); rawLimit != "" {
		parsedLimit, err := strconv.Atoi(rawLimit)
		if err != nil || parsedLimit <= 0 {
			writeAdminError(w, http.StatusBadRequest, "bad_request", "limit must be a positive integer")
			return
		}

		limit = max(1, min(parsedLimit, maxAdminListLimit))
	}

	input := &s3.ListObjectsV2Input{
		Bucket:  aws.String(h.config.S3BucketName),
		MaxKeys: aws.Int32(int32(limit)),
		Prefix:  aws.String(prefix),
	}
	if cursor := r.URL.Query().Get("cursor"); cursor != "" {
		input.ContinuationToken = aws.String(cursor)
	}

	ctx, cancel := timeoutContext(r.Context())
	defer cancel()

	output, err := h.client.ListObjectsV2(ctx, input)
	if err != nil {
		writeAdminError(w, http.StatusBadGateway, "storage_error", "failed to list objects")
		return
	}

	items := make([]adminListItem, 0, len(output.Contents))
	for _, object := range output.Contents {
		item := adminListItem{
			Key:  aws.ToString(object.Key),
			ETag: aws.ToString(object.ETag),
		}
		if object.Size != nil {
			item.Size = *object.Size
		}
		if object.LastModified != nil {
			item.LastModified = object.LastModified.UTC().Format(time.RFC3339)
		}
		items = append(items, item)
	}

	writeAdminJson(w, http.StatusOK, adminListResponse{
		Prefix:     prefix,
		Items:      items,
		NextCursor: aws.ToString(output.NextContinuationToken),
	})
}
