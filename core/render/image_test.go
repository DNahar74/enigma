package render

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
)

func TestRenderImage(t *testing.T) {
	// Create a simple 2x2 PNG image in memory
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{255, 0, 0, 255}) // Red
	img.Set(1, 0, color.RGBA{0, 255, 0, 255}) // Green
	img.Set(0, 1, color.RGBA{0, 0, 255, 255}) // Blue
	img.Set(1, 1, color.RGBA{255, 255, 0, 255}) // Yellow

	var buf bytes.Buffer
	err := png.Encode(&buf, img)
	if err != nil {
		t.Fatalf("Failed to encode test image: %v", err)
	}

	// Render the image
	output := RenderImage(buf.Bytes(), 10)

	// Since width is small, it shouldn't be resized out of bounds.
	// We expect the output to contain terminal escape codes.
	if !strings.Contains(output, "\x1b[38;2;255;0;0m") {
		t.Errorf("Expected red color escape sequence for top-left pixel, got:\n%s", output)
	}
	if !strings.Contains(output, "\x1b[48;2;0;0;255m") {
		t.Errorf("Expected blue background escape sequence for bottom-left pixel, got:\n%s", output)
	}
	if !strings.Contains(output, "▀") {
		t.Errorf("Expected upper half block character '▀', got:\n%s", output)
	}
}

func TestRenderImage_InvalidBytes(t *testing.T) {
	output := RenderImage([]byte("not an image"), 10)
	if !strings.Contains(output, "[Image: Failed to decode") {
		t.Errorf("Expected error message for invalid image bytes, got: %s", output)
	}
}
