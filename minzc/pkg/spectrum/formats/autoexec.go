package formats

import (
	"fmt"

	"github.com/minz/minzc/pkg/spectrum"
)

// ExecBASIC executes a tokenized BASIC command on a Spectrum machine.
// It waits for ROM initialization, pokes the command at E_LINE,
// sets up the execution context, and jumps to the ROM's statement handler.
//
// The tokens slice should contain one or more tokenized BASIC statements
// (WITHOUT a trailing $0D — this function appends it automatically).
//
// This is the uniform automation mechanism used by both TAP and TRD loaders.
//
// ROM entry points used (48K ROM):
//   - $1B17 (STMT-R-1): Statement processor entry
//   - $12A2 (MAIN-EXEC): Error recovery handler
//   - $12B7: Return address after CALL $1B17 in MAIN loop
//
// System variables modified:
//   - E_LINE ($5C59): Editing line address
//   - CH_ADD ($5C5D): Address of next character to interpret
//   - ERR_SP ($5C3D): Error recovery stack pointer
//   - K_CUR  ($5C5B): Keyboard cursor position
//   - WORKSP ($5C61): Temporary workspace
//   - STKBOT ($5C63): Bottom of calculator stack
//   - STKEND ($5C65): Top of calculator stack
func ExecBASIC(m *spectrum.Machine, tokens []byte) {
	// Get E_LINE — the editing area where we'll inject the command
	eLine := readWord(m, 0x5C59)
	if eLine < 0x5C00 || eLine > 0xFF00 {
		eLine = 0x5CCB // fallback
	}

	// Poke tokenized command at E_LINE
	for i, b := range tokens {
		m.Memory.Write(eLine+uint16(i), b, false)
	}
	// Append ENTER terminator
	m.Memory.Write(eLine+uint16(len(tokens)), 0x0D, false)

	// Update workspace pointers (after command + ENTER)
	worksp := eLine + uint16(len(tokens)) + 1
	writeWord(m, 0x5C5B, worksp) // K_CUR
	writeWord(m, 0x5C61, worksp) // WORKSP
	writeWord(m, 0x5C63, worksp) // STKBOT
	writeWord(m, 0x5C65, worksp) // STKEND

	// Set CH_ADD to E_LINE so the ROM reads our command
	writeWord(m, 0x5C5D, eLine)

	// Set up stack for BASIC execution
	sp := uint16(0xFF48)

	// Push MAIN-EXEC ($12A2) as error recovery handler
	sp -= 2
	m.Memory.Write(sp, 0xA2, false)
	m.Memory.Write(sp+1, 0x12, false)

	// Set ERR_SP to point to the error handler
	writeWord(m, 0x5C3D, sp)

	// Push return address $12B7 (after CALL $1B17 in MAIN loop)
	sp -= 2
	m.Memory.Write(sp, 0xB7, false)
	m.Memory.Write(sp+1, 0x12, false)

	m.CPU.SetSP(sp)

	// Jump to STMT-R-1 — processes the editing line as BASIC
	m.CPU.SetPC(0x1B17)
}

// WaitROMInit runs the machine for the given number of frames to let
// the ROM initialize system variables, screen, etc.
func WaitROMInit(m *spectrum.Machine, frames int) {
	for i := 0; i < frames; i++ {
		m.RunFrame()
	}
}

// BASIC tokens for common commands.
const (
	TokenRUN       = 0xF7
	TokenLOAD      = 0xEF
	TokenBORDER    = 0xE7
	TokenRANDOMIZE = 0xF9
	TokenUSR       = 0xC0
	TokenVAL       = 0xB0
	TokenREM       = 0xEA
	TokenQUOTE     = 0x22
	TokenCOLON     = 0x3A
)

// TokenizeLOAD returns the tokens for LOAD "".
func TokenizeLOAD() []byte {
	return []byte{TokenLOAD, TokenQUOTE, TokenQUOTE}
}

// TokenizeRUN returns the tokens for RUN.
func TokenizeRUN() []byte {
	return []byte{TokenRUN}
}

// TokenizeRANDOMIZEUSR returns the tokens for RANDOMIZE USR addr.
// addr is embedded as an ASCII string via VAL "addr".
func TokenizeRANDOMIZEUSR(addr uint16) []byte {
	s := fmt.Sprintf("%d", addr)
	tokens := []byte{TokenRANDOMIZE, TokenUSR, TokenVAL, TokenQUOTE}
	tokens = append(tokens, []byte(s)...)
	tokens = append(tokens, TokenQUOTE)
	return tokens
}

// emulateRET simulates a Z80 RET instruction (pop PC from stack).
// Used by ROM traps (TAP, TRD) to return to the caller.
func emulateRET(m *spectrum.Machine) {
	sp := m.CPU.SP()
	lo := m.Memory.Read(sp)
	hi := m.Memory.Read(sp + 1)
	m.CPU.SetPC(uint16(lo) | uint16(hi)<<8)
	m.CPU.SetSP(sp + 2)
}
