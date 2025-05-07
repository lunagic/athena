package athena

import (
	"net/http"
	"reflect"
)

type autoRouterConfig struct {
	Prefix          string
	Type            reflect.Type
	argumentMapping map[reflect.Type]func(w http.ResponseWriter, r *http.Request) (reflect.Value, error)
	returnMapping   map[reflect.Type]func(w http.ResponseWriter, r *http.Request, value reflect.Value)
}

func (config autoRouterConfig) Enabled() bool {
	return config.Prefix != ""
}
