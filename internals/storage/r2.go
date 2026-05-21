package storage

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type R2Client struct {
	bucket    string
	publicURL string
	client    *s3.Client
}

type UploadedObject struct {
	Key string
	URL string
}

func NewR2Client(ctx context.Context) (*R2Client, error) {
	accountID := os.Getenv("R2_ACCOUNT_ID")
	accessKeyID := os.Getenv("R2_ACCESS_KEY_ID")
	secretAccessKey := os.Getenv("R2_SECRET_ACCESS_KEY")
	bucket := os.Getenv("R2_BUCKET")
	publicURL := strings.TrimRight(os.Getenv("R2_PUBLIC_URL"), "/")

	if accountID == "" || accessKeyID == "" || secretAccessKey == "" || bucket == "" || publicURL == "" {
		return nil, fmt.Errorf("R2_ACCOUNT_ID, R2_ACCESS_KEY_ID, R2_SECRET_ACCESS_KEY, R2_BUCKET, and R2_PUBLIC_URL are required")
	}

	cfg, err := config.LoadDefaultConfig(
		ctx,
		config.WithRegion("auto"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, "")),
	)
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("https://%s.r2.cloudflarestorage.com", accountID)
	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
	})

	return &R2Client{bucket: bucket, publicURL: publicURL, client: client}, nil
}

func (r *R2Client) Upload(ctx context.Context, key string, body []byte, mediaType string) (*UploadedObject, error) {
	_, err := r.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(r.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(body),
		ContentType: aws.String(mediaType),
	})
	if err != nil {
		return nil, err
	}

	return &UploadedObject{Key: key, URL: r.publicObjectURL(key)}, nil
}

func (r *R2Client) publicObjectURL(key string) string {
	return fmt.Sprintf("%s/%s", r.publicURL, strings.TrimLeft(key, "/"))
}
