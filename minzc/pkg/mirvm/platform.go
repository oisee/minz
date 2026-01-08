// Package mirvm implements platform abstractions for the MIR virtual machine
package mirvm

import (
	"fmt"
	"io"
	"os"
)

// Platform defines the hardware abstraction interface
type Platform interface {
	// Identity
	Name() string

	// I/O Ports (Z80-style)
	PortIn(port uint16) byte
	PortOut(port uint16, value byte)

	// Memory-Mapped I/O
	IsMMIO(addr uint32) bool
	ReadMMIO(addr uint32) byte
	WriteMMIO(addr uint32, value byte)

	// Display (if available)
	HasDisplay() bool
	Display() Display

	// Terminal I/O
	ReadChar() (byte, bool)   // Returns char and ok
	WriteChar(b byte)
	WriteString(s string)

	// System
	Exit(code int)
	Tick(cycles int)  // Called each instruction for timing
}

// Display interface for platforms with graphics
type Display interface {
	Width() int
	Height() int
	BPP() int  // Bits per pixel (1, 2, 4, 8, 16, 24, 32)

	// Framebuffer access
	GetPixel(x, y int) uint32
	SetPixel(x, y int, color uint32)
	Clear(color uint32)

	// Buffer management
	GetFramebuffer() []byte
	Refresh()  // Signal display update
}

// =============================================================================
// GenericPlatform - Simple platform with framebuffer, I/O ports, and terminal
// =============================================================================

// GenericPlatform implements a basic platform suitable for testing
type GenericPlatform struct {
	name        string
	display     *GenericDisplay
	stdout      io.Writer
	stdin       io.Reader
	ports       [256]byte      // 256 I/O ports
	exitCode    int
	exited      bool
	cycleCount  int64

	// Callbacks for custom I/O
	onPortOut   func(port uint16, value byte)
	onPortIn    func(port uint16) byte
}

// GenericDisplay implements a simple framebuffer display
type GenericDisplay struct {
	width       int
	height      int
	bpp         int
	framebuffer []byte
	palette     []uint32  // For indexed color modes
	dirty       bool
}

// NewGenericPlatform creates a new generic platform
func NewGenericPlatform(name string, width, height, bpp int) *GenericPlatform {
	p := &GenericPlatform{
		name:   name,
		stdout: os.Stdout,
		stdin:  os.Stdin,
	}

	if width > 0 && height > 0 {
		p.display = NewGenericDisplay(width, height, bpp)
	}

	return p
}

// NewGenericDisplay creates a new framebuffer display
func NewGenericDisplay(width, height, bpp int) *GenericDisplay {
	bytesPerPixel := (bpp + 7) / 8
	if bytesPerPixel == 0 {
		bytesPerPixel = 1
	}

	d := &GenericDisplay{
		width:       width,
		height:      height,
		bpp:         bpp,
		framebuffer: make([]byte, width*height*bytesPerPixel),
		palette:     make([]uint32, 256),  // Default 256-color palette
	}

	// Initialize default grayscale palette
	for i := 0; i < 256; i++ {
		d.palette[i] = uint32(i) | uint32(i)<<8 | uint32(i)<<16 | 0xFF000000
	}

	return d
}

// Platform interface implementation for GenericPlatform

func (p *GenericPlatform) Name() string { return p.name }

func (p *GenericPlatform) PortIn(port uint16) byte {
	if p.onPortIn != nil {
		return p.onPortIn(port)
	}
	// Default: return port value from internal array (low 8 bits of port)
	return p.ports[port&0xFF]
}

func (p *GenericPlatform) PortOut(port uint16, value byte) {
	p.ports[port&0xFF] = value

	if p.onPortOut != nil {
		p.onPortOut(port, value)
	}

	// Built-in port behaviors
	switch port & 0xFF {
	case 0x01:  // Terminal output port
		p.WriteChar(value)
	case 0x02:  // Exit port
		p.exitCode = int(value)
		p.exited = true
	}
}

func (p *GenericPlatform) IsMMIO(addr uint32) bool {
	// Default: framebuffer at 0xF00000-0xFFFFFF (last 1MB)
	return addr >= 0xF00000
}

