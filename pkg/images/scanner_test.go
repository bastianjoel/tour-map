package images

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestIsImageFile(t *testing.T) {
	tests := []struct {
		filename string
		expected bool
	}{
		{"photo.jpg", true},
		{"photo.JPEG", true},
		{"photo.JPG", true},
		{"photo.tiff", true},
		{"photo.tif", true},
		{"photo.png", false},
		{"photo.gif", false},
		{"document.pdf", false},
		{"no_ext", false},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			if got := IsImageFile(tt.filename); got != tt.expected {
				t.Errorf("IsImageFile(%q) = %v, expected %v", tt.filename, got, tt.expected)
			}
		})
	}
}

func TestScanner_NonExistentDir(t *testing.T) {
	scanner := NewScanner("/path/that/does/not/exist")
	err := scanner.Scan()
	if err != nil {
		t.Errorf("Scan() on non-existent dir should return nil error, got %v", err)
	}
	images := scanner.GetImages()
	if len(images) != 0 {
		t.Errorf("expected 0 images, got %d", len(images))
	}
}

func TestScanner_EmptyDirectory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "images-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Write a non-image file
	os.WriteFile(filepath.Join(tmpDir, "readme.txt"), []byte("hello"), 0644)

	scanner := NewScanner(tmpDir)
	if err := scanner.Scan(); err != nil {
		t.Fatalf("Scan() failed: %v", err)
	}

	if len(scanner.GetImages()) != 0 {
		t.Errorf("expected 0 images, got %d", len(scanner.GetImages()))
	}
}

func TestScanner_MultipleImagesSorting(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "images-sort-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Write two dummy images with different mod times
	img1Path := filepath.Join(tmpDir, "img1.jpg")
	img2Path := filepath.Join(tmpDir, "img2.jpg")

	os.WriteFile(img1Path, []byte("fake1"), 0644)
	os.WriteFile(img2Path, []byte("fake2"), 0644)

	t1 := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)

	os.Chtimes(img1Path, t1, t1)
	os.Chtimes(img2Path, t2, t2)

	scanner := NewScanner(tmpDir)
	if err := scanner.Scan(); err != nil {
		t.Fatalf("Scan() failed: %v", err)
	}

	imgs := scanner.GetImages()
	if len(imgs) != 2 {
		t.Fatalf("expected 2 images, got %d", len(imgs))
	}
	if imgs[0].Filename != "img1.jpg" || imgs[1].Filename != "img2.jpg" {
		t.Errorf("images not sorted by timestamp: %v, %v", imgs[0].Filename, imgs[1].Filename)
	}
}
