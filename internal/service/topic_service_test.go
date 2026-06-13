package service

import (
	"testing"
)

func TestParseInt64(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int64
	}{
		{"zero", "0", 0},
		{"positive", "42", 42},
		{"large", "999999", 999999},
		{"id_path", "123", 123},
		{"name_ascii", "landscape", 0},
		{"name_chinese", "风光", 0},
		{"mixed", "abc123", 0},
		{"empty", "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseInt64(tt.input)
			if got != tt.want {
				t.Errorf("parseInt64(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}
