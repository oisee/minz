package formats

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/minz/minzc/pkg/spectrum"
)

// Console modes for RST $10 / RST $18 trapping.
const (
	// RST10Addr is the ROM's print-a-character routine.
	// Register A contains the character to output.
	RST10Addr uint16 = 0x0010

	// RST18Addr is the ROM's get-a-character routine.
	RST18Addr uint16 = 0x0018
)

// ConsoleCapture intercepts RST $10 (character output) and writes
// characters to a writer (typically os.Stdout). The ROM still renders
// the character normally — we just mirror it to the console.
//
// This turns the Spectrum into a text console: BASIC PRINT output
// appears on your terminal alongside the emulator display.
type ConsoleCapture struct {
	machine *spectrum.Machine
	writer  io.Writer
	enabled bool

	// Filter control characters (0-31 except CR/LF)
	filterControl bool

	// Track column for clean line wrapping
	col int
}

// InstallConsoleCapture sets up RST $10 trapping on the machine.
// Characters printed by BASIC (PRINT, LIST, error messages) will be
// mirrored to the given writer.
func InstallConsoleCapture(m *spectrum.Machine, w io.Writer) *ConsoleCapture {
	cc := &ConsoleCapture{
		machine:       m,
		writer:        w,
		enabled:       true,
		filterControl: true,
	}

	m.SetPCTrap(RST10Addr, func() {
		if !cc.enabled {
			// Execute the real RST $10 — JP to ($5C36) which is PRINT-A
			cc.execRST10(m)
			return
		}

		a := byte(m.CPU.AF() >> 8)

		// Mirror to console
		cc.outputChar(a)

		// Still let the ROM handle it (execute the real RST $10)
		cc.execRST10(m)
	})

	return cc
}

func (cc *ConsoleCapture) outputChar(ch byte) {
	switch {
	case ch == 0x0D: // CR → newline
		fmt.Fprintln(cc.writer)
		cc.col = 0
	case ch == 0x06: // PRINT comma → tab
		fmt.Fprint(cc.writer, "\t")
		cc.col += 8
	case ch >= 0x20 && ch < 0x80: // Printable ASCII
		cc.writer.Write([]byte{ch})
		cc.col++
	case ch >= 0x80 && ch < 0x90: // UDG block graphics
		// Map to Unicode block elements for visual approximation
		cc.writer.Write([]byte("@"))
		cc.col++
	case ch >= 0xA5: // BASIC token — expand to keyword
		if name := tokenName(ch); name != "" {
			fmt.Fprint(cc.writer, name)
			cc.col += len(name)
		}
	default:
		if !cc.filterControl {
			cc.writer.Write([]byte{ch})
		}
		// Control chars (AT, TAB, INK, PAPER, etc.) — skip in filter mode
	}
}

// tokenName returns the keyword name for a BASIC token.
func tokenName(token byte) string {
	for _, kw := range basicKeywords {
		if kw.token == token {
			return kw.keyword
		}
	}
	return ""
}

// execRST10 lets the ROM handle the actual character output normally.
// We're mirroring, not intercepting — the screen still renders.
//
// The PC trap fires when PC == $0010. The caller's RST $10 already
// executed (pushed return address, set PC=$0010). We just need to jump
// past the trap to the ROM's JP target at $0010.
func (cc *ConsoleCapture) execRST10(m *spectrum.Machine) {
	// ROM at $0010: JP $xxxx — read the 3-byte JP target
	opcode := m.Memory.Read(0x0010)
	if opcode == 0xC3 { // JP nn
		target := uint16(m.Memory.Read(0x0011)) | uint16(m.Memory.Read(0x0012))<<8
		m.CPU.SetPC(target)
	} else {
		// Non-standard ROM — skip the instruction
		m.CPU.SetPC(0x0011)
	}
}

// SetEnabled enables or disables console capture.
func (cc *ConsoleCapture) SetEnabled(enabled bool) {
	cc.enabled = enabled
}

