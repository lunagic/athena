package athena

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/lunagic/athena/athenaservices/cache"
	"github.com/lunagic/athena/athenaservices/database"
	"github.com/lunagic/athena/athenaservices/mailer"
	"github.com/lunagic/athena/athenaservices/queue"
	"github.com/lunagic/athena/athenaservices/storage"
	"github.com/lunagic/athena/athenaservices/vault"
	"github.com/lunagic/environment-go/environment"
)

type Config struct {
	//
	logger *slog.Logger
	// App
	AppHTTPHost string `env:"APP_HTTP_HOST"`
	AppHTTPPort int    `env:"APP_HTTP_PORT"`
	AppKey      string `env:"APP_KEY"`
	// App Drivers
	AppDriverMailer   string `env:"APP_DRIVER_MAILER"`
	AppDriverStorage  string `env:"APP_DRIVER_STORAGE"`
	AppDriverDatabase string `env:"APP_DRIVER_DATABASE"`
	AppDriverCache    string `env:"APP_DRIVER_CACHE"`
	AppDriverQueue    string `env:"APP_DRIVER_QUEUE"`
	// Services
	AmazonS3AccessKeyID     string `env:"AMAZON_S3_ACCESS_KEY_ID"`
	AmazonS3AccessKeySecret string `env:"AMAZON_S3_ACCESS_KEY_SECRET"`
	AmazonS3Bucket          string `env:"AMAZON_S3_BUCKET"`
	AmazonS3Endpoint        string `env:"AMAZON_S3_ENDPOINT"`
	AmazonS3Region          string `env:"AMAZON_S3_REGION"`
	MySQLHost               string `env:"MYSQL_HOST"`
	MySQLName               string `env:"MYSQL_NAME"`
	MySQLPass               string `env:"MYSQL_PASS"`
	MySQLPort               int    `env:"MYSQL_PORT"`
	MySQLUser               string `env:"MYSQL_USER"`
	PostgresHost            string `env:"POSTGRES_HOST"`
	PostgresName            string `env:"POSTGRES_NAME"`
	PostgresPass            string `env:"POSTGRES_PASS"`
	PostgresPort            int    `env:"POSTGRES_PORT"`
	PostgresUser            string `env:"POSTGRES_USER"`
	RabbitMQHost            string `env:"RABBITMQ_HOST"`
	RabbitMQPass            string `env:"RABBITMQ_PASS"`
	RabbitMQPort            int    `env:"RABBITMQ_PORT"`
	RabbitMQUser            string `env:"RABBITMQ_USER"`
	RedisHost               string `env:"REDIS_HOST"`
	RedisNumber             int    `env:"REDIS_NUMBER"`
	RedisPass               string `env:"REDIS_PASS"`
	RedisPort               int    `env:"REDIS_PORT"`
	RedisUser               string `env:"REDIS_USER"`
	SMTPHost                string `env:"SMTP_HOST"`
	SMTPName                string `env:"SMTP_NAME"`
	SMTPPass                string `env:"SMTP_PASS"`
	SMTPPort                int    `env:"SMTP_PORT"`
	SMTPUser                string `env:"SMTP_USER"`
	SMTPUsername            string `env:"SMTP_USER"`
	SQLitePath              string `env:"SQLITE_PATH"`
	LocalStoragePath        string `env:"LOCAL_STORAGE_PATH"`
}

func NewDefaultConfig() Config {
	return Config{
		AppDriverCache:    "memory",
		AppDriverDatabase: "sqlite",
		AppDriverMailer:   "smtp",
		AppDriverQueue:    "memory",
		AppDriverStorage:  "local",
		AppHTTPHost:       "0.0.0.0",
		AppHTTPPort:       2291,
		MySQLHost:         "127.0.0.1",
		MySQLPort:         3306,
		PostgresHost:      "127.0.0.1",
		PostgresPort:      5432,
		RabbitMQHost:      "127.0.0.1",
		RabbitMQPort:      5672,
		RedisHost:         "127.0.0.1",
		RedisPort:         6379,
		SMTPHost:          "127.0.0.1",
		SMTPPort:          1025,
		SQLitePath:        "tmp/database.sqlite",
	}
}

