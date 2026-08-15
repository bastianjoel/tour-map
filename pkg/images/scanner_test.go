package images

import (
	"image"
	"image/color"
	"image/jpeg"
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
		{"photo.png", true},
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

func TestExtractExifTimestamp_Nil(t *testing.T) {
	_, ok := ExtractExifTimestamp(nil)
	if ok {
		t.Errorf("expected false for nil exif")
	}
}

func TestScanner_NonExistentDir(t *testing.T) {
	scanner := NewScanner("/path/that/does/not/exist", "")
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

	scanner := NewScanner(tmpDir, "")
	if err := scanner.Scan(); err != nil {
		t.Fatalf("Scan() failed: %v", err)
	}

	if len(scanner.GetImages()) != 0 {
		t.Errorf("expected 0 images, got %d", len(scanner.GetImages()))
	}
}

func TestScanner_MultipleImagesSortingAndCompression(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "images-sort-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	rawDir := filepath.Join(tmpDir, "raw")
	compDir := filepath.Join(tmpDir, "compressed")
	os.MkdirAll(rawDir, 0755)
	os.MkdirAll(compDir, 0755)

	// Write two dummy images with different mod times
	img1Path := filepath.Join(rawDir, "img1.jpg")
	img2Path := filepath.Join(rawDir, "img2.jpg")

	// Create valid small JPEGs
	dummyImg := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			dummyImg.Set(x, y, color.RGBA{R: 200, G: 100, B: 50, A: 255})
		}
	}
	f1, _ := os.Create(img1Path)
	jpeg.Encode(f1, dummyImg, nil)
	f1.Close()

	f2, _ := os.Create(img2Path)
	jpeg.Encode(f2, dummyImg, nil)
	f2.Close()

	t1 := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)

	os.Chtimes(img1Path, t1, t1)
	os.Chtimes(img2Path, t2, t2)

	scanner := NewScanner(rawDir, compDir)
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

	// Verify compressed images were created in compDir
	if _, err := os.Stat(filepath.Join(compDir, "img1.jpg")); os.IsNotExist(err) {
		t.Errorf("expected compressed img1.jpg to exist in %s", compDir)
	}
	if _, err := os.Stat(filepath.Join(compDir, "img2.jpg")); os.IsNotExist(err) {
		t.Errorf("expected compressed img2.jpg to exist in %s", compDir)
	}
}

func TestScanner_RecursiveSubdirectories(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "images-recursive-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	rawDir := filepath.Join(tmpDir, "raw")
	compDir := filepath.Join(tmpDir, "comp")

	tripADir := filepath.Join(rawDir, "2026", "tripA")
	subDir := filepath.Join(rawDir, "2026", "tripB", "sub")
	os.MkdirAll(tripADir, 0755)
	os.MkdirAll(subDir, 0755)

	pRoot := filepath.Join(rawDir, "root.jpg")
	p1 := filepath.Join(tripADir, "photo1.jpg")
	p2 := filepath.Join(subDir, "photo2.jpg")

	dummyImg := image.NewRGBA(image.Rect(0, 0, 50, 50))
	for _, p := range []string{pRoot, p1, p2} {
		f, _ := os.Create(p)
		jpeg.Encode(f, dummyImg, nil)
		f.Close()
	}

	tRoot := time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)
	t1 := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 8, 1, 11, 0, 0, 0, time.UTC)

	os.Chtimes(pRoot, tRoot, tRoot)
	os.Chtimes(p1, t1, t1)
	os.Chtimes(p2, t2, t2)

	scanner := NewScanner(rawDir, compDir)
	if err := scanner.Scan(); err != nil {
		t.Fatalf("Scan() failed: %v", err)
	}

	imgs := scanner.GetImages()
	if len(imgs) != 3 {
		t.Fatalf("expected 3 images recursively found, got %d", len(imgs))
	}

	if imgs[0].Filename != "root.jpg" {
		t.Errorf("expected root.jpg, got %s", imgs[0].Filename)
	}
	if imgs[1].Filename != "2026/tripA/photo1.jpg" {
		t.Errorf("expected 2026/tripA/photo1.jpg, got %s", imgs[1].Filename)
	}
	if imgs[2].Filename != "2026/tripB/sub/photo2.jpg" {
		t.Errorf("expected 2026/tripB/sub/photo2.jpg, got %s", imgs[2].Filename)
	}

	// Verify nested compressed structure
	if _, err := os.Stat(filepath.Join(compDir, "2026", "tripA", "photo1.jpg")); os.IsNotExist(err) {
		t.Errorf("nested compressed image 2026/tripA/photo1.jpg not found in compDir")
	}
	if _, err := os.Stat(filepath.Join(compDir, "2026", "tripB", "sub", "photo2.jpg")); os.IsNotExist(err) {
		t.Errorf("nested compressed image 2026/tripB/sub/photo2.jpg not found in compDir")
	}
}
