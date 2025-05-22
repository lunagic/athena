package athena

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/lunagic/poseidon/poseidon"
)

func WithHandler(path string, handler http.Handler) ConfigurationFunc {
	return func(app *App) error {
		app.handlers[path] = handler
		return nil
	}
}

func WithMiddlewares(middlewares poseidon.Middlewares) ConfigurationFunc {
	return func(app *App) error {
		app.middlewares = middlewares

		return nil
	}
}

// Serve the application over HTTP
func (app *App) Serve(ctx context.Context) error {
	listener, err := net.Listen("tcp", app.config.ListenAddr())
	if err != nil {
		return err
	}

	app.logger.Info(
		"Server Listen on HTTP",
		"addr", fmt.Sprintf("http://%s", strings.ReplaceAll(listener.Addr().String(), "[::]", "0.0.0.0")),
	)

	return (&http.Server{
		Handler: app.Handler(),
		Addr:    app.config.ListenAddr(),
	}).Serve(listener)
}

func (app App) Handler() http.Handler {
	mux := http.NewServeMux()

	// Add all the handlers
	for path, handler := range app.handlers {
		mux.Handle(path, handler)
	}

	return app.middlewares.Apply(mux)
}
