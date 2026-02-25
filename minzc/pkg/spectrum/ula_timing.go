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
	FirstScreenTState: 24, // 24 T-states = 48 pixels of left border before screen data
	ContentionPattern: []int{6, 5, 4, 3, 2, 1, 0, 0},
	BorderTop:         48, // lines 16..63
	BorderBottom:      56, // lines 256..311
	BorderLeft:        48, // pixels (= FirstScreenTState * 2)
	BorderRight:       48, // pixels
	TotalPixelWidth:   352, // 48 + 256 + 48
	TotalPixelHeight:  296, // 48 + 192 + 56
	ScreenWidth:       256,
	ScreenHeight:      192,
}

// ModePentagon128 is the Pentagon 128K timing (no contention).
// Timing values from FUSE/libspectrum: top_left_pixel_tstates = 17988,
// which is line 80, col 68 within each 224 T-state scanline.
var ModePentagon128 = &VideoMode{
	Name:              "Pentagon128",
	CPUClockHz:        3584000,
	TStatesPerLine:    224,
	LinesPerFrame:     320,
	FirstScreenLine:   80,
	FirstScreenTState: 66, // Adjusted from FUSE ref 68 (17988%224) — shifted 4px right to match Pentagon hardware
	ContentionPattern: nil, // no contention
	BorderTop:         64,
	BorderBottom:      48,
	BorderLeft:        48,
	BorderRight:       48,
	TotalPixelWidth:   352,
	TotalPixelHeight:  304, // 64 + 192 + 48
	ScreenWidth:       256,
	ScreenHeight:      192,
}

// FrameAction is the type of ULA action at a given T-state.
type FrameAction byte

const (
	ActionNone              FrameAction = iota
	ActionFetchBitmap                   // ULA fetches bitmap byte from VRAM
	ActionFetchAttr                     // ULA fetches attribute byte from VRAM
	ActionScreenPixel                   // Render 8 screen pixels using cached bitmap+attr
	ActionBorderPixel                   // Render 2 border pixels (1 T-state = 2 pixels)
	ActionFetchBitmapBorder             // Pre-fetch bitmap AND render 2 border pixels (pipeline)
	ActionFetchAttrBorder               // Pre-fetch attr AND render 2 border pixels (pipeline)
)

// FrameEntry describes what the ULA does at a given T-state.
type FrameEntry struct {
	Action FrameAction
	X, Y   int    // pixel position in framebuffer
	Addr   uint16 // VRAM address (for fetch actions)
}

