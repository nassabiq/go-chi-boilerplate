package storageusecase

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/nassabiq/go-chi-boilerplate/internal/config"
	"github.com/nassabiq/go-chi-boilerplate/internal/core/domain/payloads"
	"github.com/nassabiq/go-chi-boilerplate/internal/pkg/storage"
)

func TestStorageUsecase_Validation(t *testing.T) {
	cfg := &config.Config{
		S3: config.S3Config{
			Endpoint:      "http://localhost:9000",
			Region:        "us-east-1",
			AccessKey:     "minioadmin",
			SecretKey:     "minioadmin",
			Bucket:        "test-bucket",
			UsePathStyle:  true,
			PresignExpiry: 15 * time.Minute,
		},
	}

	s3Storage, err := storage.NewS3Storage(cfg)
	if err != nil {
		t.Fatalf("unexpected error creating storage: %v", err)
	}

	v := validator.New()
	uc := NewStorageUsecase(&StorageDependencies{
		Validator: v,
		Storage:   s3Storage,
		Config:    cfg,
		Logger:    slog.Default(),
	})

	ctx := context.Background()

	// Test invalid empty filename for direct upload
	_, err = uc.UploadFile(ctx, "", "text/plain", 10, strings.NewReader("test"), "uploads")
	if err == nil {
		t.Error("expected error for empty filename")
	}

	// Test presign upload payload validation
	_, err = uc.GetPresignedUploadURL(ctx, payloads.PresignUploadRequest{
		Filename:    "", // invalid empty filename
		ContentType: "text/plain",
	})
	if err == nil {
		t.Error("expected validation error for empty filename in presign request")
	}

	// Test valid presign upload request
	res, err := uc.GetPresignedUploadURL(ctx, payloads.PresignUploadRequest{
		Filename:    "document.pdf",
		ContentType: "application/pdf",
		Folder:      "docs",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.UploadURL == "" || res.FileKey == "" {
		t.Errorf("expected non-empty upload url and file key, got %+v", res)
	}
	if !strings.HasPrefix(res.FileKey, "docs/") {
		t.Errorf("expected file key to start with docs/, got %s", res.FileKey)
	}

	// Test presign preview request
	prevRes, err := uc.GetPresignedPreviewURL(ctx, payloads.PresignPreviewRequest{
		FileKey:   res.FileKey,
		ExpiresIn: "30m",
	})
	if err != nil {
		t.Fatalf("unexpected error generating preview url: %v", err)
	}

	if prevRes.PreviewURL == "" {
		t.Errorf("expected non-empty preview url")
	}
}

func TestStorageUsecase_UploadFile_NilBody(t *testing.T) {
	cfg := &config.Config{}
	s3Storage, _ := storage.NewS3Storage(cfg)
	uc := NewStorageUsecase(&StorageDependencies{
		Validator: validator.New(),
		Storage:   s3Storage,
		Config:    cfg,
		Logger:    slog.Default(),
	})

	_, err := uc.UploadFile(context.Background(), "test.txt", "text/plain", 10, nil, "uploads")
	if err == nil {
		t.Error("expected error when body is nil")
	}
}

// ensure io.Reader interface compatibility
var _ io.Reader = (*strings.Reader)(nil)
