package spectrum

// VideoMode holds machine-specific timing parameters as pure data.
// Different Spectrum models have different ULA timing characteristics.
type VideoMode struct {
	Name            string
	CPUClockHz      int // 3500000 (48K) / 3546900 (Pentagon)
	TStatesPerLine  int // 224 (48K) / 224 (Pentagon)
	LinesPerFrame   int // 312 (48K) / 320 (Pentagon)
	FirstScreenLine int // 64 (48K) / 80 (Pentagon)
	FirstScreenTState int // T-state offset within a line where screen data starts

	// Contention pattern for ULA-contended memory ($4000-$7FFF).
	// nil means no contention (e.g. Pentagon).
	// For 48K: {6,5,4,3,2,1,0,0} repeating every 8 T-states.
	ContentionPattern []int

	// Border dimensions in pixels
	BorderTop    int
	BorderBottom int
	BorderLeft   int
	BorderRight  int

	// Total display dimensions including border (in pixels)
	TotalPixelWidth  int
	TotalPixelHeight int

	// Screen area: 256x192 pixels (32x24 characters, 8x8 each)
	ScreenWidth  int // always 256
	ScreenHeight int // always 192
}

// TStatesPerFrame returns total T-states in one frame.
func (m *VideoMode) TStatesPerFrame() int {
	return m.TStatesPerLine * m.LinesPerFrame
}

// Pre-defined video modes.

// Mode48K is the ZX Spectrum 48K ULA timing.
var Mode48K = &VideoMode{
	Name:              "48K",
	CPUClockHz:        3500000,
	TStatesPerLine:    224,
	LinesPerFrame:     312,
	FirstScreenLine:   64,
	FirstScreenTState: 14, // pixel data starts at T=14 within each line
	ContentionPattern: []int{6, 5, 4, 3, 2, 1, 0, 0},
	BorderTop:         48, // lines 16..63
	BorderBottom:      56, // lines 256..311
	BorderLeft:        48, // pixels
	BorderRight:       48, // pixels
	TotalPixelWidth:   352, // 48 + 256 + 48
	TotalPixelHeight:  296, // 48 + 192 + 56
	ScreenWidth:       256,
	ScreenHeight:      192,
}

// ModePentagon128 is the Pentagon 128K timing (no contention).
var ModePentagon128 = &VideoMode{
	Name:              "Pentagon128",
	CPUClockHz:        3546900,
	TStatesPerLine:    224,
	LinesPerFrame:     320,
	FirstScreenLine:   80,
	FirstScreenTState: 14,
	ContentionPattern: nil, // no contention
	BorderTop:         64,
	BorderBottom:      56,
	BorderLeft:        48,
	BorderRight:       48,
	TotalPixelWidth:   352,
	TotalPixelHeight:  312, // 64 + 192 + 56
	ScreenWidth:       256,
	ScreenHeight:      192,
}

// FrameAction is the type of ULA action at a given T-state.
type FrameAction byte

const (
	ActionNone        FrameAction = iota
	ActionFetchBitmap             // ULA fetches bitmap byte from VRAM
	ActionFetchAttr               // ULA fetches attribute byte from VRAM
	ActionScreenPixel             // Render 8 screen pixels using cached bitmap+attr
	ActionBorderPixel             // Render 8 border pixels
)

// FrameEntry describes what the ULA does at a given T-state.
type FrameEntry struct {
	Action FrameAction
	X, Y   int    // pixel position in framebuffer
	Addr   uint16 // VRAM address (for fetch actions)
}

