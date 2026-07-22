package main

import (
	"html/template"
	"path/filepath"
	"testing"
)

func TestTemplatesParse(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", ".."))
	rootFiles, err := filepath.Glob(filepath.Join(root, "web", "templates", "*.html"))
	if err != nil {
		t.Fatal(err)
	}
	subFiles, err := filepath.Glob(filepath.Join(root, "web", "templates", "*", "*.html"))
	if err != nil {
		t.Fatal(err)
	}
	files := append(rootFiles, subFiles...)
	funcs := template.FuncMap{
		"sub":       func(a, b int) int { return a - b },
		"add":       func(a, b int) int { return a + b },
		"iterate":   func(n int) []int { return make([]int, n) },
		"or":        func(a, b string) string { return a + b },
		"thumbURL":  func(value string) string { return value },
		"mediumURL": func(value string) string { return value },
	}
	if _, err := template.New("shareo").Funcs(funcs).ParseFiles(files...); err != nil {
		t.Fatal(err)
	}
}
