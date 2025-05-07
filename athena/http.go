package athena

import "net/http"

func (service *App) buildHandler() error {
	mux := http.NewServeMux()

	// Add all the handlers
	for path, handler := range service.handlers {
		mux.Handle(path, handler)
	}

	service.httpHandler = mux

	return nil
}
