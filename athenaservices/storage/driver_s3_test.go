package storage_test

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/lunagic/athena/athenaservices/storage"
	"github.com/lunagic/athena/athenatools"
)

func Test_Driver_S3(t *testing.T) {
	t.Parallel()

	if os.Getenv("ATHENA_TEST_STORAGE_AWS_ACCESS_KEY_SECRET") == "" {
		t.Skip()
	}

	driver, err := storage.NewDriverS3(
		storage.S3Config{
			Region:          os.Getenv("ATHENA_TEST_STORAGE_AWS_REGION"),
			AccessKeyID:     os.Getenv("ATHENA_TEST_STORAGE_AWS_ACCESS_KEY_ID"),
			AccessKeySecret: os.Getenv("ATHENA_TEST_STORAGE_AWS_ACCESS_KEY_SECRET"),
			Bucket:          "athena-dev-private",
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	testSuite(t, driver)
}

func Test_Driver_S3_R2(t *testing.T) {
	t.Parallel()

	if os.Getenv("ATHENA_TEST_STORAGE_R2_ACCOUNT_ID") == "" {
		t.Skip()
	}

	hasher := sha256.New()
	hasher.Write([]byte(os.Getenv("ATHENA_TEST_STORAGE_R2_ACCESS_KEY_SECRET")))
	hashedSecretKey := hex.EncodeToString(hasher.Sum(nil))

	driver, err := storage.NewDriverS3(
		storage.S3Config{
			Endpoint:        fmt.Sprintf("https://%s.r2.cloudflarestorage.com", os.Getenv("ATHENA_TEST_STORAGE_R2_ACCOUNT_ID")),
			AccessKeyID:     os.Getenv("ATHENA_TEST_STORAGE_R2_ACCESS_KEY_ID"),
			AccessKeySecret: hashedSecretKey,
			Bucket:          os.Getenv("ATHENA_TEST_STORAGE_R2_BUCKET"),
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	testSuite(t, driver)
}

func Test_Driver_S3_Minio(t *testing.T) {
	t.Parallel()

	accessKeyID := uuid.NewString()
	accessKeySecret := uuid.NewString()
	bucketName := uuid.NewString()

	driver := athenatools.GetDockerService(
		t,
		athenatools.DockerServiceConfig[storage.Driver]{
			DockerImage:    "bitnami/minio",
			DockerImageTag: "latest",
			InternalPort:   9000,
			Environment: map[string]string{
				"MINIO_ROOT_USER":       accessKeyID,
				"MINIO_ROOT_PASSWORD":   accessKeySecret,
				"MINIO_DEFAULT_BUCKETS": bucketName,
			},
			Builder: func(host string, port int) (storage.Driver, error) {
				driver, err := storage.NewDriverS3(
					storage.S3Config{
						Endpoint:        fmt.Sprintf("http://%s:%d", host, port),
						AccessKeyID:     accessKeyID,
						AccessKeySecret: accessKeySecret,
						Bucket:          bucketName,
					},
				)
				if err != nil {
					return nil, err
				}

				// Check if fully ready
				if err := driver.IsReady(t.Context()); err != nil {
					return nil, err
				}

				return driver, nil
			},
		},
	)

	testSuite(t, driver)
}
