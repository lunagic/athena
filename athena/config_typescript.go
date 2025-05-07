package athena

import (
	"fmt"
	"io"
	"net/http"
	"reflect"

	"github.com/lunagic/typescript-go/typescript"
)

type typeScriptConfig struct {
	fileWriter            io.Writer
	typesMap              map[string]reflect.Type
	argumentTypesToIgnore map[reflect.Type]bool
}

func (config typeScriptConfig) Enabled() bool {
	return config.fileWriter != nil
}

func (service *App) calculateTypeScript() error {
	if !service.typeScript.Enabled() {
		return nil
	}

	routes := map[string]typescript.Route{}
	if service.autoRouter.Prefix != "" {
		for methodIndex := range service.autoRouter.Type.NumMethod() {
			method := service.autoRouter.Type.Method(methodIndex)
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
				if in == service.autoRouter.Type {
					continue
				}

				if _, found := service.autoRouter.argumentMapping[in]; found {
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

			routes[""] = typescript.Route{
				Path:         fmt.Sprintf("%s?_method=%s", service.autoRouter.Prefix, "method name"),
				Method:       httpMethod,
				RequestBody:  httpRequest,
				ResponseBody: method.Type.Out(0),
			}
		}
	}

	ts := typescript.New(
		typescript.WithCustomNamespace("Athena"),
		typescript.WithTypes(service.typeScript.typesMap),
		typescript.WithRoutes(
			routes,
		),
	)

	return ts.Generate(service.typeScript.fileWriter)
}
