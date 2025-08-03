package emulator

import (
	"fmt"
	"strings"
)

// ZX Spectrum screen constants
const (
	SCREEN_WIDTH  = 32  // 32 characters per line
	SCREEN_HEIGHT = 24  // 24 lines
	SCREEN_SIZE   = 768 // 32x24 = 768 characters
	
	// ZX Spectrum system variables (in standard ROM)
	SYSVAR_BASE = 0x5C00
	CHARS       = 0x5C36 // Character set address (2 bytes)
	CHANS       = 0x5C4F // Channel information (2 bytes)
	CURCHL      = 0x5C51 // Current channel (2 bytes)
	PROG        = 0x5C53 // BASIC program start (2 bytes)
	NXTLIN      = 0x5C55 // Next line address (2 bytes)
	DATADD      = 0x5C57 // DATA address (2 bytes)
	E_LINE      = 0x5C59 // Edit line (2 bytes)
	K_CUR       = 0x5C5B // Keyboard cursor (2 bytes)
	CH_ADD      = 0x5C5D // Current character address (2 bytes)
	X_PTR       = 0x5C5F // Syntax error position (2 bytes)
	STKBOT      = 0x5C61 // Stack bottom (2 bytes)
	STKEND      = 0x5C63 // Stack end (2 bytes)
	BREG        = 0x5C67 // B register for calculator
	MEM         = 0x5C68 // Memory address for calculator (2 bytes)
	FLAGS2      = 0x5C6A // Various flags
	DF_SZ       = 0x5C6B // Display file size
	S_TOP       = 0x5C6C // Top line of screen (2 bytes)
	OLDPPC      = 0x5C6E // Old program counter (2 bytes)
	OSPCC       = 0x5C70 // Old stack pointer
	FLAGX       = 0x5C71 // Various flags
	STRLEN      = 0x5C72 // String length (2 bytes)
	T_ADDR      = 0x5C74 // Temporary address (2 bytes)
	SEED        = 0x5C76 // Random seed (2 bytes)
	FRAMES      = 0x5C78 // Frame counter (3 bytes)
	UDG         = 0x5C7B // User defined graphics (2 bytes)
	COORDS      = 0x5C7D // Coordinates (x, y)
	P_POSN      = 0x5C7F // Print position (column)
	PR_CC       = 0x5C80 // Print position address in display (2 bytes)
	ECHO_E      = 0x5C82 // Echo position (33 column, 24 line) (2 bytes)
	DF_CC       = 0x5C84 // Display file position (2 bytes)
	DF_CCL      = 0x5C86 // Display file column/line (2 bytes)
	S_POSN      = 0x5C88 // Screen position (column, line)
	S_POSNL     = 0x5C89 // Screen position line
	SPOSNL      = 0x5C89 // Alternative name
	ATTR_P      = 0x5C8D // Permanent attribute
	MASK_P      = 0x5C8E // Permanent mask
	ATTR_T      = 0x5C8F // Temporary attribute
	MASK_T      = 0x5C90 // Temporary mask
	P_FLAG      = 0x5C91 // Various flags
	MEMBOT      = 0x5C92 // Memory bottom (30 bytes for calculator)
	
	// RST routines
	RST_00 = 0x0000 // Reset
	RST_08 = 0x0008 // Error
	RST_10 = 0x0010 // Print character (RST 16)
	RST_18 = 0x0018 // Collect character
	RST_20 = 0x0020 // Collect next character
	RST_28 = 0x0028 // Calculator
	RST_30 = 0x0030 // Syntax tables
	RST_38 = 0x0038 // Interrupt
	
	// Special ZX ports (we can use these for debugging)
	PORT_DEBUG_OUT = 0x42 // Debug output port
	PORT_SCREEN    = 0x21 // Screen control port (invented)
)

// ZXScreen simulates a ZX Spectrum text screen
type ZXScreen struct {
	// Screen buffer (32x24 characters)
	buffer      [SCREEN_HEIGHT][SCREEN_WIDTH]byte
	
	// Cursor position
	cursorX     int
	cursorY     int
	
	// Attributes (for future color support)
	ink         byte
	paper       byte
	bright      bool
	flash       bool
	
	// System variables mirror
	sysvarMem   []byte // Mirror of system variables area
	
	// Output capture
	outputBuffer strings.Builder
	
	// Debug mode
	debugMode   bool
}

// NewZXScreen creates a new ZX Spectrum screen emulator
func NewZXScreen() *ZXScreen {
	s := &ZXScreen{
		cursorX: 0,
		cursorY: 0,
		ink:     7, // White
		paper:   0, // Black
		bright:  false,
		flash:   false,
		sysvarMem: make([]byte, 256), // System variables area
		debugMode: false,
	}
	s.Clear()
	return s
}

// Clear clears the screen
func (s *ZXScreen) Clear() {
	for y := 0; y < SCREEN_HEIGHT; y++ {
		for x := 0; x < SCREEN_WIDTH; x++ {
			s.buffer[y][x] = ' '
		}
	}
	s.cursorX = 0
	s.cursorY = 0
	s.outputBuffer.Reset()
}

// PrintChar prints a character at the current cursor position
func (s *ZXScreen) PrintChar(ch byte) {
	// Handle special characters
	switch ch {
	case 0x0D: // Carriage return
		s.NewLine()
		return
	case 0x08: // Backspace
		if s.cursorX > 0 {
			s.cursorX--
			s.buffer[s.cursorY][s.cursorX] = ' '
		}
		return
	case 0x09: // Tab
		s.cursorX = ((s.cursorX / 8) + 1) * 8
		if s.cursorX >= SCREEN_WIDTH {
			s.NewLine()
		}
		return
	case 0x0C: // Clear screen (form feed)
		s.Clear()
		return
	}
	
	// Print normal character
	if ch >= 0x20 && ch < 0x80 { // Printable ASCII
		s.buffer[s.cursorY][s.cursorX] = ch
		s.outputBuffer.WriteByte(ch)
		s.cursorX++
		
		// Wrap to next line if needed
		if s.cursorX >= SCREEN_WIDTH {
			s.NewLine()
		}
	}
}

