package repository

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/BrandonVee/TokenRouter/internal/config"
	"github.com/BrandonVee/TokenRouter/internal/pkg/servertiming"
	"github.com/BrandonVee/TokenRouter/internal/service"
	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type imageHistoryS3Store struct {
	client        *s3.Client
	presignClient *s3.PresignClient
	bucket        string
}

// NewImageHistoryS3Store 根据独立配置创建私有生图对象存储；未启用时返回空实现依赖。
func NewImageHistoryS3Store(cfg *config.Config) (service.ImageHistoryObjectStore, error) {
	if cfg == nil || !cfg.ImageHistory.Enabled {
		return nil, nil
	}
	region := strings.TrimSpace(cfg.ImageHistory.Region)
	if region == "" {
		region = "auto"
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.ImageHistory.AccessKeyID,
			cfg.ImageHistory.SecretAccessKey,
			"",
		)),
	)
	if err != nil {
		return nil, fmt.Errorf("load image history S3 config: %w", err)
	}
	client := s3.NewFromConfig(awsCfg, func(options *s3.Options) {
		if endpoint := strings.TrimSpace(cfg.ImageHistory.Endpoint); endpoint != "" {
			options.BaseEndpoint = &endpoint
		}
		options.UsePathStyle = cfg.ImageHistory.ForcePathStyle
		options.APIOptions = append(options.APIOptions, v4.SwapComputePayloadSHA256ForUnsignedPayloadMiddleware)
		options.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
	})
	return &imageHistoryS3Store{
		client:        client,
		presignClient: s3.NewPresignClient(client),
		bucket:        cfg.ImageHistory.Bucket,
	}, nil
}

// NewImageHistoryS3StoreFactory 创建可供页面配置热切换的 S3 存储工厂。
func NewImageHistoryS3StoreFactory() service.ImageHistoryObjectStoreFactory {
	return func(imageCfg config.ImageHistoryConfig) (service.ImageHistoryObjectStore, error) {
		return NewImageHistoryS3Store(&config.Config{ImageHistory: imageCfg})
	}
}

// TestConnection 使用 HeadBucket 验证当前凭据和桶是否可用。
func (s *imageHistoryS3Store) TestConnection(ctx context.Context) error {
	finish := servertiming.ObserveDependency(ctx, "s3")
	_, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: &s.bucket})
	finish()
	if err != nil {
		return fmt.Errorf("S3 HeadBucket image history: %w", err)
	}
	return nil
}

func (s *imageHistoryS3Store) Put(ctx context.Context, key, contentType string, data []byte) error {
	size := int64(len(data))
	finish := servertiming.ObserveDependency(ctx, "s3")
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        &s.bucket,
		Key:           &key,
		Body:          bytes.NewReader(data),
		ContentLength: &size,
		ContentType:   &contentType,
	})
	finish()
	if err != nil {
		return fmt.Errorf("S3 PutObject image history: %w", err)
	}
	return nil
}

func (s *imageHistoryS3Store) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	finish := servertiming.ObserveDependency(ctx, "s3")
	result, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: &s.bucket, Key: &key})
	finish()
	if err != nil {
		return nil, fmt.Errorf("S3 GetObject image history: %w", err)
	}
	return result.Body, nil
}

func (s *imageHistoryS3Store) Delete(ctx context.Context, key string) error {
	finish := servertiming.ObserveDependency(ctx, "s3")
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: &s.bucket, Key: &key})
	finish()
	if err != nil {
		return fmt.Errorf("S3 DeleteObject image history: %w", err)
	}
	return nil
}

func (s *imageHistoryS3Store) PresignGet(ctx context.Context, key string, expiry time.Duration) (string, error) {
	result, err := s.presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: &s.bucket,
		Key:    &key,
	}, s3.WithPresignExpires(expiry))
	if err != nil {
		return "", fmt.Errorf("presign image history object: %w", err)
	}
	return result.URL, nil
}
