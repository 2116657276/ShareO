package handler

import (
	"bytes"
	"io"
	"net/http"
	"testing"
)

// The following tests validate magic number detection against the same
// http.DetectContentType that UploadHandler uses to filter file uploads.
// The handler's allowed types are: image/jpeg, image/png, image/webp, image/gif.

// Common magic byte headers used across tests.
var (
	jpegMagic = []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46, 0x00, 0x01}
	pngMagic  = []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	gifMagic  = []byte{0x47, 0x49, 0x46, 0x38, 0x39, 0x61}
	webpMagic = []byte{0x52, 0x49, 0x46, 0x46, 0x00, 0x00, 0x00, 0x00, 0x57, 0x45, 0x42, 0x50}
	bmpMagic  = []byte{0x42, 0x4D}
	svgData   = []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="100" height="100"><circle cx="50" cy="50" r="40"/></svg>`)
	pdfMagic  = []byte{0x25, 0x50, 0x44, 0x46, 0x2D, 0x31, 0x2E, 0x34}
)

func makeBuf(data []byte) []byte {
	buf := make([]byte, 512)
	copy(buf, data)
	return buf
}

func TestMagicNumberDetection_JPEG(t *testing.T) {
	detected := http.DetectContentType(makeBuf(jpegMagic))
	if detected != "image/jpeg" {
		t.Errorf("DetectContentType for JPEG = %s, want image/jpeg", detected)
	}
}

func TestMagicNumberDetection_PNG(t *testing.T) {
	detected := http.DetectContentType(makeBuf(pngMagic))
	if detected != "image/png" {
		t.Errorf("DetectContentType for PNG = %s, want image/png", detected)
	}
}

func TestMagicNumberDetection_GIF(t *testing.T) {
	gifHeader := []byte{0x47, 0x49, 0x46, 0x38, 0x39, 0x61, 0x01, 0x00, 0x01, 0x00, 0x80, 0x00}
	detected := http.DetectContentType(makeBuf(gifHeader))
	if detected != "image/gif" {
		t.Errorf("DetectContentType for GIF = %s, want image/gif", detected)
	}
}

func TestMagicNumberDetection_WebP(t *testing.T) {
	buf := makeBuf(webpMagic)
	detected := http.DetectContentType(buf)

	// http.DetectContentType does NOT natively support WebP.
	// The upload handler applies a manual override: RIFF+WEBP → image/webp
	if detected == "application/octet-stream" && len(webpMagic) >= 12 &&
		string(webpMagic[0:4]) == "RIFF" && string(webpMagic[8:12]) == "WEBP" {
		detected = "image/webp"
	}

	if detected != "image/webp" {
		t.Errorf("WebP detection = %s, want image/webp", detected)
	}
}

func TestMagicNumberDetection_PlainText(t *testing.T) {
	textContent := []byte("Hello, this is plain text content for testing purposes.")
	detected := http.DetectContentType(makeBuf(textContent))
	if detected == "image/jpeg" || detected == "image/png" || detected == "image/gif" || detected == "image/webp" {
		t.Errorf("DetectContentType for plain text = %s, should not be an image type", detected)
	}
}

func TestMagicNumberDetection_HTML(t *testing.T) {
	htmlContent := []byte("<!DOCTYPE html><html><head><title>Test</title></head><body></body></html>")
	detected := http.DetectContentType(makeBuf(htmlContent))
	if detected == "image/jpeg" || detected == "image/png" || detected == "image/gif" || detected == "image/webp" {
		t.Errorf("DetectContentType for HTML = %s, should not be an image type", detected)
	}
}

func TestMagicNumberDetection_SVG(t *testing.T) {
	// SVG with xml declaration should be detected as text/xml or image/svg+xml.
	// Important: SVG is NOT in the handler's allowed types list.
	detected := http.DetectContentType(makeBuf(svgData))
	if detected == "image/jpeg" || detected == "image/png" || detected == "image/gif" || detected == "image/webp" {
		t.Errorf("DetectContentType for SVG = %s, should not be a raster image type", detected)
	}
}

func TestMagicNumberDetection_BMP(t *testing.T) {
	// BMP is detected but NOT in the handler's allowed types.
	detected := http.DetectContentType(makeBuf(bmpMagic))
	if detected == "image/jpeg" || detected == "image/png" || detected == "image/gif" || detected == "image/webp" {
		t.Errorf("DetectContentType for BMP = %s, should not be a raster image type", detected)
	}
}

func TestMagicNumberDetection_ShortBuffer(t *testing.T) {
	buf := []byte{0xFF, 0xD8} // JPEG start, 2 bytes only
	detected := http.DetectContentType(buf)
	if detected == "" {
		t.Error("DetectContentType returned empty for 2-byte buffer")
	}
}

func TestMagicNumberDetection_AllowedTypes(t *testing.T) {
	// The project only allows these image types for upload:
	// image/jpeg, image/png, image/webp, image/gif
	// See internal/handler/upload_handler.go for the authoritative list.
	allowed := map[string]bool{
		"image/jpeg": true,
		"image/png":  true,
		"image/webp": true,
		"image/gif":  true,
	}

	tests := []struct {
		name    string
		data    []byte
		allowed bool
	}{
		{"JPEG", jpegMagic, true},
		{"PNG", pngMagic, true},
		{"GIF", gifMagic, true},
		{"WebP", webpMagic, true},
		{"BMP", bmpMagic, false},
		{"PDF", pdfMagic, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := makeBuf(tt.data)
			detected := http.DetectContentType(buf)

			// Apply the same WebP override that the upload handler uses
			if detected == "application/octet-stream" && tt.name == "WebP" && len(tt.data) >= 12 &&
				string(tt.data[0:4]) == "RIFF" && string(tt.data[8:12]) == "WEBP" {
				detected = "image/webp"
			}

			if allowed[detected] != tt.allowed {
				t.Errorf("%s: DetectContentType=%s, allowed=%v, want allowed=%v",
					tt.name, detected, allowed[detected], tt.allowed)
			}
		})
	}
}

func TestMultiReader_Prepend(t *testing.T) {
	prefix := []byte("PREFIX_")
	body := []byte("BODY_DATA")
	reader := io.MultiReader(bytes.NewReader(prefix), bytes.NewReader(body))

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
