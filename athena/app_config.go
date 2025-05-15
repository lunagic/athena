package athena

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"log/slog"
	"net/http"
	"reflect"

	"github.com/lunagic/athena/athena/internal/agenda"
	"github.com/lunagic/athena/athenaservices/cache"
	"github.com/lunagic/athena/athenaservices/database"
	"github.com/lunagic/athena/athenaservices/queue"
	"github.com/lunagic/poseidon/poseidon"
)

type AppConfigFunc func(service *App) error

func WithLogger(logger *slog.Logger) AppConfigFunc {
	return func(app *App) error {
		app.logger = logger

		return nil
	}
}

func WithBackgroundJobs(cacheDriver cache.Driver, jobs []*BackgroundJob) AppConfigFunc {
	return func(service *App) error {
		service.jobs = func() []agenda.Job {
			x := []agenda.Job{}
			for _, j := range jobs {
				x = append(x, j)
			}
			return x
		}()
		service.jobCacheDriver = cacheDriver
		return nil
	}
}

func WithHandler(path string, handler http.Handler) AppConfigFunc {
	return func(app *App) error {
		app.handlers[path] = handler
		return nil
	}
}

func WithMiddlewares(middlewares poseidon.Middlewares) AppConfigFunc {
	return func(service *App) error {
		service.middlewares = middlewares

		return nil
	}
}

func WithDatabaseAutoMigration(db *database.Service, entities []database.Entity) AppConfigFunc {
	return func(app *App) error {
		app.database = db
		app.databaseAutoMigrationEntities = entities

		return nil
	}
}

func WithTypeScriptOutput(writer io.Writer, mapping map[string]reflect.Type) AppConfigFunc {
	return func(app *App) error {
		app.typeScript.fileWriter = writer
		app.typeScript.typesMap = mapping

		return nil
	}
}

func WithQueue[T any](
	ctx context.Context,
	q queue.Queue[T],
	handler func(
		ctx context.Context,
		payload T,
	) error,
) AppConfigFunc {
	return func(service *App) error {
		go func() {
			_ = q.Consume(ctx, handler)
		}()
		return nil
	}
}

func WithRouter[T any](prefix string, router T, middlewares ...poseidon.Middleware) AppConfigFunc {
	return func(app *App) error {
		app.autoRouter.Type = reflect.TypeFor[T]()
		app.autoRouter.Prefix = prefix

		return WithHandler(
			app.autoRouter.Prefix,
			poseidon.Middlewares(middlewares).Apply(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					methodString := r.URL.Query().Get(autoRouterQueryParamName)

					// Confirm that the method name is in the interface
					methodDef, found := app.autoRouter.Type.MethodByName(methodString)
					if !found {
						http.NotFound(w, r)
						return
					}

					// Get the method from the instance provided
					method := reflect.ValueOf(router).MethodByName(methodString)

					in := []reflect.Value{}
					for inIndex := range methodDef.Type.NumIn() {
						inType := methodDef.Type.In(inIndex)

						if inType == app.autoRouter.Type {
							continue
						}

						if inType.Implements(reflect.TypeFor[http.ResponseWriter]()) {
							in = append(in, reflect.ValueOf(w))
							continue
						}

						if inType == reflect.TypeFor[*http.Request]() {
							in = append(in, reflect.ValueOf(r))
							continue
						}

						overrideFunc, found := app.autoRouter.argumentMapping[inType]
						if found {
							value, err := overrideFunc(w, r)
							if err != nil {
								return
							}
							in = append(in, value)
							continue
						}

						if r.Method == http.MethodPost && methodDef.Type.In(inIndex) != reflect.TypeFor[any]() {
							payload := reflect.New(methodDef.Type.In(inIndex)).Interface()
							if err := json.NewDecoder(r.Body).Decode(payload); err != nil {
								log.Printf("MagicRouter: json decoding: %s", err)
								http.NotFound(w, r)
								return
							}
							in = append(in, reflect.ValueOf(payload).Elem())
							continue
						}

						in = append(in, reflect.ValueOf(inIndex))
					}

					outArgs := method.Call(in)
					for outIndex := methodDef.Type.NumOut() - 1; outIndex >= 0; outIndex-- {
						out := outArgs[outIndex]
						outType := methodDef.Type.Out(outIndex)

						overrideFunc, found := app.autoRouter.returnMapping[outType]
						if found {
							trackableWriter := poseidon.NewResponseWriter(w)
							overrideFunc(trackableWriter, r, out)
							// If the overrideFunc has written a response we should not continue
							if trackableWriter.Written() {
								return
							}
							continue
						}

						poseidon.RespondJSON(w, http.StatusOK, out.Interface())
						return
					}
				}),
			),
		)(app)
	}
}

func WithRouterArgumentProvider[T any](customArgumentProvider func(w http.ResponseWriter, r *http.Request) (T, error)) AppConfigFunc {
	return func(app *App) error {
		newType := reflect.TypeFor[T]()
		if _, found := app.autoRouter.argumentMapping[newType]; found {
			return errors.New("duplicate CustomArgumentProvider type, it was already registered")
		}

		app.typeScript.argumentTypesToIgnore[newType] = true
		app.autoRouter.argumentMapping[newType] = func(w http.ResponseWriter, r *http.Request) (reflect.Value, error) {
			result, err := customArgumentProvider(w, r)

			return reflect.ValueOf(result), err
		}

		return nil
	}
}

func WithRouterReturnProvider[T any](customReturnProvider func(w http.ResponseWriter, r *http.Request, value T)) AppConfigFunc {
	return func(app *App) error {
		newType := reflect.TypeFor[T]()
		if _, found := app.autoRouter.argumentMapping[newType]; found {
			return errors.New("duplicate CustomReturnProvider type, it was already registered")
		}

		app.typeScript.argumentTypesToIgnore[newType] = true
		app.autoRouter.returnMapping[newType] = func(w http.ResponseWriter, r *http.Request, value reflect.Value) {
			var typedValue T
			i := value.Interface()
			if i != nil {
				typedValue = i.(T)
			}

			customReturnProvider(w, r, typedValue)
		}

		return nil
	}
}
