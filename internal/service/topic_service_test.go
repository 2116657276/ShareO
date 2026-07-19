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
		// Additional edge cases
		{"negative", "-1", 0}, // parseInt64 only handles positive integers; '-' is non-digit → 0
		{"whitespace", " 123 ", 0},
		{"float", "1.5", 0},
		{"max_int64", "9223372036854775807", 9223372036854775807},
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

func TestParseInt64_NonNumeric(t *testing.T) {
	nonNumeric := []string{"hello", "#tag", "user_name", "abc-def"}
	for _, input := range nonNumeric {
		got := parseInt64(input)
		if got != 0 {
			t.Errorf("parseInt64(%q) = %d, want 0 (non-numeric should return 0)", input, got)
		}
	}
}
