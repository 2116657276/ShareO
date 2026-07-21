package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCrossOriginProtectionMatrix(t *testing.T) {
	t.Parallel()
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	protected, err := CrossOriginProtection(next, []string{"https://trusted.example"})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name       string
		method     string
		origin     string
		fetchSite  string
		wantStatus int
	}{
		{name: "safe get from cross site", method: http.MethodGet, fetchSite: "cross-site", wantStatus: http.StatusNoContent},
		{name: "cli post without browser headers", method: http.MethodPost, wantStatus: http.StatusNoContent},
		{name: "same origin browser post", method: http.MethodPost, origin: "https://shareo.test", fetchSite: "same-origin", wantStatus: http.StatusNoContent},
		{name: "trusted origin browser post", method: http.MethodPost, origin: "https://trusted.example", fetchSite: "cross-site", wantStatus: http.StatusNoContent},
		{name: "cross site browser post", method: http.MethodPost, origin: "https://evil.example", fetchSite: "cross-site", wantStatus: http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "https://shareo.test/resource", nil)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			if tt.fetchSite != "" {
				req.Header.Set("Sec-Fetch-Site", tt.fetchSite)
			}
			recorder := httptest.NewRecorder()
			protected.ServeHTTP(recorder, req)
			if recorder.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, tt.wantStatus)
			}
		})
	}
}

func TestCrossOriginProtectionRejectsInvalidTrustedOrigin(t *testing.T) {
	t.Parallel()
	_, err := CrossOriginProtection(http.NotFoundHandler(), []string{"*"})
	if err == nil {
		t.Fatal("expected invalid trusted origin to be rejected")
	}
}