func NewTestConfig(t *testing.T) Config {
	config := NewDefaultConfig()
	config.SQLitePath = fmt.Sprintf("%s/database.sqlite", t.TempDir())

	return config
}

func NewConfig() (Config, error) {
	config := NewDefaultConfig()
	if err := environment.New().Decode(&config); err != nil {
		return Config{}, err
	}

	return config, nil
}

func (config Config) Logger() *slog.Logger {
	return config.logger
}
func (config Config) Vault() vault.Vault {
	return vault.New([]byte(config.AppKey))
}

func (config Config) ListenAddr() string {
	return fmt.Sprintf("%s:%d", config.AppHTTPHost, config.AppHTTPPort)
}

func (config Config) Mailer() (mailer.Driver, error) {
	switch config.AppDriverMailer {
	case "smtp":
		return mailer.NewDriverSMTP(mailer.DriverSMTPConfig{
			Host: config.SMTPHost,
			Port: config.SMTPPort,
			User: config.SMTPUser,
			Pass: config.SMTPPass,
			Name: config.SMTPName,
		})
	}

	return nil, fmt.Errorf("invalid mailer driver: %s", config.AppDriverMailer)
}

func (config Config) Storage() (storage.Driver, error) {
	switch config.AppDriverStorage {
	case "local":
		return storage.NewDriverLocal(config.LocalStoragePath, config.Vault())
	case "s3":
		return storage.NewDriverS3(storage.S3Config{
			Endpoint:        config.AmazonS3Endpoint,
			Region:          config.AmazonS3Region,
			Bucket:          config.AmazonS3Bucket,
			AccessKeyID:     config.AmazonS3AccessKeyID,
			AccessKeySecret: config.AmazonS3AccessKeySecret,
		})
	}

	return nil, fmt.Errorf("invalid storage driver: %s", config.AppDriverStorage)
}

func (config Config) Database(configFuncs ...database.ServiceConfigFunc) (*database.Service, error) {
	switch config.AppDriverDatabase {
	case "sqlite":
		if err := os.MkdirAll(filepath.Dir(config.SQLitePath), 0755); err != nil {
			return nil, err
		}

		return database.New(
			database.NewDriverSQLite(config.SQLitePath),
			configFuncs...,
		)
	case "postgres":
		return database.New(
			database.NewDriverPostgres(database.DriverPostgresConfig{
				Host: config.PostgresHost,
				Port: config.PostgresPort,
				User: config.PostgresUser,
				Pass: config.PostgresPass,
				Name: config.PostgresName,
			}),
			configFuncs...,
		)
	case "mysql":
		return database.New(
			database.NewDriverMySQL(database.DriverMySQLConfig{
				Host: config.MySQLHost,
				Port: config.MySQLPort,
				User: config.MySQLUser,
				Pass: config.MySQLPass,
				Name: config.MySQLName,
			}),
			configFuncs...,
		)
	}

	return nil, fmt.Errorf("invalid database driver: %s", config.AppDriverDatabase)
}

func (config Config) Cache() (cache.Driver, error) {
	switch config.AppDriverCache {
	case "memory":
		return cache.NewDriverMemory()
	case "redis":
		return cache.NewDriverRedis(cache.DriverRedisConfig{
			Host:   config.RedisHost,
			Number: config.RedisNumber,
			Pass:   config.RedisPass,
			Port:   config.RedisPort,
			User:   config.RedisUser,
		})
	}

	return nil, fmt.Errorf("invalid cache driver: %s", config.AppDriverCache)
}

func (config Config) Queue() (queue.Driver, error) {
	switch config.AppDriverQueue {
	case "memory":
		return queue.NewDriverMemory()
	case "kafka":
		return nil, fmt.Errorf("queue driver not yet implemented: %s", config.AppDriverQueue)
	case "rabbitmq":
		return queue.NewDriverRabbitMQ(queue.DriverRabbitMQConfig{
			Host: config.RabbitMQHost,
			Pass: config.RabbitMQPass,
			Port: config.RabbitMQPort,
			User: config.RabbitMQUser,
		})
	}

	return nil, fmt.Errorf("invalid queue driver: %s", config.AppDriverQueue)
}