// InstallConsoleInput sets up input injection: characters from the reader
// are fed to the Spectrum when it requests keyboard input via RST $18.
// This allows piping stdin into BASIC INPUT statements.
func InstallConsoleInput(m *spectrum.Machine, r io.Reader) {
	// Buffer for input characters
	buf := make([]byte, 1)

	m.SetPCTrap(RST18Addr, func() {
		// RST $18 at $0018 jumps to the "get character" routine.
		// Read the JP target from ROM.
		target := uint16(m.Memory.Read(0x0019)) | uint16(m.Memory.Read(0x001A))<<8

		// For now, just let the ROM handle it normally.
		// TODO: Inject characters from reader when INPUT is waiting
		_ = buf
		_ = r
		m.CPU.SetPC(target)
	})
}

// InstallStdioConsole is a convenience function that sets up both
// console capture (stdout) and console input (stdin).
func InstallStdioConsole(m *spectrum.Machine) *ConsoleCapture {
	cc := InstallConsoleCapture(m, os.Stdout)
	// Input injection is more complex — leave for future
	// InstallConsoleInput(m, os.Stdin)
	return cc
}

// KeystrokeQueue injects keystrokes into the Spectrum keyboard matrix.
// This works with any ROM — no tokenization needed.
type KeystrokeQueue struct {
	machine *spectrum.Machine
	keys    []keystroke
	pos     int
	frameDelay int // frames to hold each key
	gapFrames  int // frames between keys
	counter    int // frame counter
	holding    bool
}

type keystroke struct {
	specKeys    []spectrum.SpecKey // keys to press simultaneously
	pauseFrames int               // >0: idle pause (no key), skip this many frames
}

// NewKeystrokeQueue creates a keystroke injector.
// holdFrames: how many frames to hold each key (2-3 is good)
// gapFrames: how many frames of no keys between presses (1-2 is good)
func NewKeystrokeQueue(m *spectrum.Machine, holdFrames, gapFrames int) *KeystrokeQueue {
	return &KeystrokeQueue{
		machine:    m,
		frameDelay: holdFrames,
		gapFrames:  gapFrames,
	}
}

// TypeText converts a text string to Spectrum keystrokes and queues them.
// Handles letters, digits, space, enter, and common punctuation.
//
// Special sequences:
//
//	_       — pause 10 frames (0.2 sec)
//	.       — pause 1 frame
//	{N}     — pause N frames
//	{wait}  — pause 50 frames (1 sec)
//	\n      — ENTER key
//
// ENTER is NOT auto-appended.
// For literal _ or . Spectrum keys, use {_} or {.}
func (kq *KeystrokeQueue) TypeText(text string) {
	i := 0
	for i < len(text) {
		ch := text[i]

		// {N}, {wait}, {_}, {.} — brace escape syntax
		if ch == '{' {
			end := strings.Index(text[i:], "}")
			if end > 0 {
				inner := text[i+1 : i+end]
				switch inner {
				case "wait":
					kq.keys = append(kq.keys, keystroke{pauseFrames: 50})
				case "_":
					// Literal underscore: SS+0
					kq.keys = append(kq.keys, keystroke{specKeys: charToSpecKeys('_')})
				case ".":
					// Literal dot: SS+M
					kq.keys = append(kq.keys, keystroke{specKeys: charToSpecKeys('.')})
				default:
					if n, err := strconv.Atoi(inner); err == nil {
						kq.keys = append(kq.keys, keystroke{pauseFrames: n})
					}
				}
				i += end + 1
				continue
			}
		}

		// _ = 10-frame pause, . = 1-frame pause
		if ch == '_' {
			kq.keys = append(kq.keys, keystroke{pauseFrames: 10})
			i++
			continue
		}
		if ch == '.' {
			kq.keys = append(kq.keys, keystroke{pauseFrames: 1})
			i++
			continue
		}

		keys := charToSpecKeys(ch)
		if keys != nil {
			kq.keys = append(kq.keys, keystroke{specKeys: keys})
		}
		i++
	}
}