// NewLine moves cursor to next line
func (s *ZXScreen) NewLine() {
	s.cursorX = 0
	s.cursorY++
	s.outputBuffer.WriteByte('\n')
	
	// Scroll if at bottom
	if s.cursorY >= SCREEN_HEIGHT {
		s.Scroll()
		s.cursorY = SCREEN_HEIGHT - 1
	}
}

// Scroll scrolls the screen up one line
func (s *ZXScreen) Scroll() {
	// Move all lines up
	for y := 0; y < SCREEN_HEIGHT-1; y++ {
		for x := 0; x < SCREEN_WIDTH; x++ {
			s.buffer[y][x] = s.buffer[y+1][x]
		}
	}
	
	// Clear bottom line
	for x := 0; x < SCREEN_WIDTH; x++ {
		s.buffer[SCREEN_HEIGHT-1][x] = ' '
	}
}

// SetCursor sets cursor position
func (s *ZXScreen) SetCursor(x, y int) {
	if x >= 0 && x < SCREEN_WIDTH {
		s.cursorX = x
	}
	if y >= 0 && y < SCREEN_HEIGHT {
		s.cursorY = y
	}
}

// GetScreen returns the screen as a string
func (s *ZXScreen) GetScreen() string {
	var sb strings.Builder
	
	// Draw top border
	sb.WriteString("╔")
	for i := 0; i < SCREEN_WIDTH; i++ {
		sb.WriteString("═")
	}
	sb.WriteString("╗\n")
	
	// Draw screen content
	for y := 0; y < SCREEN_HEIGHT; y++ {
		sb.WriteString("║")
		for x := 0; x < SCREEN_WIDTH; x++ {
			ch := s.buffer[y][x]
			if ch == 0 {
				ch = ' '
			}
			sb.WriteByte(ch)
		}
		sb.WriteString("║\n")
	}
	
	// Draw bottom border
	sb.WriteString("╚")
	for i := 0; i < SCREEN_WIDTH; i++ {
		sb.WriteString("═")
	}
	sb.WriteString("╝\n")
	
	return sb.String()
}

// GetCompactScreen returns screen with only non-empty lines
func (s *ZXScreen) GetCompactScreen() string {
	var lines []string
	
	for y := 0; y < SCREEN_HEIGHT; y++ {
		line := strings.TrimRight(string(s.buffer[y][:]), " ")
		if line != "" {
			lines = append(lines, line)
		}
	}
	
	if len(lines) == 0 {
		return "[Screen empty]"
	}
	
	return strings.Join(lines, "\n")
}

// GetOutput returns captured output
func (s *ZXScreen) GetOutput() string {
	return s.outputBuffer.String()
}

// UpdateSysvar updates a system variable
func (s *ZXScreen) UpdateSysvar(addr uint16, value byte) {
	if addr >= SYSVAR_BASE && addr < SYSVAR_BASE+256 {
		s.sysvarMem[addr-SYSVAR_BASE] = value
		
		// Handle specific system variables
		switch addr {
		case S_POSN: // Screen column position
			s.cursorX = int(value)
		case S_POSNL: // Screen line position
			s.cursorY = int(value)
		case ATTR_P: // Permanent attributes
			s.ink = value & 0x07
			s.paper = (value >> 3) & 0x07
			s.bright = (value & 0x40) != 0
			s.flash = (value & 0x80) != 0
		}
	}
}

// GetSysvar reads a system variable
func (s *ZXScreen) GetSysvar(addr uint16) byte {
	if addr >= SYSVAR_BASE && addr < SYSVAR_BASE+256 {
		// Return current values for position variables
		switch addr {
		case S_POSN:
			return byte(s.cursorX)
		case S_POSNL:
			return byte(s.cursorY)
		case ATTR_P:
			attr := s.ink | (s.paper << 3)
			if s.bright {
				attr |= 0x40
			}
			if s.flash {
				attr |= 0x80
			}
			return attr
		}
		return s.sysvarMem[addr-SYSVAR_BASE]
	}
	return 0
}

// HandleRST16 handles RST 16 (print character) call
func (s *ZXScreen) HandleRST16(a byte) {
	s.PrintChar(a)
	
	if s.debugMode {
		fmt.Printf("[RST 16: '%c' (0x%02X) at (%d,%d)]\n", 
			a, a, s.cursorX, s.cursorY)
	}
}

// HandlePort handles I/O port operations
func (s *ZXScreen) HandlePort(port byte, value byte, isOut bool) {
	if !isOut {
		return // Only handle output for now
	}
	
	switch port {
	case PORT_DEBUG_OUT:
		// Debug output port - print character directly
		s.PrintChar(value)
		
	case PORT_SCREEN:
		// Screen control port
		// Bit 0-2: Command
		// Bit 3-7: Parameter
		cmd := value & 0x07
		param := value >> 3
		
		switch cmd {
		case 0: // Clear screen
			s.Clear()
		case 1: // Set cursor X
			s.cursorX = int(param)
		case 2: // Set cursor Y
			s.cursorY = int(param)
		case 3: // Set ink
			s.ink = param & 0x07
		case 4: // Set paper
			s.paper = param & 0x07
		case 5: // Scroll
			s.Scroll()
		case 6: // New line
			s.NewLine()
		}
	}
}

// EnableDebug enables debug mode
func (s *ZXScreen) EnableDebug(enable bool) {
	s.debugMode = enable
}