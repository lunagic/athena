package athena

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"reflect"

	"github.com/lunagic/poseidon/poseidon"
	"github.com/lunagic/typescript-go/typescript"
)

func WithRouter[T any](prefix string, router T, middlewares ...poseidon.Middleware) ConfigurationFunc {
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

						if inType == reflect.TypeFor[context.Context]() {
							in = append(in, reflect.ValueOf(r.Context()))
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

func WithRouterArgumentProvider[T any](customArgumentProvider func(w http.ResponseWriter, r *http.Request) (T, error)) ConfigurationFunc {
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

func WithRouterReturnProvider[T any](customReturnProvider func(w http.ResponseWriter, r *http.Request, value T)) ConfigurationFunc {
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

func (app *App) routerTypeScriptRoutes() map[string]typescript.Route {
	routes := map[string]typescript.Route{}
	if app.autoRouter.Prefix != "" {
		for methodIndex := range app.autoRouter.Type.NumMethod() {
			method := app.autoRouter.Type.Method(methodIndex)
			if !method.IsExported() {
				continue
			}

			if method.Type.NumOut() == 0 {
				continue
			}

			httpMethod := "GET"
			var httpRequest reflect.Type

			for inIndex := range method.Type.NumIn() {
				in := method.Type.In(inIndex)

				// For struct methods (compared to interface methods),
				// index 0 is the receiver. The "actual" parameters you pass
				// when calling the method start at index 1.
				if in == app.autoRouter.Type {
					continue
				}

				if _, found := app.autoRouter.argumentMapping[in]; found {
					continue
				}

				if in.Implements(reflect.TypeFor[http.ResponseWriter]()) {
					continue
				}

				if in == reflect.TypeFor[*http.Request]() {
					continue
				}

				httpMethod = http.MethodPost
				httpRequest = method.Type.In(inIndex)
				break
			}

			routes[method.Name] = typescript.Route{
				Path:         fmt.Sprintf("%s?%s=%s", app.autoRouter.Prefix, autoRouterQueryParamName, method.Name),
				Method:       httpMethod,
				RequestBody:  httpRequest,
				ResponseBody: method.Type.Out(0),
			}
		}
	}

	return routes
}

const autoRouterQueryParamName = "method"

type autoRouterConfig struct {
	Prefix          string
	Type            reflect.Type
	argumentMapping map[reflect.Type]func(w http.ResponseWriter, r *http.Request) (reflect.Value, error)
	returnMapping   map[reflect.Type]func(w http.ResponseWriter, r *http.Request, value reflect.Value)
}

func (config autoRouterConfig) Enabled() bool {
	return config.Prefix != ""
}
