package tasks

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/qalibre/qalibre/internal/appdb"
)

func s3Configured(d *appdb.Dataset) bool {
	return strings.TrimSpace(d.S3Endpoint) != "" && strings.TrimSpace(d.S3Bucket) != ""
}

func uploadFileToS3(ctx context.Context, d *appdb.Dataset, localFilePath string, s3Key string) error {
	endpoint := strings.TrimSpace(d.S3Endpoint)
	bucket := strings.TrimSpace(d.S3Bucket)
	region := strings.TrimSpace(d.S3Region)
	accessKey := strings.TrimSpace(d.S3AccessKey)
	secretKey := strings.TrimSpace(d.S3SecretKey)

	if endpoint == "" || bucket == "" {
		return nil
	}

	// Read file content
	file, err := os.Open(localFilePath)
	if err != nil {
		return fmt.Errorf("failed to open file for S3 upload: %w", err)
	}
	defer file.Close()

	if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		if d.S3UseSSL {
			endpoint = "https://" + endpoint
		} else {
			endpoint = "http://" + endpoint
		}
	}

	// Initialize credentials
	creds := credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")

	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(region),
		config.WithCredentialsProvider(creds),
	)
	if err != nil {
		return fmt.Errorf("failed to load S3 SDK config: %w", err)
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = d.S3ForcePathStyle
	})

	slog.Info("Uploading file to S3", "localFilePath", localFilePath, "bucket", bucket, "key", s3Key, "endpoint", endpoint)

	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(s3Key),
		Body:   file,
	})
	if err != nil {
		return fmt.Errorf("failed to upload object to S3: %w", err)
	}

	slog.Info("Successfully uploaded file to S3", "key", s3Key)
	return nil
}
