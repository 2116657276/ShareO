package service

import (
	"testing"
)

func TestClampPage(t *testing.T) {
	tests := []struct {
		name         string
		page         int
		pageSize     int
		wantPage     int
		wantPageSize int
	}{
		{"zero_page_clamped", 0, 20, 1, 20},
		{"zero_both_clamped", 0, 0, 1, 20},
		{"negative_page_clamped", -1, 20, 1, 20},
		{"negative_pageSize_clamped", 1, -5, 1, 20},
		{"over_max_pageSize", 1, 100, 1, 20},
		{"exact_max_pageSize", 1, 50, 1, 50},
		{"normal_values", 3, 12, 3, 12},
		{"edge_pageSize_below_min_clamped_to_default", 1, 1, 1, 1},
		{"large_page", 100, 20, 100, 20},
		{"pageSize_at_max_boundary", 1, 50, 1, 50},
		{"pageSize_above_max_boundary", 1, 51, 1, 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPage, gotPageSize := clampPage(tt.page, tt.pageSize)
			if gotPage != tt.wantPage {
				t.Errorf("clampPage(%d, %d) page = %d, want %d", tt.page, tt.pageSize, gotPage, tt.wantPage)
			}
			if gotPageSize != tt.wantPageSize {
				t.Errorf("clampPage(%d, %d) pageSize = %d, want %d", tt.page, tt.pageSize, gotPageSize, tt.wantPageSize)
			}
		})
	}
}