// Update should be called once per frame. It injects the current keystroke
// into the keyboard matrix.
func (kq *KeystrokeQueue) Update() {
	if kq.pos >= len(kq.keys) {
		return // done
	}

	kq.counter++

	// Handle pause entries (no key press, just wait)
	ks := &kq.keys[kq.pos]
	if ks.pauseFrames > 0 {
		if kq.counter >= ks.pauseFrames {
			kq.counter = 0
			kq.pos++
		}
		return
	}

	if kq.holding {
		// Currently pressing a key
		if kq.counter >= kq.frameDelay {
			// Release key, enter gap
			kq.holding = false
			kq.counter = 0
			kq.pos++
		} else {
			// Hold the key
			for _, sk := range ks.specKeys {
				kq.machine.Keyboard.KeyPress(sk.Row, sk.Bit)
			}
		}
	} else {
		// In gap between keys
		if kq.counter >= kq.gapFrames {
			// Start pressing next key
			if kq.pos < len(kq.keys) {
				kq.holding = true
				kq.counter = 0
				for _, sk := range kq.keys[kq.pos].specKeys {
					kq.machine.Keyboard.KeyPress(sk.Row, sk.Bit)
				}
			}
		}
	}
}

// Done returns true when all keystrokes have been injected.
func (kq *KeystrokeQueue) Done() bool {
	return kq.pos >= len(kq.keys)
}

// charToSpecKeys maps an ASCII character to Spectrum key presses.
// Some characters require Symbol Shift (SS) combinations.
func charToSpecKeys(ch byte) []spectrum.SpecKey {
	switch {
	// Letters (simple key press)
	case ch >= 'A' && ch <= 'Z':
		return letterKey(ch)
	case ch >= 'a' && ch <= 'z':
		return letterKey(ch - 32) // uppercase

	// Digits
	case ch >= '0' && ch <= '9':
		return digitKey(ch)

	// Special keys
	case ch == ' ':
		return []spectrum.SpecKey{spectrum.KeySpace}
	case ch == '\n' || ch == '\r':
		return []spectrum.SpecKey{spectrum.KeyEnter}

	// Symbol Shift + key combinations for punctuation
	case ch == '!':
		return []spectrum.SpecKey{spectrum.KeySym, spectrum.Key1}
	case ch == '@':
		return []spectrum.SpecKey{spectrum.KeySym, spectrum.Key2}
	case ch == '#':
		return []spectrum.SpecKey{spectrum.KeySym, spectrum.Key3}
	case ch == '$':
		return []spectrum.SpecKey{spectrum.KeySym, spectrum.Key4}
	case ch == '%':
		return []spectrum.SpecKey{spectrum.KeySym, spectrum.Key5}
	case ch == '&':
		return []spectrum.SpecKey{spectrum.KeySym, spectrum.Key6}
	case ch == '\'':
		return []spectrum.SpecKey{spectrum.KeySym, spectrum.Key7}
	case ch == '(':
		return []spectrum.SpecKey{spectrum.KeySym, spectrum.Key8}
	case ch == ')':
		return []spectrum.SpecKey{spectrum.KeySym, spectrum.Key9}
	case ch == '_':
		return []spectrum.SpecKey{spectrum.KeySym, spectrum.Key0}
	case ch == '<':
		return []spectrum.SpecKey{spectrum.KeySym, spectrum.KeyR}
	case ch == '>':
		return []spectrum.SpecKey{spectrum.KeySym, spectrum.KeyT}
	case ch == ';':
		return []spectrum.SpecKey{spectrum.KeySym, spectrum.KeyO}
	case ch == '"':
		return []spectrum.SpecKey{spectrum.KeySym, spectrum.KeyP}
	case ch == '-':
		return []spectrum.SpecKey{spectrum.KeySym, spectrum.KeyJ}
	case ch == '+':
		return []spectrum.SpecKey{spectrum.KeySym, spectrum.KeyK}
	case ch == '=':
		return []spectrum.SpecKey{spectrum.KeySym, spectrum.KeyL}
	case ch == ':':
		return []spectrum.SpecKey{spectrum.KeySym, spectrum.KeyZ}
	case ch == ',':
		return []spectrum.SpecKey{spectrum.KeySym, spectrum.KeyN}
	case ch == '.':
		return []spectrum.SpecKey{spectrum.KeySym, spectrum.KeyM}
	case ch == '/':
		return []spectrum.SpecKey{spectrum.KeySym, spectrum.KeyV}
	case ch == '*':
		return []spectrum.SpecKey{spectrum.KeySym, spectrum.KeyB}
	case ch == '?':
		return []spectrum.SpecKey{spectrum.KeySym, spectrum.KeyC}
	}
	return nil
}

