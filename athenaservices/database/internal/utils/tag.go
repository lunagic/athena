package utils

import (
	"reflect"
	"strings"
)

type DBTag struct {
	Column                 string
	ReadOnly               bool
	PrimaryKey             bool
	AutoIncrement          bool
	TypeOverride           string
	ForeignKeyTargetTable  string
	ForeignKeyTargetColumn string
	Default                string
	Comment                string
	HasDefault             bool
}

func ParseTag(tagString reflect.StructTag) DBTag {
	parts := strings.Split(tagString.Get("db"), ",")

	tag := DBTag{}

	for i, part := range parts {
		if i == 0 {
			tag.Column = part
			continue
		}

		if part == "readOnly" {
			tag.ReadOnly = true

			continue
		}

		if part == "primaryKey" {
			tag.PrimaryKey = true

			continue
		}
		if part == "autoIncrement" {
			tag.AutoIncrement = true

			continue
		}

		if strings.HasPrefix(part, "default=") {
			tag.Default = strings.TrimPrefix(part, "default=")
			tag.HasDefault = true

			continue
		}

		if strings.HasPrefix(part, "comment=") {
			tag.Comment = strings.TrimPrefix(part, "comment=")

			continue
		}

		if strings.HasPrefix(part, "foreignKey=") {
			parts := strings.Split(strings.TrimPrefix(part, "foreignKey="), ".")
			if len(parts) == 2 {
				tag.ForeignKeyTargetTable = parts[0]
				tag.ForeignKeyTargetColumn = parts[1]
			}
			continue
		}
	}

	return tag
}