func (p *GenericPlatform) ReadMMIO(addr uint32) byte {
	if p.display == nil {
		return 0
	}

	offset := addr - 0xF00000
	if int(offset) < len(p.display.framebuffer) {
		return p.display.framebuffer[offset]
	}
	return 0
}

func (p *GenericPlatform) WriteMMIO(addr uint32, value byte) {
	if p.display == nil {
		return
	}

	offset := addr - 0xF00000
	if int(offset) < len(p.display.framebuffer) {
		p.display.framebuffer[offset] = value
		p.display.dirty = true
	}
}

func (p *GenericPlatform) HasDisplay() bool {
	return p.display != nil
}

func (p *GenericPlatform) Display() Display {
	return p.display
}

func (p *GenericPlatform) ReadChar() (byte, bool) {
	buf := make([]byte, 1)
	n, err := p.stdin.Read(buf)
	if err != nil || n == 0 {
		return 0, false
	}
	return buf[0], true
}

func (p *GenericPlatform) WriteChar(b byte) {
	p.stdout.Write([]byte{b})
}

func (p *GenericPlatform) WriteString(s string) {
	fmt.Fprint(p.stdout, s)
}

func (p *GenericPlatform) Exit(code int) {
	p.exitCode = code
	p.exited = true
}

func (p *GenericPlatform) Tick(cycles int) {
	p.cycleCount += int64(cycles)
}

func (p *GenericPlatform) HasExited() bool {
	return p.exited
}

func (p *GenericPlatform) ExitCode() int {
	return p.exitCode
}

func (p *GenericPlatform) CycleCount() int64 {
	return p.cycleCount
}

// SetOutput sets the output writer for terminal I/O
func (p *GenericPlatform) SetOutput(w io.Writer) {
	p.stdout = w
}

// SetInput sets the input reader for terminal I/O
func (p *GenericPlatform) SetInput(r io.Reader) {
	p.stdin = r
}

// SetPortOutHandler sets a custom handler for port output
func (p *GenericPlatform) SetPortOutHandler(handler func(port uint16, value byte)) {
	p.onPortOut = handler
}

// SetPortInHandler sets a custom handler for port input
func (p *GenericPlatform) SetPortInHandler(handler func(port uint16) byte) {
	p.onPortIn = handler
}

// Display interface implementation for GenericDisplay

func (d *GenericDisplay) Width() int  { return d.width }
func (d *GenericDisplay) Height() int { return d.height }
func (d *GenericDisplay) BPP() int    { return d.bpp }

func (d *GenericDisplay) GetPixel(x, y int) uint32 {
	if x < 0 || x >= d.width || y < 0 || y >= d.height {
		return 0
	}

	switch d.bpp {
	case 8:
		idx := y*d.width + x
		return d.palette[d.framebuffer[idx]]
	case 16:
		idx := (y*d.width + x) * 2
		// RGB565 format
		lo := uint16(d.framebuffer[idx])
		hi := uint16(d.framebuffer[idx+1])
		rgb565 := lo | (hi << 8)
		r := ((rgb565 >> 11) & 0x1F) << 3
		g := ((rgb565 >> 5) & 0x3F) << 2
		b := (rgb565 & 0x1F) << 3
		return uint32(r)<<16 | uint32(g)<<8 | uint32(b) | 0xFF000000
	case 24:
		idx := (y*d.width + x) * 3
		return uint32(d.framebuffer[idx]) |
			   uint32(d.framebuffer[idx+1])<<8 |
			   uint32(d.framebuffer[idx+2])<<16 |
			   0xFF000000
	case 32:
		idx := (y*d.width + x) * 4
		return uint32(d.framebuffer[idx]) |
			   uint32(d.framebuffer[idx+1])<<8 |
			   uint32(d.framebuffer[idx+2])<<16 |
			   uint32(d.framebuffer[idx+3])<<24
	default:
		return 0
	}
}

