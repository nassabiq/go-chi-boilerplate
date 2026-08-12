package storage

import (
	"context"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/nassabiq/go-chi-boilerplate/internal/config"
)

type S3Storage struct {
	client        *s3.Client
	presignClient *s3.PresignClient
	bucket        string
	publicBaseURL string
	expiry        time.Duration
}

func NewS3Storage(cfg *config.Config) (*S3Storage, error) {
	s3Cfg := cfg.S3

	customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		if s3Cfg.Endpoint != "" {
			return aws.Endpoint{
				PartitionID:       "aws",
				URL:               s3Cfg.Endpoint,
				SigningRegion:     s3Cfg.Region,
				HostnameImmutable: true,
			}, nil
		}
		return aws.Endpoint{}, &aws.EndpointNotFoundError{}
	})

	ctx := context.Background()
	awsOpts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(s3Cfg.Region),
		awsconfig.WithEndpointResolverWithOptions(customResolver),
	}

	if s3Cfg.AccessKey != "" && s3Cfg.SecretKey != "" {
		awsOpts = append(awsOpts, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(s3Cfg.AccessKey, s3Cfg.SecretKey, ""),
		))
	}

	awsConfig, err := awsconfig.LoadDefaultConfig(ctx, awsOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load aws config: %w", err)
	}

	client := s3.NewFromConfig(awsConfig, func(o *s3.Options) {
		o.UsePathStyle = s3Cfg.UsePathStyle
	})

	presignClient := s3.NewPresignClient(client)

	expiry := s3Cfg.PresignExpiry
	if expiry <= 0 {
		expiry = 15 * time.Minute
	}

	return &S3Storage{
		client:        client,
		presignClient: presignClient,
		bucket:        s3Cfg.Bucket,
		publicBaseURL: s3Cfg.PublicBaseURL,
		expiry:        expiry,
	}, nil
}

func (s *S3Storage) Upload(ctx context.Context, key string, body io.Reader, size int64, contentType string) (string, error) {
	input := &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        body,
		ContentType: aws.String(contentType),
	}
	if size > 0 {
		input.ContentLength = aws.Int64(size)
	}

	_, err := s.client.PutObject(ctx, input)
	if err != nil {
		return "", fmt.Errorf("s3 put object failed: %w", err)
	}

	return s.GetPublicURL(key), nil
}

func (s *S3Storage) GeneratePresignedUploadURL(ctx context.Context, key string, contentType string, expiry time.Duration) (string, time.Time, error) {
	if expiry <= 0 {
		expiry = s.expiry
	}

	expiresAt := time.Now().Add(expiry)
	presignReq, err := s.presignClient.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
	}, s3.WithPresignExpires(expiry))

	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to presign upload url: %w", err)
	}

	return presignReq.URL, expiresAt, nil
}

func (s *S3Storage) GeneratePresignedPreviewURL(ctx context.Context, key string, expiry time.Duration) (string, time.Time, error) {
	if expiry <= 0 {
		expiry = s.expiry
	}

	expiresAt := time.Now().Add(expiry)
	presignReq, err := s.presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(expiry))

	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to presign preview url: %w", err)
	}

	return presignReq.URL, expiresAt, nil
}

func (s *S3Storage) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("failed to delete s3 object: %w", err)
	}

	return nil
}

func (s *S3Storage) GetPublicURL(key string) string {
	if s.publicBaseURL != "" {
		return strings.TrimRight(s.publicBaseURL, "/") + "/" + strings.TrimLeft(key, "/")
	}
	return path.Join(s.bucket, key)
}
