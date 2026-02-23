package spectrum

// ZX Spectrum 16-color palette: 8 normal + 8 bright colors.
// Each entry is [R, G, B, A].
var spectrumPalette = [16][4]byte{
	// Normal
	{0x00, 0x00, 0x00, 0xFF}, // 0: black
	{0x00, 0x00, 0xCD, 0xFF}, // 1: blue
	{0xCD, 0x00, 0x00, 0xFF}, // 2: red
	{0xCD, 0x00, 0xCD, 0xFF}, // 3: magenta
	{0x00, 0xCD, 0x00, 0xFF}, // 4: green
	{0x00, 0xCD, 0xCD, 0xFF}, // 5: cyan
	{0xCD, 0xCD, 0x00, 0xFF}, // 6: yellow
	{0xCD, 0xCD, 0xCD, 0xFF}, // 7: white
	// Bright
	{0x00, 0x00, 0x00, 0xFF}, // 8: bright black (same)
	{0x00, 0x00, 0xFF, 0xFF}, // 9: bright blue
	{0xFF, 0x00, 0x00, 0xFF}, // 10: bright red
	{0xFF, 0x00, 0xFF, 0xFF}, // 11: bright magenta
	{0x00, 0xFF, 0x00, 0xFF}, // 12: bright green
	{0x00, 0xFF, 0xFF, 0xFF}, // 13: bright cyan
	{0xFF, 0xFF, 0x00, 0xFF}, // 14: bright yellow
	{0xFF, 0xFF, 0xFF, 0xFF}, // 15: bright white
}

// ULA handles ZX Spectrum display rendering using the FrameMap approach.
type ULA struct {
	frameMap    []FrameEntry
	framebuffer []byte // RGBA pixels (width * height * 4)
	borderColor byte   // 0-7 (3 bits)
	flashState  bool   // toggles every 16 frames
	flashCount  int    // frame counter for flash

	// Cached fetch data (used between FetchBitmap and ScreenPixel)
	lastBitmap byte
	lastAttr   byte

	mode   *VideoMode
	memory *Memory

	// Current rendering position
	lastTState int
}

// NewULA creates a ULA renderer for the given mode and memory.
func NewULA(mode *VideoMode, mem *Memory) *ULA {
	return &ULA{
		frameMap:    GenerateFrameMap(mode),
		framebuffer: make([]byte, mode.TotalPixelWidth*mode.TotalPixelHeight*4),
		mode:        mode,
		memory:      mem,
	}
}

// Framebuffer returns the RGBA pixel data for the current frame.
func (u *ULA) Framebuffer() []byte {
	return u.framebuffer
}

// SetBorderColor sets the current border color (0-7).
func (u *ULA) SetBorderColor(color byte) {
	u.borderColor = color & 0x07
}

// BorderColor returns the current border color.
func (u *ULA) BorderColor() byte {
	return u.borderColor
}

// StepTo advances ULA rendering from the last position to the given T-state.
// Called after each CPU instruction to keep the display in sync.
func (u *ULA) StepTo(tstate int) {
	if tstate > len(u.frameMap) {
		tstate = len(u.frameMap)
	}

	for t := u.lastTState; t < tstate; t++ {
		entry := &u.frameMap[t]
		switch entry.Action {
		case ActionNone:
			continue

		case ActionFetchBitmap:
			// Read bitmap byte from VRAM (internal — no contention)
			offset := entry.Addr - 0x4000
			u.lastBitmap = u.memory.ReadScreen(offset)

		case ActionFetchAttr:
			// Read attribute byte from VRAM
			offset := entry.Addr - 0x4000
			u.lastAttr = u.memory.ReadScreen(offset)

		case ActionScreenPixel:
			u.renderScreenPixels(entry.X, entry.Y)

		case ActionBorderPixel:
			u.renderBorderPixels(entry.X, entry.Y)
		}
	}

	u.lastTState = tstate
}

