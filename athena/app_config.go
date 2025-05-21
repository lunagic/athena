package athena

import (
	"log/slog"
	"net/http"

	"github.com/lunagic/poseidon/poseidon"
)

type ConfigurationFunc func(app *App) error

func WithLogger(logger *slog.Logger) ConfigurationFunc {
	return func(app *App) error {
		app.logger = logger

		return nil
	}
}

func WithMiddlewares(middlewares poseidon.Middlewares) ConfigurationFunc {
	return func(app *App) error {
		app.middlewares = middlewares

		return nil
	}
}

func WithHandler(path string, handler http.Handler) ConfigurationFunc {
	return func(app *App) error {
		app.handlers[path] = handler
		return nil
	}
}
