package main

const (
	adminPermissionList   = "list"
	adminPermissionUpload = "upload"
	adminPermissionDelete = "delete"
)

type adminErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

type adminSessionResponse struct {
	Name        string   `json:"name"`
	Prefixes    []string `json:"prefixes"`
	Permissions []string `json:"permissions"`
	UploadMode  string   `json:"upload_mode"`
}

type adminListItem struct {
	Key          string `json:"key"`
	Size         int64  `json:"size"`
	ETag         string `json:"etag"`
	LastModified string `json:"last_modified"`
}

type adminListResponse struct {
	Prefix     string          `json:"prefix"`
	Items      []adminListItem `json:"items"`
	NextCursor string          `json:"next_cursor"`
}

type adminUploadResponse struct {
	Key  string `json:"key"`
	ETag string `json:"etag"`
}
