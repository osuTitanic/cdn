package main

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func (h *CdnHandler) HandleAdminUpload(w http.ResponseWriter, r *http.Request) {
	accessKey, ok := accessKeyFromContext(r.Context())
	if !ok {
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "admin session context is missing")
		return
	}

	if err := requirePermission(accessKey, adminPermissionUpload); err != nil {
		writeAdminError(w, http.StatusForbidden, "forbidden", err.Error())
		return
	}

	objectKey, err := objectKeyFromRequestPath(r.URL.Path)
	if err != nil {
		writeAdminError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if !accessKey.AllowsPath(objectKey) {
		writeAdminError(w, http.StatusForbidden, "forbidden", "upload permission is required for this path")
		return
	}

	input := &s3.PutObjectInput{
		Bucket: aws.String(h.config.S3BucketName),
		Key:    aws.String(objectKey),
		Body:   r.Body,
	}
	if r.ContentLength >= 0 {
		input.ContentLength = aws.Int64(r.ContentLength)
	}
	if contentType := r.Header.Get("Content-Type"); contentType != "" {
		input.ContentType = aws.String(contentType)
	}
	if cacheControl := r.Header.Get("Cache-Control"); cacheControl != "" {
		input.CacheControl = aws.String(cacheControl)
	}
	if contentDisposition := r.Header.Get("Content-Disposition"); contentDisposition != "" {
		input.ContentDisposition = aws.String(contentDisposition)
	}

	output, err := h.client.PutObject(r.Context(), input)
	if err != nil {
		writeAdminError(w, http.StatusBadGateway, "storage_error", "failed to upload object")
		return
	}
	go OnFileUploaded(objectKey, accessKey, r)

	writeAdminJson(w, http.StatusOK, adminUploadResponse{
		Key:  objectKey,
		ETag: aws.ToString(output.ETag),
	})
}

func OnFileUploaded(objectKey string, accessKey AccessKey, r *http.Request) {
	if accessKey.UploadCallback == "" {
		return
	}
	params := url.Values{}
	params.Set("key", objectKey)
	params.Set("validation", accessKey.CallbackValidation)

	request, err := http.NewRequest("POST", accessKey.UploadCallback, strings.NewReader(params.Encode()))
	if err != nil {
		return
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("User-Agent", "titanic-cdn")
	http.DefaultClient.Do(request)
}
