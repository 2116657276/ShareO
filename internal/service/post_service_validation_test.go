package service

import (
	"strings"
	"testing"
)

func TestValidatePostInputNormalizesAndBoundsImages(t *testing.T) {
	content, images, err := validatePostInput("  一段描述  ", []string{
		" /api/v1/images/posts/original/2026/08/08/a.jpg ",
		"/api/v1/images/posts/thumb/2026/08/08/a-thumb.jpg",
	})
	if err != nil {
		t.Fatal(err)
	}
	if content != "一段描述" || images[0] != "/api/v1/images/posts/original/2026/08/08/a.jpg" {
		t.Fatalf("content=%q images=%v", content, images)
	}
}

func TestValidatePostInputRejectsUntrustedImageURLs(t *testing.T) {
	for _, imageURL := range []string{
		"https://example.com/a.jpg",
		"/api/v1/images/../../secret",
		"/api/v1/images/posts/a.jpg?redirect=https://example.com",
		"/static/a.jpg",
		"/api/v1/images/posts/",
	} {
		if validUploadedImageURL(imageURL) {
			t.Errorf("accepted invalid image URL %q", imageURL)
		}
	}
}

func TestValidatePostInputRejectsOversizedOrDuplicateInput(t *testing.T) {
	if _, _, err := validatePostInput(strings.Repeat("字", MaxPostContentLength+1), []string{"/api/v1/images/posts/a.jpg"}); err != ErrPostInvalid {
		t.Fatalf("oversized content error=%v", err)
	}
	if _, _, err := validatePostInput("描述", []string{"/api/v1/images/posts/a.jpg", "/api/v1/images/posts/a.jpg"}); err != ErrPostInvalid {
		t.Fatalf("duplicate image error=%v", err)
	}
	if _, _, err := validatePostInput("描述", make([]string, MaxPostImages+1)); err != ErrPostInvalid {
		t.Fatalf("too many images error=%v", err)
	}
}
