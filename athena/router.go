package athena

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"

	"github.com/lunagic/poseidon/poseidon"
	"github.com/lunagic/typescript-go/typescript"
)

type Validator interface {
	Validate(r *http.Request) error
}

func WithRouter[T any](
	prefix string,
	router T,
	errorHandler func(w http.ResponseWriter, r *http.Request, err error),
	middlewares ...poseidon.Middleware,
) ConfigurationFunc {
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

						// For struct methods (compared to interface methods),
						// index 0 is the receiver. The "actual" parameters you pass
						// when calling the method start at index 1.
						if inType == app.autoRouter.Type {
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
								errorHandler(w, r, err)
								return
							}

							payloadThatCanBeValidated, ok := payload.(Validator)
							if ok {
								if err := payloadThatCanBeValidated.Validate(r); err != nil {
									errorHandler(w, r, err)
									return
								}
							}

							in = append(in, reflect.ValueOf(payload).Elem())
							continue
						}

						in = append(in, reflect.ValueOf(inIndex))
					}

					outArgs := method.Call(in)
					if len(outArgs) > 1 {
						errorValue := outArgs[1]
						errorType := methodDef.Type.Out(1)
						if errorType != reflect.TypeFor[error]() {
							panic("second return type must be error")
						}
						errAny := errorValue.Interface()
						if errAny != nil {
							errorHandler(w, r, errAny.(error))
							return
						}
					}

					poseidon.RespondJSON(w, http.StatusOK, outArgs[0].Interface())

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
}

func (config autoRouterConfig) Enabled() bool {
	return config.Prefix != ""
}
