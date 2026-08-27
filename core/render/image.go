package render

import (
	"bytes"
	"fmt"
	_ "golang.org/x/image/webp"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"strings"

	"github.com/nfnt/resize"
)

func colorToRGB(c color.Color) (r, g, b uint8) {
	r32, g32, b32, _ := c.RGBA()
	return uint8(r32 >> 8), uint8(g32 >> 8), uint8(b32 >> 8)
}

// RenderImage takes raw image bytes and returns a TrueColor half-block ASCII string.
// maxWidth ensures the image fits within the current terminal viewport width.
func RenderImage(imgBytes []byte, maxWidth int) string {
	img, _, err := image.Decode(bytes.NewReader(imgBytes))
	if err != nil {
		return fmt.Sprintf("[Image: Failed to decode - %v]", err)
	}

	bounds := img.Bounds()
	imgWidth := bounds.Dx()

	newWidth := uint(imgWidth)
	if newWidth > uint(maxWidth) {
		newWidth = uint(maxWidth)
	}
	// Limit max width so it doesn't flood standard terminals
	if newWidth > 80 {
		newWidth = 80
	}

	// Resize image proportionally using high-quality Bilinear interpolation
	img = resize.Resize(newWidth, 0, img, resize.Bilinear)

	bounds = img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()

	var sb strings.Builder
	sb.WriteString("\n") // padding

	// We iterate height by 2 because each terminal character ('▀') acts as two pixels vertically
	// The foreground color paints the top pixel, the background color paints the bottom pixel.
	for y := 0; y < h; y += 2 {
		for x := 0; x < w; x++ {
			topColor := img.At(x, y)
			tr, tg, tb := colorToRGB(topColor)

			// Default bottom color to black if out of bounds (odd height image)
			var br, bg, bb uint8 = 0, 0, 0
			if y+1 < h {
				bottomColor := img.At(x, y+1)
				br, bg, bb = colorToRGB(bottomColor)
			}

			// \x1b[38;2;R;G;Bm sets TrueColor foreground (top half)
			// \x1b[48;2;R;G;Bm sets TrueColor background (bottom half)
			// '▀' is the upper-half block character
			sb.WriteString(fmt.Sprintf("\x1b[38;2;%d;%d;%dm\x1b[48;2;%d;%d;%dm▀", tr, tg, tb, br, bg, bb))
		}
		// Reset colors at the end of the line
		sb.WriteString("\x1b[0m\n")
	}

	return sb.String()
}
