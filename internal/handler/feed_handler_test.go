package handler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizeFeedSort_AllowsOnlyHomeFeedSorts(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "latest", input: "latest", want: "latest"},
		{name: "hot", input: "hot", want: "hot"},
		{name: "unknown", input: "following", want: "latest"},
		{name: "quote payload", input: "' ; alert(1);//", want: "latest"},
		{name: "entity payload", input: "&#39; + alert(1) + &#39;", want: "latest"},
		{name: "whitespace", input: "  HOT  ", want: "hot"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeFeedSort(tt.input); got != tt.want {
				t.Fatalf("normalizeFeedSort(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFeedTemplateDoesNotInterpolateSortIntoExecutableExpression(t *testing.T) {
	path := filepath.Join("..", "..", "web", "templates", "feed", "feed.html")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read feed template: %v", err)
	}
	template := string(content)
	if strings.Contains(template, `x-data="feedPage({{`) {
		t.Fatal("feed template must not interpolate dynamic values into an Alpine expression")
	}
	for _, want := range []string{`x-data="feedPage($el)"`, `data-page="{{ .Page }}"`, `data-total="{{ .Total }}"`, `data-sort="{{ .Sort }}"`} {
		if !strings.Contains(template, want) {
			t.Fatalf("feed template missing safe initialization contract %q", want)
		}
	}
}
