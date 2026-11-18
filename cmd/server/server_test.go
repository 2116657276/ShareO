package main

import (
	"net/http"
	"testing"
	"time"
)

func TestNewHTTPServerHasBoundedConnectionResources(t *testing.T) {
	srv := newHTTPServer("127.0.0.1:8080", http.NotFoundHandler())

	if srv.ReadHeaderTimeout != 5*time.Second {
		t.Fatalf("ReadHeaderTimeout = %s, want 5s", srv.ReadHeaderTimeout)
	}
	if srv.ReadTimeout != 2*time.Minute {
		t.Fatalf("ReadTimeout = %s, want 2m", srv.ReadTimeout)
	}
	if srv.WriteTimeout != 60*time.Second {
		t.Fatalf("WriteTimeout = %s, want 60s", srv.WriteTimeout)
	}
	if srv.IdleTimeout != 2*time.Minute {
		t.Fatalf("IdleTimeout = %s, want 2m", srv.IdleTimeout)
	}
	if srv.MaxHeaderBytes != 1<<20 {
		t.Fatalf("MaxHeaderBytes = %d, want 1MiB", srv.MaxHeaderBytes)
	}
}
