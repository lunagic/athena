package utils

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
)

func Prepare(statement string, parameters map[string]any, numberedParams bool) (string, []any, error) {
	paramFinder := regexp.MustCompile(`(?m):\w+`)
	spaceFinder := regexp.MustCompile(`(?m)\s^\s+`)

	statement = strings.TrimSpace(spaceFinder.ReplaceAllString(statement, " "))

	args := []any{}
	counter := 0
	paramBuilder := func() string {
		counter++
		if !numberedParams {
			return "?"
		}

		return fmt.Sprintf("$%d", counter)
	}

	newStatement := paramFinder.ReplaceAllStringFunc(statement, func(s string) string {
		parameterValue, found := parameters[s]
		if !found {
			return s
		}

		rt := reflect.TypeOf(parameterValue)
		if rt.Kind() == reflect.Array || rt.Kind() == reflect.Slice {
			localArgs := []string{}

			valueOf := reflect.ValueOf(parameterValue)
			for i := range valueOf.Len() {
				localArgs = append(localArgs, paramBuilder())
				args = append(args, valueOf.Index(i).Interface())
			}

			return strings.Join(localArgs, ", ")
		}

		args = append(args, parameterValue)

		return paramBuilder()
	})

	return newStatement, args, nil
}
