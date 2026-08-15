package images

import (
	"image"
	"image/color"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"

	"github.com/rwcarlsen/goexif/exif"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/tiff"
)

const (
	// DefaultMaxImageDimension is the default maximum width or height for compressed images.
	DefaultMaxImageDimension = 1920

	// DefaultJPEGQuality is the default quality level for compressed JPEG images (1-100).
	DefaultJPEGQuality = 80
)

// getExifOrientation reads the EXIF orientation tag from an image file.
func getExifOrientation(r io.Reader) int {
	x, err := exif.Decode(r)
	if err != nil {
		return 1
	}
	tag, err := x.Get(exif.Orientation)
	if err != nil {
		return 1
	}
	val, err := tag.Int(0)
	if err != nil {
		return 1
	}
	return val
}

// rotateAndFlip applies EXIF orientation transformation to an image.
func rotateAndFlip(img image.Image, orientation int) image.Image {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	switch orientation {
	case 2: // Flip horizontally
		dst := image.NewRGBA(image.Rect(0, 0, w, h))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				dst.Set(w-1-x, y, img.At(bounds.Min.X+x, bounds.Min.Y+y))
			}
		}
		return dst

	case 3: // Rotate 180
		dst := image.NewRGBA(image.Rect(0, 0, w, h))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				dst.Set(w-1-x, h-1-y, img.At(bounds.Min.X+x, bounds.Min.Y+y))
			}
		}
		return dst

	case 4: // Flip vertically
		dst := image.NewRGBA(image.Rect(0, 0, w, h))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				dst.Set(x, h-1-y, img.At(bounds.Min.X+x, bounds.Min.Y+y))
			}
		}
		return dst

	case 5: // Transpose
		dst := image.NewRGBA(image.Rect(0, 0, h, w))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				dst.Set(y, x, img.At(bounds.Min.X+x, bounds.Min.Y+y))
			}
		}
		return dst

	case 6: // Rotate 90 CW
		dst := image.NewRGBA(image.Rect(0, 0, h, w))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				dst.Set(h-1-y, x, img.At(bounds.Min.X+x, bounds.Min.Y+y))
			}
		}
		return dst

	case 7: // Transverse
		dst := image.NewRGBA(image.Rect(0, 0, h, w))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				dst.Set(h-1-y, w-1-x, img.At(bounds.Min.X+x, bounds.Min.Y+y))
			}
		}
		return dst

	case 8: // Rotate 270 CW (90 CCW)
		dst := image.NewRGBA(image.Rect(0, 0, h, w))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				dst.Set(y, w-1-x, img.At(bounds.Min.X+x, bounds.Min.Y+y))
			}
		}
		return dst

	default: // Orientation 1 or unknown: normal
		return img
	}
}

// CompressImage decodes a source image file, corrects its orientation, resizes it if it exceeds
// maxDim, and writes the optimized JPEG to destPath.
func CompressImage(srcPath, destPath string, maxDim, quality int) error {
	if maxDim <= 0 {
		maxDim = DefaultMaxImageDimension
	}
	if quality <= 0 {
		quality = DefaultJPEGQuality
	}

	// 1. Read EXIF orientation
	orientation := 1
	if f, err := os.Open(srcPath); err == nil {
		orientation = getExifOrientation(f)
		f.Close()
	}

	// 2. Decode source image
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	srcImg, _, err := image.Decode(srcFile)
	if err != nil {
		return err
	}

	// 3. Apply orientation
	srcImg = rotateAndFlip(srcImg, orientation)

	// 4. Calculate target dimensions
	bounds := srcImg.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()

	var targetImg image.Image = srcImg

	if w > maxDim || h > maxDim {
		var newW, newH int
		if w >= h {
			newW = maxDim
			newH = int(float64(h) * float64(maxDim) / float64(w))
		} else {
			newH = maxDim
			newW = int(float64(w) * float64(maxDim) / float64(h))
		}
		if newW < 1 {
			newW = 1
		}
		if newH < 1 {
			newH = 1
		}

		dst := image.NewRGBA(image.Rect(0, 0, newW, newH))
		// Fill with opaque background in case of transparent sources
		draw.Draw(dst, dst.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)
		draw.BiLinear.Scale(dst, dst.Bounds(), srcImg, bounds, draw.Over, nil)
		targetImg = dst
	}

	// 5. Ensure destination directory exists
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return err
	}

	// 6. Write compressed JPEG
	outFile, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	return jpeg.Encode(outFile, targetImg, &jpeg.Options{Quality: quality})
}
