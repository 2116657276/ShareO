package upload

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

func TestSquareCrop_Horizontal(t *testing.T) {
	// 200x100 image → square crop should be 100x100 (centered)
	img := image.NewRGBA(image.Rect(0, 0, 200, 100))
	// Fill left half red, right half blue
	for x := 0; x < 200; x++ {
		for y := 0; y < 100; y++ {
			if x < 100 {
				img.Set(x, y, color.RGBA{255, 0, 0, 255})
			} else {
				img.Set(x, y, color.RGBA{0, 0, 255, 255})
			}
		}
	}

	cropped := squareCrop(img, 200, 100)
	bounds := cropped.Bounds()
	if bounds.Dx() != 100 || bounds.Dy() != 100 {
		t.Errorf("expected 100x100, got %dx%d", bounds.Dx(), bounds.Dy())
	}

	// Center pixel should be from the center (x=100) of original (blue area)
	r, g, b, _ := cropped.At(50, 50).RGBA()
	if r>>8 != 0 || g>>8 != 0 || b>>8 != 255 {
		t.Errorf("center should be blue (from right half), got R=%d G=%d B=%d", r>>8, g>>8, b>>8)
	}
}

func TestSquareCrop_Vertical(t *testing.T) {
	// 100x200 image → square crop should be 100x100 (centered)
	img := image.NewRGBA(image.Rect(0, 0, 100, 200))
	cropped := squareCrop(img, 100, 200)
	bounds := cropped.Bounds()
	if bounds.Dx() != 100 || bounds.Dy() != 100 {
		t.Errorf("expected 100x100, got %dx%d", bounds.Dx(), bounds.Dy())
	}
}

func TestSquareCrop_Square(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	cropped := squareCrop(img, 100, 100)
	bounds := cropped.Bounds()
	if bounds.Dx() != 100 || bounds.Dy() != 100 {
		t.Errorf("expected 100x100, got %dx%d", bounds.Dx(), bounds.Dy())
	}
	// Should return the same image pointer
	if cropped != img {
		t.Error("square image should return same instance")
	}
}

func TestDecodeImage_JPEG(t *testing.T) {
	buf := new(bytes.Buffer)
	img := image.NewRGBA(image.Rect(0, 0, 50, 50))
	jpeg.Encode(buf, img, &jpeg.Options{Quality: 90})

	decoded, err := decodeImage(bytes.NewReader(buf.Bytes()), "image/jpeg")
	if err != nil {
		t.Fatalf("decode err: %v", err)
	}
	if decoded == nil {
		t.Fatal("decoded is nil")
	}
	b := decoded.Bounds()
	if b.Dx() != 50 || b.Dy() != 50 {
		t.Errorf("expected 50x50, got %dx%d", b.Dx(), b.Dy())
	}
}

func TestDecodeImage_PNG(t *testing.T) {
	buf := new(bytes.Buffer)
	img := image.NewRGBA(image.Rect(0, 0, 30, 30))
	png.Encode(buf, img)

	decoded, err := decodeImage(bytes.NewReader(buf.Bytes()), "image/png")
	if err != nil {
		t.Fatalf("decode err: %v", err)
	}
	if decoded == nil {
		t.Fatal("decoded is nil")
	}
}

func TestDecodeImage_Unsupported(t *testing.T) {
	decoded, err := decodeImage(bytes.NewReader([]byte{}), "application/pdf")
	if err != nil {
		t.Errorf("unsupported should not error: %v", err)
	}
	if decoded != nil {
		t.Error("unsupported format should return nil")
	}
}

func TestEncodeJPEG(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	b, err := encodeJPEG(img, 85)
	if err != nil {
		t.Fatalf("encodeJPEG returned error: %v", err)
	}
	if len(b) == 0 {
		t.Error("encodeJPEG returned empty bytes")
	}
	// Verify it's valid JPEG
	_, err = jpeg.Decode(bytes.NewReader(b))
	if err != nil {
		t.Errorf("encoded data is not valid JPEG: %v", err)
	}
}

func TestThumbSizeConstants(t *testing.T) {
	if ThumbSize != 300 {
		t.Errorf("ThumbSize = %d, want 300", ThumbSize)
	}
	if MediumSize != 1200 {
		t.Errorf("MediumSize = %d, want 1200", MediumSize)
	}
}