// renderScreenPixels renders 8 pixels from the cached bitmap and attribute.
func (u *ULA) renderScreenPixels(x, y int) {
	attr := u.lastAttr
	bitmap := u.lastBitmap

	ink := attr & 0x07
	paper := (attr >> 3) & 0x07
	bright := attr & 0x40
	flash := attr & 0x80

	// Apply bright offset
	if bright != 0 {
		ink += 8
		paper += 8
	}

	// Apply flash (swap ink/paper)
	if flash != 0 && u.flashState {
		ink, paper = paper, ink
	}

	inkColor := spectrumPalette[ink]
	paperColor := spectrumPalette[paper]

	stride := u.mode.TotalPixelWidth * 4

	for bit := 7; bit >= 0; bit-- {
		px := x + (7 - bit)
		if px < 0 || px >= u.mode.TotalPixelWidth {
			continue
		}

		offset := y*stride + px*4
		if offset+3 >= len(u.framebuffer) {
			continue
		}

		if bitmap&(1<<uint(bit)) != 0 {
			u.framebuffer[offset+0] = inkColor[0]
			u.framebuffer[offset+1] = inkColor[1]
			u.framebuffer[offset+2] = inkColor[2]
			u.framebuffer[offset+3] = inkColor[3]
		} else {
			u.framebuffer[offset+0] = paperColor[0]
			u.framebuffer[offset+1] = paperColor[1]
			u.framebuffer[offset+2] = paperColor[2]
			u.framebuffer[offset+3] = paperColor[3]
		}
	}
}

// renderBorderPixels renders 8 pixels of border color.
func (u *ULA) renderBorderPixels(x, y int) {
	color := spectrumPalette[u.borderColor]
	stride := u.mode.TotalPixelWidth * 4

	for i := 0; i < 8; i++ {
		px := x + i
		if px < 0 || px >= u.mode.TotalPixelWidth {
			continue
		}
		if y < 0 || y >= u.mode.TotalPixelHeight {
			continue
		}

		offset := y*stride + px*4
		if offset+3 >= len(u.framebuffer) {
			continue
		}

		u.framebuffer[offset+0] = color[0]
		u.framebuffer[offset+1] = color[1]
		u.framebuffer[offset+2] = color[2]
		u.framebuffer[offset+3] = color[3]
	}
}

// EndFrame finalizes the frame: toggles flash state, resets position.
func (u *ULA) EndFrame() {
	u.flashCount++
	if u.flashCount >= 16 {
		u.flashCount = 0
		u.flashState = !u.flashState
	}
	u.lastTState = 0
}

// ClearFramebuffer fills the framebuffer with the border color.
func (u *ULA) ClearFramebuffer() {
	color := spectrumPalette[u.borderColor]
	for i := 0; i < len(u.framebuffer); i += 4 {
		u.framebuffer[i+0] = color[0]
		u.framebuffer[i+1] = color[1]
		u.framebuffer[i+2] = color[2]
		u.framebuffer[i+3] = color[3]
	}
}

// RenderFullScreen renders the complete screen from VRAM (non-incremental).
// Useful for snapshot loading or when you need a clean render.
func (u *ULA) RenderFullScreen() {
	u.ClearFramebuffer()

	for charRow := 0; charRow < 24; charRow++ {
		for charCol := 0; charCol < 32; charCol++ {
			// Read attribute
			attrAddr := 0x5800 + charRow*32 + charCol
			attr := u.memory.ReadScreen(uint16(attrAddr - 0x4000))

			ink := attr & 0x07
			paper := (attr >> 3) & 0x07
			bright := attr & 0x40
			flash := attr & 0x80

			if bright != 0 {
				ink += 8
				paper += 8
			}
			if flash != 0 && u.flashState {
				ink, paper = paper, ink
			}

			inkColor := spectrumPalette[ink]
			paperColor := spectrumPalette[paper]

			for pixRow := 0; pixRow < 8; pixRow++ {
				pixelLine := charRow*8 + pixRow
				bitmapAddr := screenBitmapAddr(pixelLine, charCol)
				bitmap := u.memory.ReadScreen(bitmapAddr - 0x4000)

				y := u.mode.BorderTop + pixelLine
				stride := u.mode.TotalPixelWidth * 4

				for bit := 7; bit >= 0; bit-- {
					px := u.mode.BorderLeft + charCol*8 + (7 - bit)
					offset := y*stride + px*4
					if offset+3 >= len(u.framebuffer) {
						continue
					}

					if bitmap&(1<<uint(bit)) != 0 {
						u.framebuffer[offset+0] = inkColor[0]
						u.framebuffer[offset+1] = inkColor[1]
						u.framebuffer[offset+2] = inkColor[2]
						u.framebuffer[offset+3] = inkColor[3]
					} else {
						u.framebuffer[offset+0] = paperColor[0]
						u.framebuffer[offset+1] = paperColor[1]
						u.framebuffer[offset+2] = paperColor[2]
						u.framebuffer[offset+3] = paperColor[3]
					}
				}
			}
		}
	}
}
