package handler

import "testing"

func TestValidImageObjectName(t *testing.T) {
	tests := []struct {
		name  string
		value string
		valid bool
	}{
		{name: "post medium", value: "posts/medium/2026/08/12/image.jpg", valid: true},
		{name: "wrong prefix", value: "avatars/user.jpg", valid: false},
		{name: "traversal", value: "posts/../../secret", valid: false},
		{name: "query", value: "posts/medium/image.jpg?raw=1", valid: false},
		{name: "empty", value: "", valid: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validImageObjectName(tt.value); got != tt.valid {
				t.Fatalf("validImageObjectName(%q) = %t, want %t", tt.value, got, tt.valid)
			}
		})
	}
}
