// Package main is the entry point to the server. It reads configuration, sets up logging and error handling,
// handles signals from the OS, and starts and stops the server.
package main

import (
	"canvas/messaging"
	"canvas/server"
	"canvas/storage"
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/aws/smithy-go/logging"
	"github.com/maragudk/env"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

func main() {
	os.Exit(start())
}

func start() int {
	_ = env.Load()

	log, err := createLogger("development")

	if err != nil {
		fmt.Println("Error setting up logger: ", err)
		return 1
	}

	defer func() {
		_ = log.Sync()
	}()

	host := env.GetStringOrDefault("HOST", "localhost")
	port := env.GetIntOrDefault("PORT", 8080)

	awsConfig, err := config.LoadDefaultConfig(context.Background(),
		config.WithLogger(createAWSLogAdapter(log)),
		config.WithEndpointResolver(createAWSEndpointResolver()),
	)
	if err != nil {
		log.Info("Error creating AWS config", zap.Error(err))
		return 1
	}

	s := server.New(server.Options{
		Database: createDatabase(log),
		Host:     host,
		Log:      log,
		Port:     port,
		Queue:    createQueue(log, awsConfig),
	})

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	eg, ctx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		if err := s.Start(); err != nil {
			log.Info("Error starting server", zap.Error(err))
			return err
		}
		return nil
	})

	<-ctx.Done()

	eg.Go(func() error {
		if err := s.Stop(); err != nil {
			log.Info("Error stopping server", zap.Error(err))
			return err
		}
		return nil
	})

	if err := eg.Wait(); err != nil {
		return 1
	}

	return 0
}

func createLogger(env string) (*zap.Logger, error) {
	switch env {
	case "production":
		return zap.NewProduction()
	case "development":
		return zap.NewDevelopment()
	default:
		return zap.NewNop(), nil
	}
}

func createAWSLogAdapter(log *zap.Logger) logging.LoggerFunc {
	return func(classification logging.Classification, format string, v ...interface{}) {
		switch classification {
		case logging.Debug:
			log.Sugar().Debugf(format, v...)
		case logging.Warn:
			log.Sugar().Warnf(format, v...)
		}
	}
}

// createAWSEndpointResolver used for local development endpoints.
// See https://aws.github.io/aws-sdk-go-v2/docs/configuring-sdk/endpoints/
func createAWSEndpointResolver() aws.EndpointResolverFunc {
	sqsEndpointURL := env.GetStringOrDefault("SQS_ENDPOINT_URL", "")

	return func(service, region string) (aws.Endpoint, error) {
		if sqsEndpointURL != "" && service == sqs.ServiceID {
			return aws.Endpoint{
				URL: sqsEndpointURL,
			}, nil
		}
		// Fallback to default endpoint
		return aws.Endpoint{}, &aws.EndpointNotFoundError{}
	}
}

func getStringOrDefault(name, defaultV string) string {
	v, ok := os.LookupEnv(name)
	if !ok {
		return defaultV
	}
	return v
}

func getIntOrDefault(name string, defaultV int) int {
	v, ok := os.LookupEnv(name)
	if !ok {
		return defaultV
	}
	vAsInt, err := strconv.Atoi(v)
	if err != nil {
		return defaultV
	}
	return vAsInt
}

func createDatabase(log *zap.Logger) *storage.Database {
	return storage.NewDatabase(storage.NewDatabaseOptions{
		Host:                  env.GetStringOrDefault("DB_HOST", "localhost"),
		Port:                  env.GetIntOrDefault("DB_PORT", 5432),
		User:                  env.GetStringOrDefault("DB_USER", ""),
		Password:              env.GetStringOrDefault("DB_PASSWORD", ""),
		Name:                  env.GetStringOrDefault("DB_NAME", ""),
		MaxOpenConnections:    env.GetIntOrDefault("DB_MAX_OPEN_CONNECTIONS", 10),
		MaxIdleConnections:    env.GetIntOrDefault("DB_MAX_IDLE_CONNECTIONS", 10),
		ConnectionMaxLifetime: env.GetDurationOrDefault("DB_CONNECTION_MAX_LIFETIME", time.Hour),
		Log:                   log,
	})
}

func createQueue(log *zap.Logger, awsConfig aws.Config) *messaging.Queue {
	return messaging.NewQueue(messaging.NewQueueOptions{
		Config:   awsConfig,
		Log:      log,
		Name:     env.GetStringOrDefault("QUEUE_NAME", "jobs"),
		WaitTime: env.GetDurationOrDefault("QUEUE_WAIT_TIME", 20*time.Second),
	})
}
