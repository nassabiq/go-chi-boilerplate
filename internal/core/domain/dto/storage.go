package dto

import "time"

type UploadFileResponse struct {
	FileKey     string `json:"file_key"`
	URL         string `json:"url"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
}

type PresignedUploadResponse struct {
	UploadURL string            `json:"upload_url"`
	FileKey   string            `json:"file_key"`
	ExpiresAt time.Time         `json:"expires_at"`
	Headers   map[string]string `json:"headers,omitempty"`
}

type PresignedPreviewResponse struct {
	PreviewURL string    `json:"preview_url"`
	FileKey    string    `json:"file_key"`
	ExpiresAt  time.Time `json:"expires_at"`
}
