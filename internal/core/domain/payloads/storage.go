package payloads

type PresignUploadRequest struct {
	Filename    string `json:"filename" validate:"required"`
	ContentType string `json:"content_type" validate:"required"`
	Folder      string `json:"folder"`
}

type PresignPreviewRequest struct {
	FileKey   string `json:"file_key" validate:"required"`
	ExpiresIn string `json:"expires_in"` // e.g. "15m", "1h"
}

type DeleteFileRequest struct {
	FileKey string `json:"file_key" validate:"required"`
}
