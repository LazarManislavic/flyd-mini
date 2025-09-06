package s3

import (
	"context"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type S3Client struct {
	Client *s3.Client
	Bucket string
}


// NewS3Client creates a new S3 client configured for a public bucket.
// It uses anonymous credentials, sets the specified region, and returns a client
// for interacting with the given S3 bucket.
func NewS3Client(ctx context.Context, bucket string, region string) (*S3Client, error) {
	// Load AWS Config with Anonymous credentials as the bucket is public
	cfg, err := config.LoadDefaultConfig(
		ctx,
		config.WithRegion(region),
		config.WithCredentialsProvider(aws.AnonymousCredentials{}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	return &S3Client{
		Client: s3.NewFromConfig(cfg),
		Bucket: bucket,
	}, nil
}

func (s *S3Client) GetObjectStream(ctx context.Context, key string) (io.ReadCloser, error) {
	resp, err := s.Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.Bucket),
		Key: aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get object %s: %w", key, err)
	}
	return resp.Body, nil
}