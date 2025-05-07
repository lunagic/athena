package database

// ================================================================
// ================================================================
// ============================= DONE =============================
// ================================================================
// ================================================================

import (
	"context"
	"database/sql"
	"log/slog"
)

type ServiceConfigFunc func(service *Service) error

func WithPostConnectFunc(callback func(db *sql.DB) error) ServiceConfigFunc {
	return func(service *Service) error {
		return callback(service.standardLibraryDB)
	}
}

func WithPreRunFunc(preRunFunc func(ctx context.Context, statement string, args []any) error) ServiceConfigFunc {
	return func(service *Service) error {
		service.preRunFuncs = append(service.preRunFuncs, preRunFunc)
		return nil
	}
}

func WithPostRunFunc(postRunFunc func(ctx context.Context) error) ServiceConfigFunc {
	return func(service *Service) error {
		service.postRunFuncs = append(service.postRunFuncs, postRunFunc)
		return nil
	}
}

func WithLogger(logger *slog.Logger) ServiceConfigFunc {
	return func(service *Service) error {
		service.preRunFuncs = append(service.preRunFuncs, func(ctx context.Context, statement string, args []any) error {
			logger.Info("Database Run",
				"statement", statement,
				"args", args,
			)

			return nil
		})
		service.postRunFuncs = append(service.postRunFuncs, func(ctx context.Context) error {
			return nil
		})
		return nil
	}
}
