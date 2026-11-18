package model

import (
	"reflect"
	"strings"
	"testing"
)

func TestGormSchemaTagsMatchPostgresRuntime(t *testing.T) {
	cases := []struct {
		model any
		field string
		want  string
	}{
		{User{}, "Role", "type:varchar(10)"},
		{Post{}, "Status", "type:varchar(10)"},
		{Message{}, "Meta", "type:jsonb"},
		{BotTaskOutbox{}, "Status", "type:varchar(10)"},
	}
	for _, tc := range cases {
		field, ok := reflect.TypeOf(tc.model).FieldByName(tc.field)
		if !ok {
			t.Fatalf("%T is missing field %s", tc.model, tc.field)
		}
		gormTag := field.Tag.Get("gorm")
		if !strings.Contains(gormTag, tc.want) {
			t.Errorf("%T.%s gorm tag = %q, want %q", tc.model, tc.field, gormTag, tc.want)
		}
		if strings.Contains(gormTag, "enum(") || strings.Contains(gormTag, "type:json;") {
			t.Errorf("%T.%s retains a MySQL-only type tag: %q", tc.model, tc.field, gormTag)
		}
	}
}
