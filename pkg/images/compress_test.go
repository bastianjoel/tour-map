package images

import (
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
)

func TestCompressImage_DownscalingAndQuality(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "compress-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a large 3000x2000 synthetic image
	rawWidth, rawHeight := 3000, 2000
	rawImg := image.NewRGBA(image.Rect(0, 0, rawWidth, rawHeight))
	for y := 0; y < rawHeight; y++ {
		for x := 0; x < rawWidth; x++ {
			rawImg.Set(x, y, color.RGBA{R: uint8(x % 256), G: uint8(y % 256), B: 128, A: 255})
		}
	}

	srcPath := filepath.Join(tmpDir, "raw.jpg")
	srcFile, err := os.Create(srcPath)
	if err != nil {
		t.Fatalf("failed to create raw image file: %v", err)
	}
	if err := jpeg.Encode(srcFile, rawImg, &jpeg.Options{Quality: 100}); err != nil {
		t.Fatalf("failed to encode raw image: %v", err)
	}
	srcFile.Close()

	destPath := filepath.Join(tmpDir, "nested", "sub", "compressed.jpg")
	maxDim := 1200
	quality := 75

	if err := CompressImage(srcPath, destPath, maxDim, quality); err != nil {
		t.Fatalf("CompressImage() failed: %v", err)
	}

	// Verify compressed image
	destFile, err := os.Open(destPath)
	if err != nil {
		t.Fatalf("failed to open compressed image: %v", err)
	}
	defer destFile.Close()

	destImg, _, err := image.Decode(destFile)
	if err != nil {
		t.Fatalf("failed to decode compressed image: %v", err)
	}

	bounds := destImg.Bounds()
	if bounds.Dx() != 1200 {
		t.Errorf("expected compressed width 1200, got %d", bounds.Dx())
	}
	if bounds.Dy() != 800 {
		t.Errorf("expected compressed height 800, got %d", bounds.Dy())
	}
}

func TestRotateAndFlip(t *testing.T) {
	// 2x2 test image with distinct pixel colors
	// [ (1,0,0), (0,1,0) ]
	// [ (0,0,1), (1,1,1) ]
	src := image.NewRGBA(image.Rect(0, 0, 2, 2))
	cTL := color.RGBA{255, 0, 0, 255}
	cTR := color.RGBA{0, 255, 0, 255}
	cBL := color.RGBA{0, 0, 255, 255}
	cBR := color.RGBA{255, 255, 255, 255}

	src.Set(0, 0, cTL)
	src.Set(1, 0, cTR)
	src.Set(0, 1, cBL)
	src.Set(1, 1, cBR)

	// Orientation 6: Rotate 90 CW
	// New Top-Left should be old Bottom-Left (0, 1) -> (0, 0)
	r90 := rotateAndFlip(src, 6)
	if r90.At(0, 0) != cBL {
		t.Errorf("expected rotate 90 CW top-left to be cBL, got %v", r90.At(0, 0))
	}

	// Orientation 3: Rotate 180
	// New Top-Left should be old Bottom-Right (1, 1) -> (0, 0)
	r180 := rotateAndFlip(src, 3)
	if r180.At(0, 0) != cBR {
		t.Errorf("expected rotate 180 top-left to be cBR, got %v", r180.At(0, 0))
	}
}
