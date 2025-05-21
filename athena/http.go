package athena

import (
	"net/http"
)

func (app App) Handler() http.Handler {
	return app.httpHandler
}

func (app *App) buildHandler() error {
	mux := http.NewServeMux()

	// Add all the handlers
	for path, handler := range app.handlers {
		mux.Handle(path, handler)
	}

	app.httpHandler = app.middlewares.Apply(mux)

	return nil
}
