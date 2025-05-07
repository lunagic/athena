package utils

import "reflect"

func LoopOverStructFields(value reflect.Value, fieldHandler func(fieldDefinition reflect.StructField, fieldValue reflect.Value) error) error {
	if value.Kind() == reflect.Pointer {
		value = value.Elem()
	}

	for i := range value.NumField() {
		fieldValue := value.Field(i)
		fieldDefinition := value.Type().Field(i)

		if !fieldDefinition.IsExported() {
			continue
		}

		if err := fieldHandler(fieldDefinition, fieldValue); err != nil {
			return err
		}
	}

	return nil
}
