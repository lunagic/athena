package athena_test

import (
	"testing"

	"github.com/lunagic/athena/athena"
)

func startApp(t *testing.T, app *athena.App) {
	go func() {

		if err := app.Start(t.Context()); err != nil {
			panic(err)
		}
	}()
}
