package emulator

import (
	"fmt"
)

// RemogattoZ80WithScreen extends RemogattoZ80 with ZX Spectrum screen emulation
// This provides 100% Z80 instruction coverage while maintaining compatibility
type RemogattoZ80WithScreen struct {
	*RemogattoZ80
	Screen *ZXScreen
	Hooks  *Z80Hooks

	// Compatibility fields for existing MZE code
	PC       uint16 // Direct PC access for compatibility
	exitCode uint16
}

// NewRemogattoZ80WithScreen creates a Z80 emulator with 100% coverage and screen support
func NewRemogattoZ80WithScreen() *RemogattoZ80WithScreen {
	remogatto := NewRemogattoZ80()

	z80 := &RemogattoZ80WithScreen{
		RemogattoZ80: remogatto,
		Screen:       NewZXScreen(),
		Hooks:        &Z80Hooks{},
		PC:           0x8000, // Default start address
	}

	// Set up default hooks for screen output
	z80.Hooks.OnRST10 = func(a byte) {
		z80.Screen.HandleRST16(a)
	}

	// Set up hook for keyboard input (RST 18)
	z80.Hooks.OnRST18 = func() byte {
		return z80.Screen.GetKey()
	}

	z80.Hooks.OnOUT = func(port byte, value byte) {
		z80.Screen.HandlePort(port, value, true)
	}

	z80.Hooks.OnMemWrite = func(addr uint16, value byte) {
		// Update screen system variables
		if addr >= SYSVAR_BASE && addr < SYSVAR_BASE+256 {
			z80.Screen.UpdateSysvar(addr, value)
		}
	}

	z80.Hooks.OnMemRead = func(addr uint16) byte {
		// Handle system variable reads
		if addr >= SYSVAR_BASE && addr < SYSVAR_BASE+256 {
			// ZXScreen doesn't have ReadSysvar, just return 0xFF for now
			return 0xFF
		}
		return 0xFF
	}

	// Set up RST instruction hooks
	z80.setupRSTHooks()

	// Configure I/O handlers for the remogatto emulator
	z80.RemogattoZ80.SetIOHandlers(
		func(port uint16) byte {
			if z80.Hooks.OnIN != nil {
				return z80.Hooks.OnIN(byte(port))
			}
			return 0xFF
		},
		func(port uint16, value byte) {
			if z80.Hooks.OnOUT != nil {
				z80.Hooks.OnOUT(byte(port), value)
			}
		},
	)

	return z80
}

// setupRSTHooks registers host ROM substitutes only for actual unprefixed
// RST instruction fetches. Data/code writes are SMC notifications, not execution.
func (z *RemogattoZ80WithScreen) setupRSTHooks() {
	z.RemogattoZ80.fetchedRSTHook = z.handleRSTInstruction
}

// Handled host RST is an atomic 11T ROM abstraction: PC advances one byte,
// SP and flags remain unchanged. Unhandled RST runs on the actual CPU normally.
func (z *RemogattoZ80WithScreen) handleRSTInstruction(opcode byte) bool {
	if z.Hooks == nil {
		return false
	}
	switch opcode {
	case 0xc7:
		if z.Hooks.OnRST00 == nil {
			return false
		}
		z.Hooks.OnRST00()
	case 0xcf:
		if z.Hooks.OnRST08 == nil {
			return false
		}
		z.Hooks.OnRST08()
	case 0xd7:
		if z.Hooks.OnRST10 == nil {
			return false
		}
		z.Hooks.OnRST10(z.cpu.A)
	case 0xdf:
		if z.Hooks.OnRST18 == nil {
			return false
		}
		z.SetRegisterA(z.Hooks.OnRST18())
	case 0xe7:
		if z.Hooks.OnRST20 == nil {
			return false
		}
		z.SetRegisterA(z.Hooks.OnRST20())
	case 0xef:
		if z.Hooks.OnRST28 == nil {
			return false
		}
		z.Hooks.OnRST28()
	case 0xf7:
		if z.Hooks.OnRST30 == nil {
			return false
		}
		z.Hooks.OnRST30()
	case 0xff:
		if z.Hooks.OnRST38 == nil {
			return false
		}
		z.Hooks.OnRST38()
		z.exitCode = uint16(z.cpu.A)
	default:
		return false
	}
	return true
}

// Compatibility methods for existing MZE code

// LoadAt loads binary data at specified address (compatibility method)
func (z *RemogattoZ80WithScreen) LoadAt(address uint16, data []byte) {
	z.RemogattoZ80.LoadMemory(address, data)
	z.PC = address // Track PC for compatibility
}

// ExecuteWithHooks runs the emulator and returns output and cycle count
func (z *RemogattoZ80WithScreen) ExecuteWithHooks(pc uint16) ([]byte, int) {
	z.SetPC(pc)
	_ = z.RemogattoZ80.Run() // Run returns error, ignore for now
	return z.RemogattoZ80.GetOutput(), z.RemogattoZ80.GetCycles()
}

// SetPC sets program counter and updates compatibility field
func (z *RemogattoZ80WithScreen) SetPC(pc uint16) {
	z.RemogattoZ80.SetPC(pc)
	z.PC = pc
}

// GetPC gets program counter and updates compatibility field
func (z *RemogattoZ80WithScreen) GetPC() uint16 {
	pc := z.RemogattoZ80.GetPC()
	z.PC = pc
	return pc
}

// Execute runs the emulator (compatibility method)
func (z *RemogattoZ80WithScreen) Execute() error {
	return z.RemogattoZ80.Run()
}

// Step executes one instruction and updates PC
func (z *RemogattoZ80WithScreen) Step() int {
	cycles := z.RemogattoZ80.Step()
	z.PC = z.RemogattoZ80.GetPC() // Keep PC in sync
	return cycles
}

// SetBDOSHandler sets the CP/M BDOS handler (delegates to RemogattoZ80)
func (z *RemogattoZ80WithScreen) SetBDOSHandler(handler func(function byte, de uint16) (a byte, hl uint16, handled bool)) {
	z.RemogattoZ80.SetBDOSHandler(handler)
}

// ReadMemory reads a byte for the host without advancing CPU time.
func (z *RemogattoZ80WithScreen) ReadMemory(addr uint16) byte {
	return z.RemogattoZ80.memory.readByte(addr)
}

// WriteMemory writes a byte for the host without advancing CPU time.
func (z *RemogattoZ80WithScreen) WriteMemory(addr uint16, val byte) {
	z.RemogattoZ80.memory.writeByte(addr, val)
}

// SetROMEnd changes the ROM protection boundary. Set to 0 for all-RAM platforms.
func (z *RemogattoZ80WithScreen) SetROMEnd(addr uint16) {
	z.RemogattoZ80.memory.SetROMEnd(addr)
}

// GetExitCode returns the exit code for compatibility
func (z *RemogattoZ80WithScreen) GetExitCode() uint16 {
	if z.exitCode != 0 {
		return z.exitCode
	}
	return z.RemogattoZ80.GetExitCode()
}

// SetRegisterA sets the A register (helper for RST handlers)
func (z *RemogattoZ80WithScreen) SetRegisterA(value byte) {
	z.RemogattoZ80.cpu.A = value
}

// String returns state information for debugging
func (z *RemogattoZ80WithScreen) String() string {
	return fmt.Sprintf("RemogattoZ80WithScreen[PC=%04X, 100%% coverage]", z.PC)
}

// NewZ80WithScreenFull creates a 100% coverage Z80 emulator (compatibility alias)
func NewZ80WithScreenFull() *RemogattoZ80WithScreen {
	return NewRemogattoZ80WithScreen()
}
