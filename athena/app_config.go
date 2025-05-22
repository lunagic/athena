package athena

import (
	"log/slog"
)

type ConfigurationFunc func(app *App) error

func WithLogger(logger *slog.Logger) ConfigurationFunc {
	return func(app *App) error {
		app.logger = logger

		return nil
	}
}
