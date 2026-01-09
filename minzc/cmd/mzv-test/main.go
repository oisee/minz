// mzv-test: Test MZV framebuffer and PNG export
package main

import (
	"fmt"
	"os"

	"github.com/minz/minzc/pkg/mirvm"
)

func main() {
	// Create a 320x240 Agon-style display
	platform := mirvm.NewAgonPlatform()
	display := platform.GenericPlatform.Display().(*mirvm.GenericDisplay)

	fmt.Println("MZV Framebuffer Test")
	fmt.Printf("Resolution: %dx%d, BPP: %d\n", display.Width(), display.Height(), display.BPP())

	// Draw a simple raymarched sphere approximation
	drawSphere(display)

	// Save to PNG
	outputFile := "mzv_test_output.png"
	if len(os.Args) > 1 {
		outputFile = os.Args[1]
	}

	if err := display.SavePNG(outputFile); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving PNG: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Saved framebuffer to: %s\n", outputFile)
}

// drawSphere renders a simple raymarched sphere with lighting
func drawSphere(d *mirvm.GenericDisplay) {
	width := d.Width()
	height := d.Height()

	// Sphere center and radius (in screen space)
	cx, cy := width/2, height/2
	radius := 80

	// Light direction (normalized)
	lx, ly, lz := 0.5, -0.5, 0.7
	lenL := sqrt(lx*lx + ly*ly + lz*lz)
	lx, ly, lz = lx/lenL, ly/lenL, lz/lenL

	// Clear to dark blue (Spectrum color 1)
	d.Clear(0xFF000088)

	// Draw sphere with simple lighting
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// Distance from center
			dx := float64(x - cx)
			dy := float64(y - cy)
			dist := sqrt(dx*dx + dy*dy)

			if dist < float64(radius) {
				// Calculate Z from sphere equation
				r := float64(radius)
				z := sqrt(r*r - dx*dx - dy*dy)

				// Normal at this point
				nx, ny, nz := dx/r, dy/r, z/r

				// Diffuse lighting (N dot L)
				diffuse := nx*lx + ny*ly + nz*lz
				if diffuse < 0 {
					diffuse = 0
				}

				// Add ambient
				ambient := 0.2
				brightness := ambient + diffuse*0.8

				// Clamp
				if brightness > 1.0 {
					brightness = 1.0
				}

				// Color gradient (red to yellow based on position)
				baseR := 255.0
				baseG := 100.0 + 155.0*(float64(y)/float64(height))
				baseB := 50.0

				r8 := uint8(baseR * brightness)
				g8 := uint8(baseG * brightness)
				b8 := uint8(baseB * brightness)

				color := uint32(0xFF000000) | uint32(r8)<<16 | uint32(g8)<<8 | uint32(b8)
				d.SetPixel(x, y, color)
			}
		}
	}

	// Add specular highlight
	specX, specY := cx-radius/3, cy-radius/3
	for dy := -8; dy <= 8; dy++ {
		for dx := -8; dx <= 8; dx++ {
			dist := sqrt(float64(dx*dx + dy*dy))
			if dist < 8 {
				brightness := 1.0 - dist/8.0
				alpha := uint8(brightness * 200)
				x, y := specX+dx, specY+dy
				if x >= 0 && x < width && y >= 0 && y < height {
					// Blend white highlight
					existing := d.GetPixel(x, y)
					r := uint8((existing >> 16) & 0xFF)
					g := uint8((existing >> 8) & 0xFF)
					b := uint8(existing & 0xFF)
					r = blend(r, 255, alpha)
					g = blend(g, 255, alpha)
					b = blend(b, 255, alpha)
					d.SetPixel(x, y, 0xFF000000|uint32(r)<<16|uint32(g)<<8|uint32(b))
				}
			}
		}
	}
}

func sqrt(x float64) float64 {
	if x <= 0 {
		return 0
	}
	// Newton-Raphson
	guess := x / 2
	for i := 0; i < 10; i++ {
		guess = (guess + x/guess) / 2
	}
	return guess
}

func blend(base, overlay, alpha uint8) uint8 {
	return uint8((int(base)*(255-int(alpha)) + int(overlay)*int(alpha)) / 255)
}