// GenerateFrameMap builds a per-T-state ULA action table for the given mode.
// Each entry in the returned slice corresponds to one T-state in the frame.
// Screen pixels fire every 4 T-states (character cell).
// Border pixels fire every T-state (2 pixels per T-state) for full resolution.
//
// ULA pipeline: on real hardware, the ULA pre-fetches bitmap+attr 2 T-states
// before outputting screen pixels. We model this by fetching charCol 0's data
// during the last 2 T-states of the left border (screenCol -2/-1), and for
// subsequent chars, fetching at phase 2/3 of the previous char's cycle.
// This aligns screen pixel output with border pixel timing (no 4-pixel tooth).
//
// The visible area doesn't necessarily start at col 0 within each T-state line.
// On Pentagon, FirstScreenTState=68, so the leftmost visible pixel is at
// col 44 (= 68 - BorderLeft/2). The visCol approach handles this correctly
// for all models by computing pixel X relative to the first visible column.
func GenerateFrameMap(mode *VideoMode) []FrameEntry {
	total := mode.TStatesPerFrame()
	fmap := make([]FrameEntry, total)

	// First visible column: the T-state column where the leftmost border pixel is.
	// For Pentagon: 68 - 24 = 44. For 48K: 24 - 24 = 0.
	firstVisCol := mode.FirstScreenTState - mode.BorderLeft/2

	for t := 0; t < total; t++ {
		line := t / mode.TStatesPerLine
		col := t % mode.TStatesPerLine

		// Which pixel row are we on relative to the display?
		displayLine := line - mode.FirstScreenLine

		// Screen data region: 192 lines of screen content
		if displayLine >= 0 && displayLine < 192 {
			// screenCol is relative to FirstScreenTState (where charCol 0 renders)
			screenCol := col - mode.FirstScreenTState

			if screenCol == -2 {
				// Pipeline pre-fetch: bitmap for charCol 0 + render left border pixel
				addr := screenBitmapAddr(displayLine, 0)
				pixelX := mode.BorderLeft + screenCol*2
				pixelY := mode.BorderTop + displayLine
				fmap[t] = FrameEntry{
					Action: ActionFetchBitmapBorder,
					Addr:   addr,
					X:      pixelX,
					Y:      pixelY,
				}
			} else if screenCol == -1 {
				// Pipeline pre-fetch: attr for charCol 0 + render left border pixel
				addr := screenAttrAddr(displayLine, 0)
				pixelX := mode.BorderLeft + screenCol*2
				pixelY := mode.BorderTop + displayLine
				fmap[t] = FrameEntry{
					Action: ActionFetchAttrBorder,
					Addr:   addr,
					X:      pixelX,
					Y:      pixelY,
				}
			} else if screenCol >= 0 && screenCol < 128 {
				// Within screen area: pipelined pattern per 4-T-state character cell
				//   phase 0: ScreenPixel (render 8 pixels using previously fetched data)
				//   phase 1: idle
				//   phase 2: FetchBitmap for next character (charCol+1)
				//   phase 3: FetchAttr for next character (charCol+1)
				phase := screenCol % 4

				switch phase {
				case 0:
					// Render 8 screen pixels
					charCol := screenCol / 4
					pixelX := mode.BorderLeft + charCol*8
					pixelY := mode.BorderTop + displayLine
					fmap[t] = FrameEntry{
						Action: ActionScreenPixel,
						X:      pixelX,
						Y:      pixelY,
					}
				case 2:
					// Pre-fetch bitmap for next character
					nextCharCol := screenCol/4 + 1
					if nextCharCol < 32 {
						addr := screenBitmapAddr(displayLine, nextCharCol)
						fmap[t] = FrameEntry{
							Action: ActionFetchBitmap,
							Addr:   addr,
						}
					}
				case 3:
					// Pre-fetch attr for next character
					nextCharCol := screenCol/4 + 1
					if nextCharCol < 32 {
						addr := screenAttrAddr(displayLine, nextCharCol)
						fmap[t] = FrameEntry{
							Action: ActionFetchAttr,
							Addr:   addr,
						}
					}
				default:
					// phase 1: idle
				}
			} else if screenCol >= -mode.BorderLeft/2 && screenCol < -2 {
				// Left border during screen lines (excluding pre-fetch T-states)
				pixelX := mode.BorderLeft + screenCol*2
				pixelY := mode.BorderTop + displayLine
				if pixelX >= 0 {
					fmap[t] = FrameEntry{
						Action: ActionBorderPixel,
						X:      pixelX,
						Y:      pixelY,
					}
				}
			} else if screenCol >= 128 && screenCol < 128+mode.BorderRight/2 {
				// Right border during screen lines (2 pixels per T-state)
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
		} else if displayLine >= -mode.BorderTop && displayLine < 0 {
			// Top border: use visCol to map T-state column to pixel X
			visCol := (col - firstVisCol + mode.TStatesPerLine) % mode.TStatesPerLine
			if visCol < mode.TotalPixelWidth/2 {
				pixelX := visCol * 2
				pixelY := mode.BorderTop + displayLine
				if pixelY >= 0 {
					fmap[t] = FrameEntry{
						Action: ActionBorderPixel,
						X:      pixelX,
						Y:      pixelY,
					}
				}
			}
		} else if displayLine >= 192 && displayLine < 192+mode.BorderBottom {
			// Bottom border: use visCol to map T-state column to pixel X
			visCol := (col - firstVisCol + mode.TStatesPerLine) % mode.TStatesPerLine
			if visCol < mode.TotalPixelWidth/2 {
				pixelX := visCol * 2
				pixelY := mode.BorderTop + displayLine
				if pixelY < mode.TotalPixelHeight {
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