func (d *GenericDisplay) SetPixel(x, y int, color uint32) {
	if x < 0 || x >= d.width || y < 0 || y >= d.height {
		return
	}

	d.dirty = true

	switch d.bpp {
	case 8:
		idx := y*d.width + x
		// Find closest palette entry (simplified - just use low byte)
		d.framebuffer[idx] = byte(color)
	case 16:
		idx := (y*d.width + x) * 2
		// Convert to RGB565
		r := (color >> 16) & 0xFF
		g := (color >> 8) & 0xFF
		b := color & 0xFF
		rgb565 := uint16((r>>3)<<11) | uint16((g>>2)<<5) | uint16(b>>3)
		d.framebuffer[idx] = byte(rgb565)
		d.framebuffer[idx+1] = byte(rgb565 >> 8)
	case 24:
		idx := (y*d.width + x) * 3
		d.framebuffer[idx] = byte(color)
		d.framebuffer[idx+1] = byte(color >> 8)
		d.framebuffer[idx+2] = byte(color >> 16)
	case 32:
		idx := (y*d.width + x) * 4
		d.framebuffer[idx] = byte(color)
		d.framebuffer[idx+1] = byte(color >> 8)
		d.framebuffer[idx+2] = byte(color >> 16)
		d.framebuffer[idx+3] = byte(color >> 24)
	}
}

func (d *GenericDisplay) Clear(color uint32) {
	for y := 0; y < d.height; y++ {
		for x := 0; x < d.width; x++ {
			d.SetPixel(x, y, color)
		}
	}
	d.dirty = true
}

func (d *GenericDisplay) GetFramebuffer() []byte {
	return d.framebuffer
}

func (d *GenericDisplay) Refresh() {
	d.dirty = false
}

func (d *GenericDisplay) IsDirty() bool {
	return d.dirty
}

// SetPalette sets an entry in the color palette
func (d *GenericDisplay) SetPalette(index int, color uint32) {
	if index >= 0 && index < len(d.palette) {
		d.palette[index] = color
	}
}

// GetPalette returns the color palette
func (d *GenericDisplay) GetPalette() []uint32 {
	return d.palette
}

// =============================================================================
// Predefined Platform Configurations
// =============================================================================

// NewHeadlessPlatform creates a platform with no display (terminal only)
func NewHeadlessPlatform() *GenericPlatform {
	return NewGenericPlatform("headless", 0, 0, 0)
}

// NewTerminalPlatform creates a platform for text-mode applications
func NewTerminalPlatform() *GenericPlatform {
	p := NewGenericPlatform("terminal", 80, 25, 8)
	// 80x25 character display, 8-bit color per cell
	return p
}

// NewFramebufferPlatform creates a platform with a simple framebuffer
func NewFramebufferPlatform(width, height, bpp int) *GenericPlatform {
	return NewGenericPlatform(fmt.Sprintf("fb%dx%dx%d", width, height, bpp), width, height, bpp)
}

// NewSpectrumPlatform creates a ZX Spectrum-like platform (256x192, attribute-based)
func NewSpectrumPlatform() *GenericPlatform {
	p := NewGenericPlatform("spectrum", 256, 192, 8)
	// Set Spectrum palette
	spectrumColors := []uint32{
		0xFF000000, // Black
		0xFF0000D7, // Blue
		0xFFD70000, // Red
		0xFFD700D7, // Magenta
		0xFF00D700, // Green
		0xFF00D7D7, // Cyan
		0xFFD7D700, // Yellow
		0xFFD7D7D7, // White
		// Bright versions
		0xFF000000, // Black
		0xFF0000FF, // Bright Blue
		0xFFFF0000, // Bright Red
		0xFFFF00FF, // Bright Magenta
		0xFF00FF00, // Bright Green
		0xFF00FFFF, // Bright Cyan
		0xFFFFFF00, // Bright Yellow
		0xFFFFFFFF, // Bright White
	}
	for i, c := range spectrumColors {
		p.display.SetPalette(i, c)
	}
	return p
}

// NewAgonPlatform creates an Agon Light-like platform (VDP-based)
func NewAgonPlatform() *GenericPlatform {
	// Agon Light: eZ80 + ESP32 VDP
	// Default mode: 640x480 or 320x240
	p := NewGenericPlatform("agon", 320, 240, 8)
	// TODO: Add VDU command processor
	return p
}