func letterKey(ch byte) []spectrum.SpecKey {
	switch ch {
	case 'A': return []spectrum.SpecKey{spectrum.KeyA}
	case 'B': return []spectrum.SpecKey{spectrum.KeyB}
	case 'C': return []spectrum.SpecKey{spectrum.KeyC}
	case 'D': return []spectrum.SpecKey{spectrum.KeyD}
	case 'E': return []spectrum.SpecKey{spectrum.KeyE}
	case 'F': return []spectrum.SpecKey{spectrum.KeyF}
	case 'G': return []spectrum.SpecKey{spectrum.KeyG}
	case 'H': return []spectrum.SpecKey{spectrum.KeyH}
	case 'I': return []spectrum.SpecKey{spectrum.KeyI}
	case 'J': return []spectrum.SpecKey{spectrum.KeyJ}
	case 'K': return []spectrum.SpecKey{spectrum.KeyK}
	case 'L': return []spectrum.SpecKey{spectrum.KeyL}
	case 'M': return []spectrum.SpecKey{spectrum.KeyM}
	case 'N': return []spectrum.SpecKey{spectrum.KeyN}
	case 'O': return []spectrum.SpecKey{spectrum.KeyO}
	case 'P': return []spectrum.SpecKey{spectrum.KeyP}
	case 'Q': return []spectrum.SpecKey{spectrum.KeyQ}
	case 'R': return []spectrum.SpecKey{spectrum.KeyR}
	case 'S': return []spectrum.SpecKey{spectrum.KeyS}
	case 'T': return []spectrum.SpecKey{spectrum.KeyT}
	case 'U': return []spectrum.SpecKey{spectrum.KeyU}
	case 'V': return []spectrum.SpecKey{spectrum.KeyV}
	case 'W': return []spectrum.SpecKey{spectrum.KeyW}
	case 'X': return []spectrum.SpecKey{spectrum.KeyX}
	case 'Y': return []spectrum.SpecKey{spectrum.KeyY}
	case 'Z': return []spectrum.SpecKey{spectrum.KeyZ}
	}
	return nil
}

func digitKey(ch byte) []spectrum.SpecKey {
	switch ch {
	case '0': return []spectrum.SpecKey{spectrum.Key0}
	case '1': return []spectrum.SpecKey{spectrum.Key1}
	case '2': return []spectrum.SpecKey{spectrum.Key2}
	case '3': return []spectrum.SpecKey{spectrum.Key3}
	case '4': return []spectrum.SpecKey{spectrum.Key4}
	case '5': return []spectrum.SpecKey{spectrum.Key5}
	case '6': return []spectrum.SpecKey{spectrum.Key6}
	case '7': return []spectrum.SpecKey{spectrum.Key7}
	case '8': return []spectrum.SpecKey{spectrum.Key8}
	case '9': return []spectrum.SpecKey{spectrum.Key9}
	}
	return nil
}
