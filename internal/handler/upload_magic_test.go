package handler

import (
	"bytes"
	"io"
	"net/http"
	"testing"
)

// TestMagicNumberDetection tests the real content type detection via http.DetectContentType
// against common image format magic bytes (first 512 bytes max).
func TestMagicNumberDetection_JPEG(t *testing.T) {
	// JPEG magic bytes: FF D8 FF
	jpegHeader := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46, 0x00, 0x01}
	buf := make([]byte, 512)
	copy(buf, jpegHeader)
	detected := http.DetectContentType(buf)
	if detected != "image/jpeg" {
		t.Errorf("DetectContentType for JPEG = %s, want image/jpeg", detected)
	}
}

func TestMagicNumberDetection_PNG(t *testing.T) {
	// PNG magic bytes: 89 50 4E 47 0D 0A 1A 0A
	pngHeader := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D}
	buf := make([]byte, 512)
	copy(buf, pngHeader)
	detected := http.DetectContentType(buf)
	if detected != "image/png" {
		t.Errorf("DetectContentType for PNG = %s, want image/png", detected)
	}
}

func TestMagicNumberDetection_GIF(t *testing.T) {
	// GIF magic bytes: 47 49 46 38 (GIF8)
	gifHeader := []byte{0x47, 0x49, 0x46, 0x38, 0x39, 0x61, 0x01, 0x00, 0x01, 0x00, 0x80, 0x00}
	buf := make([]byte, 512)
	copy(buf, gifHeader)
	detected := http.DetectContentType(buf)
	if detected != "image/gif" {
		t.Errorf("DetectContentType for GIF = %s, want image/gif", detected)
	}
}

func TestMagicNumberDetection_WebP(t *testing.T) {
	// WebP magic bytes: 52 49 46 46 ... 57 45 42 50 (RIFF....WEBP)
	webpHeader := []byte{0x52, 0x49, 0x46, 0x46, 0x00, 0x00, 0x00, 0x00, 0x57, 0x45, 0x42, 0x50}
	buf := make([]byte, 512)
	copy(buf, webpHeader)
	detected := http.DetectContentType(buf)

	// http.DetectContentType does NOT natively support WebP (returns application/octet-stream).
	// The handler applies manual override: RIFF+WEBP → image/webp
	if detected == "application/octet-stream" && len(webpHeader) >= 12 &&
		string(webpHeader[0:4]) == "RIFF" && string(webpHeader[8:12]) == "WEBP" {
		detected = "image/webp"
	}

	if detected != "image/webp" {
		t.Errorf("WebP detection = %s, want image/webp", detected)
	}
}

func TestMagicNumberDetection_PlainText(t *testing.T) {
	// Plain text should NOT be detected as an image type
	textContent := []byte("Hello, this is plain text content for testing purposes. It should not be detected as an image.")
	buf := make([]byte, 512)
	copy(buf, textContent)
	detected := http.DetectContentType(buf)
	if detected == "image/jpeg" || detected == "image/png" || detected == "image/gif" || detected == "image/webp" {
		t.Errorf("DetectContentType for plain text = %s, should not be an image type", detected)
	}
}

func TestMagicNumberDetection_HTML(t *testing.T) {
	// HTML should be detected as text/html, not image
	htmlContent := []byte("<!DOCTYPE html><html><head><title>Test</title></head><body></body></html>")
	buf := make([]byte, 512)
	copy(buf, htmlContent)
	detected := http.DetectContentType(buf)
	if detected == "image/jpeg" || detected == "image/png" || detected == "image/gif" || detected == "image/webp" {
		t.Errorf("DetectContentType for HTML = %s, should not be an image type", detected)
	}
	if detected != "text/html; charset=utf-8" {
		t.Errorf("DetectContentType for HTML = %s, want text/html; charset=utf-8", detected)
	}
}

func TestMagicNumberDetection_ShortBuffer(t *testing.T) {
	// Only 2 bytes — should not crash
	buf := []byte{0xFF, 0xD8} // JPEG start
	detected := http.DetectContentType(buf)
	if detected == "" {
		t.Error("DetectContentType returned empty for 2-byte buffer")
	}
}

func TestMagicNumberDetection_AllowedTypes(t *testing.T) {
	allowed := map[string]bool{
		"image/jpeg": true, "image/png": true,
		"image/webp": true, "image/gif": true,
	}

	tests := []struct {
		name    string
		header  []byte
		allowed bool
	}{
		{"JPEG", []byte{0xFF, 0xD8, 0xFF, 0xE0}, true},
		{"PNG", []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}, true},
		{"GIF", []byte{0x47, 0x49, 0x46, 0x38, 0x39, 0x61}, true},
		{"WebP", []byte{0x52, 0x49, 0x46, 0x46, 0x00, 0x00, 0x00, 0x00, 0x57, 0x45, 0x42, 0x50}, true}, // manual override required
		{"BMP", []byte{0x42, 0x4D, 0x00, 0x00, 0x00, 0x00}, false},
		{"PDF", []byte{0x25, 0x50, 0x44, 0x46, 0x2D, 0x31, 0x2E, 0x34}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := make([]byte, 512)
			copy(buf, tt.header)
			detected := http.DetectContentType(buf)
			// WebP requires manual override (http.DetectContentType doesn't support it)
		if detected == "application/octet-stream" && tt.name == "WebP" && len(tt.header) >= 12 &&
			string(tt.header[0:4]) == "RIFF" && string(tt.header[8:12]) == "WEBP" {
			detected = "image/webp"
		}

		if allowed[detected] != tt.allowed {
			t.Errorf("%s: DetectContentType=%s, allowed=%v, want allowed=%v", tt.name, detected, allowed[detected], tt.allowed)
		}
		})
	}
}

func TestMultiReader_Prepend(t *testing.T) {
	// Verify io.MultiReader correctly reassembles read bytes
	prefix := []byte("PREFIX_")
	body := []byte("BODY_DATA")
	reader := io.MultiReader(bytes.NewReader(prefix), bytes.NewReader(body))

	// Read all
	result := make([]byte, len(prefix)+len(body))
	n, err := io.ReadFull(reader, result)
	if err != nil {
		t.Fatalf("ReadFull failed: %v", err)
	}
	if n != len(prefix)+len(body) {
		t.Errorf("read %d bytes, want %d", n, len(prefix)+len(body))
	}
	expected := "PREFIX_BODY_DATA"
	if string(result) != expected {
		t.Errorf("got %q, want %q", string(result), expected)
	}
}
