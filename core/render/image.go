package render

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	_ "golang.org/x/image/webp"

	"github.com/qeesung/image2ascii/convert"
)

// RenderImage takes raw image bytes and returns a colored ASCII string representation.
// maxWidth ensures the image fits within the current terminal viewport width.
func RenderImage(imgBytes []byte, maxWidth int) string {
	img, _, err := image.Decode(bytes.NewReader(imgBytes))
	if err != nil {
		return fmt.Sprintf("[Image: Failed to decode - %v]", err)
	}

	// Calculate height preserving aspect ratio
	bounds := img.Bounds()
	imgWidth := bounds.Dx()
	imgHeight := bounds.Dy()

	// Convert pixel aspect ratio to terminal block aspect ratio
	// A terminal character is usually about twice as tall as it is wide.
	terminalRatio := 0.5

	newWidth := imgWidth
	if newWidth > maxWidth {
		newWidth = maxWidth
	}
	// Cap maximum height to not flood the terminal
	if newWidth > 80 {
		newWidth = 80
	}

	ratio := float64(newWidth) / float64(imgWidth)
	newHeight := int(float64(imgHeight) * ratio * terminalRatio)

	converter := convert.NewImageConverter()
	options := convert.Options{
		Ratio:       1.0,
		FixedWidth:  newWidth,
		FixedHeight: newHeight,
		FitScreen:   false,
		Colored:     true,
		Reversed:    false,
	}

	return converter.Image2ASCIIString(img, &options)
}
