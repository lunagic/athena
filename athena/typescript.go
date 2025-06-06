package athena

import (
	"io"
	"reflect"

	"github.com/lunagic/typescript-go/typescript"
)

func WithTypeScriptOutput(namespace string, writer io.Writer, mapping map[string]reflect.Type) ConfigurationFunc {
	return func(app *App) error {
		app.typeScript.namespace = namespace
		app.typeScript.fileWriter = writer
		app.typeScript.typesMap = mapping

		return nil
	}
}

type typeScriptConfig struct {
	namespace             string
	fileWriter            io.Writer
	typesMap              map[string]reflect.Type
	argumentTypesToIgnore map[reflect.Type]bool
}

func (config typeScriptConfig) Enabled() bool {
	return config.fileWriter != nil
}

func (app *App) calculateTypeScript() error {
	if !app.typeScript.Enabled() {
		return nil
	}

	ts := typescript.New(
		typescript.WithCustomNamespace(app.typeScript.namespace),
		typescript.WithTypes(app.typeScript.typesMap),
		typescript.WithRoutes(app.routerTypeScriptRoutes()),
	)

	return ts.Generate(app.typeScript.fileWriter)
}
