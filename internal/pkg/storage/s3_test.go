package storage

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/nassabiq/go-chi-boilerplate/internal/config"
)

func TestS3Storage_GetPublicURL(t *testing.T) {
	cfg := &config.Config{
		S3: config.S3Config{
			Bucket:        "test-bucket",
			PublicBaseURL: "http://localhost:9000/test-bucket",
		},
	}

	storage, err := NewS3Storage(cfg)
	if err != nil {
		t.Fatalf("unexpected error creating storage: %v", err)
	}

	url := storage.GetPublicURL("avatars/user-1.jpg")
	expected := "http://localhost:9000/test-bucket/avatars/user-1.jpg"
	if url != expected {
		t.Errorf("expected %s, got %s", expected, url)
	}
}

func TestS3Storage_PresignedURLs(t *testing.T) {
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

	storage, err := NewS3Storage(cfg)
	if err != nil {
		t.Fatalf("unexpected error creating storage: %v", err)
	}

	ctx := context.Background()
	key := "uploads/test-file.txt"

	uploadURL, expiresAt, err := storage.GeneratePresignedUploadURL(ctx, key, "text/plain", 10*time.Minute)
	if err != nil {
		t.Fatalf("unexpected error generating presigned upload url: %v", err)
	}

	if !strings.Contains(uploadURL, "test-bucket/uploads/test-file.txt") && !strings.Contains(uploadURL, "uploads/test-file.txt") {
		t.Errorf("upload url should contain file key, got %s", uploadURL)
	}

	if expiresAt.Before(time.Now()) {
		t.Errorf("expiresAt should be in the future, got %v", expiresAt)
	}

	previewURL, _, err := storage.GeneratePresignedPreviewURL(ctx, key, 10*time.Minute)
	if err != nil {
		t.Fatalf("unexpected error generating presigned preview url: %v", err)
	}

	if !strings.Contains(previewURL, "uploads/test-file.txt") {
		t.Errorf("preview url should contain file key, got %s", previewURL)
	}
}
