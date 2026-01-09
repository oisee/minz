// mzv-run: Render sphere in MZV virtual machine framebuffer
// This demonstrates the MZV display system using 8.8 fixed-point math
// equivalent to what compiled MinZ fp88.minz code would produce.
package main

import (
	"fmt"
	"os"

	"github.com/minz/minzc/pkg/mirvm"
)

func main() {
	outputPNG := "mzv_sphere_render.png"
	if len(os.Args) > 1 {
		outputPNG = os.Args[1]
	}

	fmt.Println("MZV Sphere Renderer - MinZ Virtual Machine")
	fmt.Println("Using 8.8 fixed-point math (fp88 equivalent)")

	// Create MZV with Agon platform (320x240)
	platform := mirvm.NewAgonPlatform()
	display := platform.GenericPlatform.Display().(*mirvm.GenericDisplay)

	fmt.Printf("Resolution: %dx%d\n", display.Width(), display.Height())
	fmt.Println("Rendering sphere with 8.8 fixed-point lighting...")

	// Render using fp88-style fixed-point math
	renderSphereFP88(display)

	fmt.Println("Render complete!")

	// Save PNG
	if err := display.SavePNG(outputPNG); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving PNG: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Saved to: %s\n", outputPNG)
}

// 8.8 Fixed-Point Math Functions (equivalent to fp88.minz)
// These implement the same algorithms as stdlib/math/fp88.minz

func fpMul(a, b int16) int16 {
	a32 := int32(a)
	b32 := int32(b)
	result := (a32 * b32) >> 8
	return int16(result)
}

func fpDiv(a, b int16) int16 {
	if b == 0 {
		if a >= 0 {
			return 32767
		}
		return -32768
	}
	a32 := int32(a)
	b32 := int32(b)
	result := (a32 << 8) / b32
	return int16(result)
}

func fpSqrt(x int16) int16 {
	if x <= 0 {
		return 0
	}

	guess := x >> 1
	if guess == 0 {
		guess = 1
	}
	if guess > 256 {
		guess = 256
	}

	// Newton-Raphson iterations
	for i := 0; i < 6; i++ {
		divResult := fpDiv(x, guess)
		guess = (guess + divResult) >> 1
		if guess == 0 {
			guess = 1
		}
	}

	return guess
}

// renderSphereFP88 renders a sphere using 8.8 fixed-point math
// This is the Go equivalent of what compiled MinZ/MIR would execute
func renderSphereFP88(d *mirvm.GenericDisplay) {
	width := d.Width()
	height := d.Height()

	// Sphere parameters
	cx, cy := width/2, height/2
	radius := 80

	// Light direction (normalized, 8.8 fixed-point: 256 = 1.0)
	// Pointing upper-left-towards: (0.5, -0.5, 0.7)
	lx := int16(128)  // 0.5
	ly := int16(-128) // -0.5
	lz := int16(179)  // 0.7

	// Clear to dark blue
	d.Clear(0xFF000088)

	// Render the sphere
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// Distance from center
			dx := x - cx
			dy := y - cy
			distSq := dx*dx + dy*dy
			radiusSq := radius * radius

			if distSq < radiusSq {
				// Calculate z from sphere equation (8.8 fixed-point)
				zSq := int16((radiusSq - distSq) << 8)
				z := fpSqrt(zSq)

				// Normal at this point (8.8 fixed-point)
				radiusFP := int16(radius << 8)
				nx := fpDiv(int16(dx<<8), radiusFP)
				ny := fpDiv(int16(dy<<8), radiusFP)
				nz := fpDiv(z, int16(radius))

				// Diffuse lighting: N dot L (8.8 * 8.8 >> 8 = 8.8)
				diffuse := fpMul(nx, lx) + fpMul(ny, ly) + fpMul(nz, lz)

				// Clamp diffuse to 0
				if diffuse < 0 {
					diffuse = 0
				}

				// brightness = 0.2 + 0.8 * diffuse = 51 + diffuse * 204 / 256
				brightness := int16(51) + int16((int32(diffuse)*204)>>8)
				if brightness > 256 {
					brightness = 256
				}

				// Calculate RGB with gradient
				r := int((255 * int(brightness)) >> 8)
				g := int(((100 + (y * 155 / height)) * int(brightness)) >> 8)
				b := int((50 * int(brightness)) >> 8)

				// Clamp
				if r > 255 {
					r = 255
				}
				if g > 255 {
					g = 255
				}
				if b > 255 {
					b = 255
				}

				// Pack color: 0xAARRGGBB
				color := uint32(0xFF000000) | uint32(r)<<16 | uint32(g)<<8 | uint32(b)
				d.SetPixel(x, y, color)
			}
		}
	}

	// Add specular highlight
	specX, specY := cx-radius/3, cy-radius/3
	for dy := -10; dy <= 10; dy++ {
		for dx := -10; dx <= 10; dx++ {
			distSq := dx*dx + dy*dy
			if distSq < 100 {
				dist := fpSqrt(int16(distSq << 8)) >> 4
				brightness := 256 - int16(dist)*25
				if brightness < 0 {
					brightness = 0
				}
				alpha := uint8(brightness * 200 / 256)
				px, py := specX+dx, specY+dy
				if px >= 0 && px < width && py >= 0 && py < height {
					// Blend white highlight
					existing := d.GetPixel(px, py)
					er := uint8((existing >> 16) & 0xFF)
					eg := uint8((existing >> 8) & 0xFF)
					eb := uint8(existing & 0xFF)
					er = blend(er, 255, alpha)
					eg = blend(eg, 255, alpha)
					eb = blend(eb, 255, alpha)
					d.SetPixel(px, py, 0xFF000000|uint32(er)<<16|uint32(eg)<<8|uint32(eb))
				}
			}
		}
	}
}

func blend(base, overlay, alpha uint8) uint8 {
	return uint8((int(base)*(255-int(alpha)) + int(overlay)*int(alpha)) / 255)
}
