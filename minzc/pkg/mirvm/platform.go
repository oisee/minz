// Package mirvm implements platform abstractions for the MIR virtual machine
package mirvm

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
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

// SavePNG exports the framebuffer to a PNG file
func (d *GenericDisplay) SavePNG(filename string) error {
	img := image.NewRGBA(image.Rect(0, 0, d.width, d.height))

	for y := 0; y < d.height; y++ {
		for x := 0; x < d.width; x++ {
			pixel := d.GetPixel(x, y)
			r := uint8((pixel >> 16) & 0xFF)
			g := uint8((pixel >> 8) & 0xFF)
			b := uint8(pixel & 0xFF)
			a := uint8((pixel >> 24) & 0xFF)
			if a == 0 {
				a = 255 // Default to opaque if alpha not set
			}
			img.Set(x, y, color.RGBA{R: r, G: g, B: b, A: a})
		}
	}

	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer file.Close()

	if err := png.Encode(file, img); err != nil {
		return fmt.Errorf("encode PNG: %w", err)
	}

	return nil
}

// ToImage converts the framebuffer to an image.Image
func (d *GenericDisplay) ToImage() image.Image {
	img := image.NewRGBA(image.Rect(0, 0, d.width, d.height))

	for y := 0; y < d.height; y++ {
		for x := 0; x < d.width; x++ {
			pixel := d.GetPixel(x, y)
			r := uint8((pixel >> 16) & 0xFF)
			g := uint8((pixel >> 8) & 0xFF)
			b := uint8(pixel & 0xFF)
			a := uint8((pixel >> 24) & 0xFF)
			if a == 0 {
				a = 255
			}
			img.Set(x, y, color.RGBA{R: r, G: g, B: b, A: a})
		}
	}

	return img
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

// =============================================================================
// VDP - Video Display Processor (Agon Light style)
// =============================================================================

// VDP implements a BBC Micro / Agon Light style command processor
type VDP struct {
	display     *GenericDisplay

	// VDU command state
	cmdBuffer   []byte
	cmdExpected int
	cmdID       byte

	// Graphics state
	graphicsX   int16  // Current graphics cursor X
	graphicsY   int16  // Current graphics cursor Y
	originX     int16  // Graphics origin X
	originY     int16  // Graphics origin Y
	foreground  uint32 // Current foreground color
	background  uint32 // Current background color

	// Text state
	textX       int
	textY       int
	textFG      uint32
	textBG      uint32

	// Screen mode
	mode        int

	// Sprites (simplified - up to 64 sprites)
	sprites     [64]*Sprite
	curSprite   int
}

// Sprite represents a hardware sprite
type Sprite struct {
	X, Y    int16
	Width   int
	Height  int
	Visible bool
	Bitmap  []byte
}

// NewVDP creates a new VDP attached to a display
func NewVDP(display *GenericDisplay) *VDP {
	return &VDP{
		display:    display,
		foreground: 0xFFFFFFFF, // White
		background: 0xFF000000, // Black
		textFG:     0xFFFFFFFF,
		textBG:     0xFF000000,
		mode:       0,
	}
}

// ProcessByte processes a single byte of VDU output
// Returns true if the byte was consumed as part of a VDU command
func (v *VDP) ProcessByte(b byte) bool {
	// If we're collecting a multi-byte command
	if v.cmdExpected > 0 {
		v.cmdBuffer = append(v.cmdBuffer, b)
		v.cmdExpected--

		if v.cmdExpected == 0 {
			v.executeCommand()
			v.cmdBuffer = nil
			v.cmdID = 0
		}
		return true
	}

	// Check for VDU command start
	switch b {
	case 0: // NUL - do nothing
		return true
	case 4: // VDU 4 - Text at text cursor
		return true
	case 5: // VDU 5 - Text at graphics cursor
		return true
	case 7: // BEL - beep
		return true
	case 8: // Backspace
		if v.textX > 0 {
			v.textX--
		}
		return true
	case 9: // Tab
		v.textX = (v.textX + 8) &^ 7
		return true
	case 10: // Line feed
		v.textY++
		return true
	case 11: // VDU 11 - cursor up
		if v.textY > 0 {
			v.textY--
		}
		return true
	case 12: // CLS
		v.display.Clear(v.background)
		v.textX, v.textY = 0, 0
		return true
	case 13: // Carriage return
		v.textX = 0
		return true
	case 16: // VDU 16 - CLG (clear graphics)
		v.display.Clear(v.background)
		return true
	case 17: // VDU 17,c - set text colour
		v.cmdID = 17
		v.cmdExpected = 1
		return true
	case 18: // VDU 18,m,c - set graphics colour
		v.cmdID = 18
		v.cmdExpected = 2
		return true
	case 22: // VDU 22,n - set screen mode
		v.cmdID = 22
		v.cmdExpected = 1
		return true
	case 23: // VDU 23 - multi-purpose command
		v.cmdID = 23
		v.cmdExpected = 9 // VDU 23 takes 9 bytes
		return true
	case 25: // VDU 25,k,x;y; - PLOT
		v.cmdID = 25
		v.cmdExpected = 5
		return true
	case 29: // VDU 29,x;y; - set graphics origin
		v.cmdID = 29
		v.cmdExpected = 4
		return true
	case 30: // Home cursor
		v.textX, v.textY = 0, 0
		return true
	case 31: // VDU 31,x,y - position cursor
		v.cmdID = 31
		v.cmdExpected = 2
		return true
	default:
		// Printable character
		if b >= 32 && b < 127 {
			// Simple character output (would need font data for real rendering)
			// For now, just advance the cursor
			v.textX++
			if v.textX >= v.display.Width()/8 {
				v.textX = 0
				v.textY++
			}
		}
		return false
	}
}

// executeCommand executes a buffered VDU command
func (v *VDP) executeCommand() {
	switch v.cmdID {
	case 17: // Set text color
		// v.cmdBuffer[0] = color index
		v.textFG = v.display.palette[v.cmdBuffer[0]]

	case 18: // Set graphics color
		// v.cmdBuffer[0] = mode, v.cmdBuffer[1] = color
		if v.cmdBuffer[0] == 0 { // Foreground
			v.foreground = v.display.palette[v.cmdBuffer[1]]
		} else { // Background
			v.background = v.display.palette[v.cmdBuffer[1]]
		}

	case 22: // Set screen mode
		v.mode = int(v.cmdBuffer[0])
		// Different modes would configure different resolutions
		// For now just clear the screen
		v.display.Clear(v.background)
		v.textX, v.textY = 0, 0
		v.graphicsX, v.graphicsY = 0, 0

	case 23: // Multi-purpose VDU 23
		v.executeVDU23()

	case 25: // PLOT command
		v.executePLOT()

	case 29: // Set graphics origin
		v.originX = int16(v.cmdBuffer[0]) | int16(v.cmdBuffer[1])<<8
		v.originY = int16(v.cmdBuffer[2]) | int16(v.cmdBuffer[3])<<8

	case 31: // Position cursor
		v.textX = int(v.cmdBuffer[0])
		v.textY = int(v.cmdBuffer[1])
	}
}

// executeVDU23 handles VDU 23 commands
func (v *VDP) executeVDU23() {
	if len(v.cmdBuffer) < 1 {
		return
	}

	subCmd := v.cmdBuffer[0]
	switch subCmd {
	case 0: // System commands
		if len(v.cmdBuffer) >= 2 {
			v.executeSystemCommand()
		}
	case 27: // Sprite commands
		if len(v.cmdBuffer) >= 2 {
			v.executeSpriteCommand()
		}
	}
}

// executeSystemCommand handles VDU 23,0,n,... commands
func (v *VDP) executeSystemCommand() {
	// VDU 23,0,n,...
	// cmd = v.cmdBuffer[1]
	switch v.cmdBuffer[1] {
	case 0x80: // Get display info
		// Would return display dimensions, etc.
	case 0x81: // Get cursor position
		// Would return cursor X,Y
	case 0x86: // Reset VDP
		v.display.Clear(v.background)
		v.textX, v.textY = 0, 0
		v.graphicsX, v.graphicsY = 0, 0
		v.originX, v.originY = 0, 0
	}
}

// executeSpriteCommand handles VDU 23,27,n,... commands
func (v *VDP) executeSpriteCommand() {
	// VDU 23,27,cmd,...
	cmd := v.cmdBuffer[1]
	switch cmd {
	case 0: // Select sprite
		if len(v.cmdBuffer) >= 3 {
			v.curSprite = int(v.cmdBuffer[2])
			if v.curSprite >= 64 {
				v.curSprite = 0
			}
		}
	case 1: // Clear all sprites
		for i := range v.sprites {
			v.sprites[i] = nil
		}
	case 4: // Define sprite
		// Would define sprite bitmap data
	case 5: // Move sprite absolute
		if sprite := v.sprites[v.curSprite]; sprite != nil && len(v.cmdBuffer) >= 6 {
			sprite.X = int16(v.cmdBuffer[2]) | int16(v.cmdBuffer[3])<<8
			sprite.Y = int16(v.cmdBuffer[4]) | int16(v.cmdBuffer[5])<<8
		}
	case 11: // Show sprite
		if v.curSprite < 64 && v.sprites[v.curSprite] != nil {
			v.sprites[v.curSprite].Visible = true
		}
	case 12: // Hide sprite
		if v.curSprite < 64 && v.sprites[v.curSprite] != nil {
			v.sprites[v.curSprite].Visible = false
		}
	}
}

// executePLOT handles VDU 25,k,x;y; commands
func (v *VDP) executePLOT() {
	if len(v.cmdBuffer) < 5 {
		return
	}

	plotType := v.cmdBuffer[0]
	x := int16(v.cmdBuffer[1]) | int16(v.cmdBuffer[2])<<8
	y := int16(v.cmdBuffer[3]) | int16(v.cmdBuffer[4])<<8

	// Apply origin offset
	x += v.originX
	y += v.originY

	// Plot type determines the operation
	switch plotType & 7 {
	case 0: // Move absolute
		v.graphicsX, v.graphicsY = x, y
	case 1: // Line to absolute (foreground)
		v.drawLine(int(v.graphicsX), int(v.graphicsY), int(x), int(y), v.foreground)
		v.graphicsX, v.graphicsY = x, y
	case 4: // Move relative
		v.graphicsX += x
		v.graphicsY += y
	case 5: // Line to relative
		newX := v.graphicsX + x
		newY := v.graphicsY + y
		v.drawLine(int(v.graphicsX), int(v.graphicsY), int(newX), int(newY), v.foreground)
		v.graphicsX, v.graphicsY = newX, newY
	}

	// Additional plot operations based on upper bits
	if plotType&0x38 != 0 {
		switch plotType & 0x38 {
		case 0x40: // Point
			v.display.SetPixel(int(x), int(y), v.foreground)
		case 0x50: // Triangle fill (would need more complex implementation)
		case 0x60: // Rectangle fill
			v.drawFilledRect(int(v.graphicsX), int(v.graphicsY), int(x), int(y), v.foreground)
		case 0x90: // Circle outline
			// v.drawCircle(int(x), int(y), radius, v.foreground)
		}
	}
}

// drawLine draws a line using Bresenham's algorithm
func (v *VDP) drawLine(x0, y0, x1, y1 int, color uint32) {
	dx := abs(x1 - x0)
	dy := -abs(y1 - y0)
	sx := 1
	if x0 > x1 {
		sx = -1
	}
	sy := 1
	if y0 > y1 {
		sy = -1
	}
	err := dx + dy

	for {
		v.display.SetPixel(x0, y0, color)

		if x0 == x1 && y0 == y1 {
			break
		}

		e2 := 2 * err
		if e2 >= dy {
			if x0 == x1 {
				break
			}
			err += dy
			x0 += sx
		}
		if e2 <= dx {
			if y0 == y1 {
				break
			}
			err += dx
			y0 += sy
		}
	}
}

// drawFilledRect draws a filled rectangle
func (v *VDP) drawFilledRect(x0, y0, x1, y1 int, color uint32) {
	if x0 > x1 {
		x0, x1 = x1, x0
	}
	if y0 > y1 {
		y0, y1 = y1, y0
	}

	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			v.display.SetPixel(x, y, color)
		}
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// =============================================================================
// Agon Platform with VDP
// =============================================================================

// AgonPlatform extends GenericPlatform with VDP support
type AgonPlatform struct {
	*GenericPlatform
	vdp *VDP
}

// NewAgonPlatform creates an Agon Light-like platform (VDP-based)
func NewAgonPlatform() *AgonPlatform {
	// Agon Light: eZ80 + ESP32 VDP
	// Default mode: 320x240
	generic := NewGenericPlatform("agon", 320, 240, 8)

	// Initialize Agon palette (similar to CGA/EGA)
	agonPalette := []uint32{
		0xFF000000, // 0: Black
		0xFFAA0000, // 1: Dark Red
		0xFF00AA00, // 2: Dark Green
		0xFFAAAA00, // 3: Dark Yellow
		0xFF0000AA, // 4: Dark Blue
		0xFFAA00AA, // 5: Dark Magenta
		0xFF00AAAA, // 6: Dark Cyan
		0xFFAAAAAA, // 7: Light Gray
		0xFF555555, // 8: Dark Gray
		0xFFFF5555, // 9: Bright Red
		0xFF55FF55, // 10: Bright Green
		0xFFFFFF55, // 11: Bright Yellow
		0xFF5555FF, // 12: Bright Blue
		0xFFFF55FF, // 13: Bright Magenta
		0xFF55FFFF, // 14: Bright Cyan
		0xFFFFFFFF, // 15: White
	}
	for i, c := range agonPalette {
		generic.display.SetPalette(i, c)
	}

	// Extend palette to 64 colors (Agon default)
	for i := 16; i < 64; i++ {
		r := uint32((i & 0x03) * 85)
		g := uint32(((i >> 2) & 0x03) * 85)
		b := uint32(((i >> 4) & 0x03) * 85)
		generic.display.SetPalette(i, 0xFF000000|r<<16|g<<8|b)
	}

	return &AgonPlatform{
		GenericPlatform: generic,
		vdp:             NewVDP(generic.display),
	}
}

// WriteChar overrides GenericPlatform to process VDU commands
func (p *AgonPlatform) WriteChar(b byte) {
	// First try to process as VDU command
	if !p.vdp.ProcessByte(b) {
		// If not a VDU command, output normally
		p.GenericPlatform.WriteChar(b)
	}
}

// VDP returns the VDP instance for direct access
func (p *AgonPlatform) VDP() *VDP {
	return p.vdp
}
