package configs

import (
	"context"
	"fmt"
	"os"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

var (
	S3Client   *s3.Client
	S3Uploader *manager.Uploader
)

func EnvAWSAccessKey() string {
	return os.Getenv("AWS_ACCESS_KEY_ID")
}

func EnvAWSSecretKey() string {
	return os.Getenv("AWS_SECRET_ACCESS_KEY")
}

func EnvAWSRegion() string {
	region := os.Getenv("AWS_REGION")
	if region == "" {
		return "us-east-1"
	}
	return region
}

func EnvRawBucket() string {
	bucket := os.Getenv("RAW_BUCKET")
	if bucket == "" {
		return "raw-syn-videos"
	}
	return bucket
}

func EnvPicturesBucket() string {
	bucket := os.Getenv("PICTURES_BUCKET")
	if bucket == "" {
		return "syn-pictures"
	}
	return bucket
}


func EnvProcessedBucket() string {
	bucket := os.Getenv("PROCESSED_BUCKET")
	if bucket == "" {
		return "processed-syn-videos"
	}
	return bucket
}

func EnvCDNURL() string {
	cdn := os.Getenv("CDN_URL")
	if cdn == "" {
		return "https://syn-video-cdn.b-cdn.net"
	}
	return cdn
}

// ConnectAWS initializes AWS S3 connection using SDK v2
func ConnectAWS() error {
	ctx := context.Background()

	accessKey := EnvAWSAccessKey()
	secretKey := EnvAWSSecretKey()
	region := EnvAWSRegion()

	fmt.Println("accessKey:", accessKey)
   fmt.Println("secretKey:", secretKey)
    fmt.Println("region:", region)

	if accessKey == "" || secretKey == "" {
		return fmt.Errorf("AWS credentials not found in environment variables")
	}

	// Create AWS config with static credentials
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			accessKey,
			secretKey,
			"", // Token (empty for IAM user)
		)),
	)
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	S3Client = s3.NewFromConfig(cfg)

	S3Uploader = manager.NewUploader(S3Client)

	return nil
}

func GetS3Uploader() *manager.Uploader {
	return S3Uploader
}

func GetS3Client() *s3.Client {
	return S3Client
}