// GenerateFrameMap builds a per-T-state ULA action table for the given mode.
// Each entry in the returned slice corresponds to one T-state in the frame.
// Actions fire every 4 T-states (one character cell = 8 pixels = 4+4 T-states).
func GenerateFrameMap(mode *VideoMode) []FrameEntry {
	total := mode.TStatesPerFrame()
	fmap := make([]FrameEntry, total)

	borderTopLines := mode.BorderTop / 1 // 1 pixel per line vertically in border
	_ = borderTopLines

	for t := 0; t < total; t++ {
		line := t / mode.TStatesPerLine
		col := t % mode.TStatesPerLine

		// Which pixel row are we on relative to the display?
		displayLine := line - mode.FirstScreenLine

		// Screen data region: 192 lines of screen content
		if displayLine >= 0 && displayLine < 192 {
			// Within screen area timing
			screenCol := col - mode.FirstScreenTState
			if screenCol >= 0 && screenCol < 128 { // 128 T-states for 256 pixels (2 pixels per T-state)
				charCol := screenCol / 4 // which character column (0-31)
				phase := screenCol % 4

				switch phase {
				case 0:
					// Fetch bitmap byte
					addr := screenBitmapAddr(displayLine, charCol)
					fmap[t] = FrameEntry{
						Action: ActionFetchBitmap,
						Addr:   addr,
					}
				case 1:
					// Fetch attribute byte
					addr := screenAttrAddr(displayLine, charCol)
					fmap[t] = FrameEntry{
						Action: ActionFetchAttr,
						Addr:   addr,
					}
				case 2:
					// Render 8 pixels
					pixelX := mode.BorderLeft + charCol*8
					pixelY := mode.BorderTop + displayLine
					fmap[t] = FrameEntry{
						Action: ActionScreenPixel,
						X:      pixelX,
						Y:      pixelY,
					}
				default:
					// phase 3: idle within this character cell
				}
			} else if screenCol >= -mode.BorderLeft/2 && screenCol < 0 {
				// Left border during screen lines
				if col%4 == 0 {
					pixelX := mode.BorderLeft + screenCol*2
					pixelY := mode.BorderTop + displayLine
					if pixelX >= 0 {
						fmap[t] = FrameEntry{
							Action: ActionBorderPixel,
							X:      pixelX,
							Y:      pixelY,
						}
					}
				}
			} else if screenCol >= 128 && screenCol < 128+mode.BorderRight/2 {
				// Right border during screen lines
				if col%4 == 0 {
					pixelX := mode.BorderLeft + 256 + (screenCol-128)*2
					pixelY := mode.BorderTop + displayLine
					if pixelX < mode.TotalPixelWidth {
						fmap[t] = FrameEntry{
							Action: ActionBorderPixel,
							X:      pixelX,
							Y:      pixelY,
						}
					}
				}
			}
		} else if displayLine >= -mode.BorderTop && displayLine < 0 {
			// Top border
			if col%4 == 0 && col < (mode.TotalPixelWidth/2) {
				pixelX := col * 2
				pixelY := mode.BorderTop + displayLine
				if pixelX < mode.TotalPixelWidth && pixelY >= 0 {
					fmap[t] = FrameEntry{
						Action: ActionBorderPixel,
						X:      pixelX,
						Y:      pixelY,
					}
				}
			}
		} else if displayLine >= 192 && displayLine < 192+mode.BorderBottom {
			// Bottom border
			if col%4 == 0 && col < (mode.TotalPixelWidth/2) {
				pixelX := col * 2
				pixelY := mode.BorderTop + displayLine
				if pixelX < mode.TotalPixelWidth && pixelY < mode.TotalPixelHeight {
					fmap[t] = FrameEntry{
						Action: ActionBorderPixel,
						X:      pixelX,
						Y:      pixelY,
					}
				}
			}
		}
	}

	return fmap
}

// screenBitmapAddr computes the VRAM address for a bitmap byte.
// ZX Spectrum screen layout: 010SSLLL RRRCCCCC
// SS = screen third (0-2), LLL = pixel row within char (0-7),
// RRR = character row within third (0-7), CCCCC = column (0-31)
func screenBitmapAddr(pixelRow, charCol int) uint16 {
	// pixelRow: 0-191
	third := pixelRow / 64        // 0-2
	rowInThird := (pixelRow % 64) / 8 // 0-7 (character row within third)
	pixelLine := pixelRow % 8     // 0-7 (pixel row within character)

	return 0x4000 | uint16(third<<11) | uint16(pixelLine<<8) | uint16(rowInThird<<5) | uint16(charCol)
}

// screenAttrAddr computes the VRAM address for an attribute byte.
// Attributes are linear: 0x5800 + (charRow * 32) + charCol
func screenAttrAddr(pixelRow, charCol int) uint16 {
	charRow := pixelRow / 8
	return 0x5800 + uint16(charRow*32) + uint16(charCol)
}

// GenerateContentionTable builds a delay lookup table for ContendRead.
// Index by (tstate % TStatesPerLine) to get the delay to add.
// Returns nil if the mode has no contention.
func GenerateContentionTable(mode *VideoMode) []int {
	if mode.ContentionPattern == nil {
		return nil
	}

	table := make([]int, mode.TStatesPerLine)
	patLen := len(mode.ContentionPattern)

	// Contention only applies during the 128 T-states of screen data fetch
	for i := 0; i < 128; i++ {
		tInLine := mode.FirstScreenTState + i
		if tInLine < mode.TStatesPerLine {
			table[tInLine] = mode.ContentionPattern[i%patLen]
		}
	}

	return table
}
