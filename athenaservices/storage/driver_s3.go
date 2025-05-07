package storage

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type S3Config struct {
	Endpoint        string
	Region          string
	AccessKeyID     string
	AccessKeySecret string
	Bucket          string
}

func NewDriverS3(s3Config S3Config) (Driver, error) {
	return &driverS3{
		client: s3.New(
			s3.Options{
				BaseEndpoint: func() *string {
					if s3Config.Endpoint != "" {
						return aws.String(s3Config.Endpoint)
					}

					return nil
				}(),
				UsePathStyle: s3Config.Endpoint != "",
				Region: func() string {
					if s3Config.Endpoint != "" {
						return "auto"
					}

					return s3Config.Region
				}(),
				Credentials: aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
					return aws.Credentials{
						AccessKeyID:     s3Config.AccessKeyID,
						SecretAccessKey: s3Config.AccessKeySecret,
					}, nil
				}),
			},
		),
		bucket: s3Config.Bucket,
	}, nil
}

type driverS3 struct {
	client *s3.Client
	bucket string
}

func (driver *driverS3) Get(ctx context.Context, filePath string) (io.Reader, error) {
	result, err := driver.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(driver.bucket),
		Key:    aws.String(filePath),
	})
	if err != nil {
		return nil, err
	}

	return result.Body, nil
}

func (driver *driverS3) Put(
	ctx context.Context,
	filePath string,
	payload io.Reader,
) error {
	if _, err := driver.client.PutObject(
		ctx,
		&s3.PutObjectInput{
			Bucket: aws.String(driver.bucket),
			Key:    aws.String(filePath),
			Body:   payload,
		},
	); err != nil {
		return err
	}

	return nil
}

func (driver *driverS3) Delete(ctx context.Context, filePath string) error {
	if _, err := driver.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(driver.bucket),
		Key:    aws.String(filePath),
	}); err != nil {
		return err
	}

	return nil
}

func (driver *driverS3) IsReady(ctx context.Context) error {
	if _, err := driver.client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(driver.bucket),
	}); err != nil {
		return err
	}

	return nil
}

func (driver *driverS3) Exists(ctx context.Context, filePath string) (bool, error) {
	if _, err := driver.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(driver.bucket),
		Key:    aws.String(filePath),
	}); err != nil {
		var oe *types.NotFound
		if errors.As(err, &oe) {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

func (driver *driverS3) PreSignedURL(ctx context.Context, filePath string, expiration time.Duration) (string, error) {
	presignClient := s3.NewPresignClient(driver.client, func(po *s3.PresignOptions) {
		po.Expires = time.Minute
	})

	objRequest, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(driver.bucket),
		Key:    aws.String(filePath),
	})
	if err != nil {
		return "", err
	}
	return objRequest.URL, nil
}

func (driver *driverS3) PublicLink(ctx context.Context, filePath string) (string, error) {
	if driver.client.Options().UsePathStyle {
		return *driver.client.Options().BaseEndpoint + "/" + driver.bucket + "/" + filePath, nil
	}

	x, err := driver.client.Options().EndpointResolverV2.ResolveEndpoint(ctx, s3.EndpointParameters{
		Bucket: aws.String(driver.bucket),
		Key:    aws.String(filePath),
		Region: aws.String(driver.client.Options().Region),
	})
	if err != nil {
		return "", err
	}

	// driver.client.Options().BaseEndpoint

	return x.URI.String() + "/" + filePath, nil
}
