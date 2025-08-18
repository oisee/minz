// Package emulator - Migration utilities to switch between emulators
package emulator

import (
	"fmt"
)

// UseFullEmulator determines which Z80 implementation to use
var UseFullEmulator = true // Set to true to use remogatto/z80

// NewZ80Auto creates either basic or full emulator based on configuration
func NewZ80Auto() interface{} {
	if UseFullEmulator {
		return NewRemogattoZ80()
	}
	return New() // Basic emulator
}

// MigrateToRemogatto migrates existing code using basic emulator to remogatto
func MigrateToRemogatto(basic *Z80) *RemogattoZ80 {
	full := NewRemogattoZ80()
	
	// Copy memory
	for i := 0; i < MEMORY_SIZE; i++ {
		full.SetMemory(uint16(i), basic.memory[i])
	}
	
	// Set registers
	full.SetPC(basic.PC)
	full.SetSP(basic.SP)
	
	// Copy output
	full.output = append(full.output, basic.output...)
	
	// Copy exit configuration
	full.exitOnRST38 = basic.exitOnRST38
	full.exitOnRET0 = basic.exitOnRET0
	
	return full
}

// EmulatorInterface defines common interface for both emulators
type EmulatorInterface interface {
	Reset()
	LoadMemory(address uint16, data []byte) error
	Run() error
	Step() int
	GetPC() uint16
	GetSP() uint16
	SetPC(pc uint16)
	SetSP(sp uint16)
	GetOutput() []byte
	GetExitCode() uint16
	GetCycles() int
	IsHalted() bool
	GetRegisters() Registers
	DumpState() string
}

// Verify both emulators implement the interface
var _ EmulatorInterface = (*RemogattoZ80)(nil)

// MakeBasicCompatible wraps basic emulator to implement EmulatorInterface
type BasicEmulatorWrapper struct {
	*Z80
}

func (w *BasicEmulatorWrapper) LoadMemory(address uint16, data []byte) error {
	w.Z80.LoadAt(address, data)
	return nil
}

func (w *BasicEmulatorWrapper) GetPC() uint16 { return w.PC }
func (w *BasicEmulatorWrapper) GetSP() uint16 { return w.SP }
func (w *BasicEmulatorWrapper) SetPC(pc uint16) { w.PC = pc }
func (w *BasicEmulatorWrapper) SetSP(sp uint16) { w.SP = sp }
func (w *BasicEmulatorWrapper) GetOutput() []byte { return w.output }
func (w *BasicEmulatorWrapper) GetExitCode() uint16 { return w.exitCode }
func (w *BasicEmulatorWrapper) GetCycles() int { return int(w.cycles) }
func (w *BasicEmulatorWrapper) IsHalted() bool { return w.halted }

func (w *BasicEmulatorWrapper) GetRegisters() Registers {
	return Registers{
		A:  w.A,
		F:  w.F,
		BC: uint16(w.B)<<8 | uint16(w.C),
		DE: uint16(w.D)<<8 | uint16(w.E),
		HL: uint16(w.H)<<8 | uint16(w.L),
		IX: w.IX,
		IY: w.IY,
		SP: w.SP,
		PC: w.PC,
	}
}

func (w *BasicEmulatorWrapper) DumpState() string {
	r := w.GetRegisters()
	return fmt.Sprintf(
		"PC=%04X SP=%04X AF=%02X%02X BC=%04X DE=%04X HL=%04X IX=%04X IY=%04X\n"+
		"Cycles=%d Halted=%v",
		r.PC, r.SP, r.A, r.F, r.BC, r.DE, r.HL, r.IX, r.IY,
		w.GetCycles(), w.IsHalted(),
	)
}

// Run method for compatibility - basic emulator doesn't have Run, so simulate it
func (w *BasicEmulatorWrapper) Run() error {
	// Run until halted or exit condition met
	for !w.halted && w.PC != 0 {
		w.Step()
		
		// Safety limit
		if w.cycles > 10000000 {
			return fmt.Errorf("execution limit exceeded")
		}
	}
	return nil
}

// WrapBasic wraps the basic emulator to implement EmulatorInterface
func WrapBasic(z *Z80) EmulatorInterface {
	return &BasicEmulatorWrapper{Z80: z}
}