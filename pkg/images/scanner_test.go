package images

import (
	"os"
	"path/filepath"
	"testing"
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
	locs := scanner.GetLocations()
	if len(locs) != 0 {
		t.Errorf("expected 0 locations, got %d", len(locs))
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

	if len(scanner.GetLocations()) != 0 {
		t.Errorf("expected 0 locations, got %d", len(scanner.GetLocations()))
	}
}
