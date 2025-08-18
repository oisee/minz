package emulator

import "fmt"

// REPLCompatibleZ80 wraps RemogattoZ80WithScreen to provide full compatibility with MZR REPL
type REPLCompatibleZ80 struct {
	*RemogattoZ80WithScreen
	
	// Direct register access for REPL compatibility
	A, F, B, C, D, E, H, L   byte
	A_, F_, B_, C_, D_, E_, H_, L_ byte  // Shadow registers
	IX, IY, SP uint16
	I, R byte
	
	// Internal state tracking
	lastRegisters Registers
}

// NewREPLCompatibleZ80 creates a Z80 emulator with REPL compatibility
func NewREPLCompatibleZ80() *REPLCompatibleZ80 {
	z80 := &REPLCompatibleZ80{
		RemogattoZ80WithScreen: NewRemogattoZ80WithScreen(),
	}
	z80.syncRegistersFromCPU()
	return z80
}

// syncRegistersFromCPU updates the public register fields from the CPU state
func (z *REPLCompatibleZ80) syncRegistersFromCPU() {
	cpu := z.RemogattoZ80.cpu
	
	// Main registers - access directly from CPU
	z.A = cpu.A
	z.F = cpu.F
	z.B = cpu.B
	z.C = cpu.C
	z.D = cpu.D
	z.E = cpu.E
	z.H = cpu.H
	z.L = cpu.L
	
	// Shadow registers
	z.A_ = cpu.A_
	z.F_ = cpu.F_
	z.B_ = cpu.B_
	z.C_ = cpu.C_
	z.D_ = cpu.D_
	z.E_ = cpu.E_
	z.H_ = cpu.H_
	z.L_ = cpu.L_
	
	// 16-bit registers
	z.IX = cpu.IX()
	z.IY = cpu.IY()
	z.SP = cpu.SP()
	z.PC = cpu.PC()
	
	// Special registers
	z.I = cpu.I
	z.R = byte(cpu.R & 0xFF)  // R is 16-bit in remogatto but we only need low byte
	
	// Store current state for comparison
	z.lastRegisters = z.RemogattoZ80.GetRegisters()
}

// syncRegistersToCPU updates the CPU state from the public register fields
func (z *REPLCompatibleZ80) syncRegistersToCPU() {
	cpu := z.RemogattoZ80.cpu
	
	// Update registers directly
	cpu.A = z.A
	cpu.F = z.F
	cpu.B = z.B
	cpu.C = z.C
	cpu.D = z.D
	cpu.E = z.E
	cpu.H = z.H
	cpu.L = z.L
	
	// Shadow registers
	cpu.A_ = z.A_
	cpu.F_ = z.F_
	cpu.B_ = z.B_
	cpu.C_ = z.C_
	cpu.D_ = z.D_
	cpu.E_ = z.E_
	cpu.H_ = z.H_
	cpu.L_ = z.L_
	
	// 16-bit registers
	cpu.SetIX(z.IX)
	cpu.SetIY(z.IY)
	cpu.SetSP(z.SP)
	cpu.SetPC(z.PC)
	
	// Special registers
	cpu.I = z.I
	cpu.R = uint16(z.R)  // R is 16-bit in remogatto
}

// ExecuteWithHooks runs code and returns output and cycle count
func (z *REPLCompatibleZ80) ExecuteWithHooks(pc uint16) ([]byte, int) {
	// Sync any manual register changes to CPU
	z.syncRegistersToCPU()
	
	// Execute using underlying emulator
	output, cycles := z.RemogattoZ80WithScreen.ExecuteWithHooks(pc)
	
	// Sync CPU state back to public fields
	z.syncRegistersFromCPU()
	
	return output, cycles
}

// Reset resets the CPU state
func (z *REPLCompatibleZ80) Reset() {
	z.RemogattoZ80WithScreen.Reset()
	z.syncRegistersFromCPU()
}

// LoadAt loads code at the specified address
func (z *REPLCompatibleZ80) LoadAt(address uint16, code []byte) {
	z.RemogattoZ80WithScreen.LoadAt(address, code)
}

// PrintCompactScreen prints the screen in compact format
func (z *REPLCompatibleZ80) PrintCompactScreen() {
	// Print the compact screen representation
	fmt.Print(z.Screen.GetCompactScreen())
}

// GetIFF1 returns interrupt flip-flop 1
func (z *REPLCompatibleZ80) GetIFF1() bool {
	return z.RemogattoZ80.cpu.IFF1 != 0
}

// GetIFF2 returns interrupt flip-flop 2
func (z *REPLCompatibleZ80) GetIFF2() bool {
	return z.RemogattoZ80.cpu.IFF2 != 0
}

// GetIM returns interrupt mode
func (z *REPLCompatibleZ80) GetIM() byte {
	return z.RemogattoZ80.cpu.IM
}