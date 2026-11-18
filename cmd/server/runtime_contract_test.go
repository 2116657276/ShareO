package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalRuntimeDoesNotUseKnownInternalTokenFallback(t *testing.T) {
	path := filepath.Join("..", "..", "scripts", "local_runtime.sh")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read local runtime script: %v", err)
	}
	script := string(content)
	if strings.Contains(script, "shareo-local-internal") {
		t.Fatal("local runtime must not use a publicly known internal token")
	}
	for _, want := range []string{"internal-token", "openssl rand -hex 32", "chmod 600"} {
		if !strings.Contains(script, want) {
			t.Fatalf("local runtime missing generated-token contract %q", want)
		}
	}
}
