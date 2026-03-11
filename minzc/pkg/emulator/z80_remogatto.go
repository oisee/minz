// Package emulator provides Z80 CPU emulation using remogatto/z80
//
// This replaces the basic emulator with a full-featured implementation
// that supports 100% of Z80 instructions including undocumented opcodes.

package emulator

import (
	"fmt"
	"io"
	"os"

	"github.com/remogatto/z80"
)

// RemogattoZ80 wraps the remogatto/z80 emulator for full instruction coverage
type RemogattoZ80 struct {
	cpu      *z80.Z80
	memory   *Memory
	ports    *Ports

	// State tracking
	cycles   int
	halted   bool
	exitCode uint16

	// Exit conditions
	exitOnRST38  bool
	exitOnRET0   bool
	exitOnDIHalt bool

	// Output capture
	output []byte

	// BDOS handler for CP/M emulation
	bdosHandler func(function byte, de uint16) (a byte, hl uint16, handled bool)

	// RST handler for Agon MOS emulation (intercepts RST vectors 0x00-0x38)
	rstHandler func(vector byte, regs RSTRegisters) (RSTRegisters, bool)

	// Profiler (nil = disabled, zero-cost check in hot loop)
	Profiler *Profiler

	// WarnOnHalt prints a warning when HALT is executed with interrupts disabled
	WarnOnHalt  bool
	haltWarned  bool

	// T-state trap: one-shot breakpoint at exact T-state count
	tstateTrapTarget int64
	tstateTrapCB     func(cycles int64)

	// Execution limit (0 = default 10M)
	MaxCycles int
}

// RSTRegisters provides access to CPU registers for RST handlers
type RSTRegisters struct {
	A, B, C, D, E, H, L byte
	HL                   uint16
}

// Memory implements z80.MemoryAccessor interface
type Memory struct {
	data       [65536]byte
	romEnd     uint16
	smcTracker func(addr uint16, oldVal, newVal byte) // Optional SMC tracking
	profiler   *Profiler                               // Optional profiler hooks
}

func NewMemory() *Memory {
	return &Memory{
		romEnd: 0x4000, // Default ROM boundary
	}
}

func (m *Memory) ReadByte(address uint16) byte {
	if m.profiler != nil {
		m.profiler.OnMemRead(address)
	}
	return m.data[address]
}

func (m *Memory) WriteByte(address uint16, value byte) {
	if address < m.romEnd {
		return // ROM protection
	}

	if m.profiler != nil {
		m.profiler.OnMemWrite(address)
	}

	oldVal := m.data[address]
	m.data[address] = value

	// Track SMC if handler is set
	if m.smcTracker != nil && oldVal != value {
		m.smcTracker(address, oldVal, value)
	}
}

// Required by MemoryAccessor interface
func (m *Memory) ReadByteInternal(address uint16) byte {
	return m.data[address]
}

func (m *Memory) WriteByteInternal(address uint16, value byte) {
	m.WriteByte(address, value)
}

func (m *Memory) ContendRead(address uint16, time int)                        {}
func (m *Memory) ContendReadNoMreq(address uint16, time int)                  {}
func (m *Memory) ContendReadNoMreq_loop(address uint16, time int, count uint) {}
func (m *Memory) ContendWriteNoMreq(address uint16, time int)                 {}
func (m *Memory) ContendWriteNoMreq_loop(address uint16, time int, count uint) {}

// Additional methods required by MemoryAccessor
func (m *Memory) Read(address uint16) byte {
	return m.ReadByte(address)
}

func (m *Memory) Write(address uint16, value byte, protectROM bool) {
	if protectROM && address < m.romEnd {
		return
	}
	m.WriteByte(address, value)
}

func (m *Memory) Data() []byte {
	return m.data[:]
}

// Ports implements z80.PortAccessor interface
type Ports struct {
	ioRead  func(port uint16) byte
	ioWrite func(port uint16, value byte)
	output  *[]byte

	profiler *Profiler // Optional profiler hooks

	// Console port mapping
	consolePort byte
	consoleIn   <-chan byte
	consoleOut  io.Writer

	// Stderr port mapping (write-only, e.g. $25)
	stderrPort byte
	stderrOut  io.Writer
}

func NewPorts(output *[]byte) *Ports {
	return &Ports{
		output: output,
	}
}

// SetConsolePort configures bidirectional console I/O on a specific port.
// Protocol: IN returns 0x00 (no data) or 0x80|byte (data ready). OUT writes raw byte.
func (p *Ports) SetConsolePort(port byte, reader io.Reader, writer io.Writer) {
	p.consolePort = port
	p.consoleOut = writer
	if reader != nil {
		ch := make(chan byte, 256)
		go func() {
			buf := make([]byte, 1)
			for {
				n, err := reader.Read(buf)
				if n > 0 {
					ch <- buf[0]
				}
				if err != nil {
					return
				}
			}
		}()
		p.consoleIn = ch
	}
}

// SetStderrPort configures a write-only stderr port.
// OUT to this port sends bytes to the writer (typically os.Stderr).
// IN from this port always returns 0x00.
func (p *Ports) SetStderrPort(port byte, writer io.Writer) {
	p.stderrPort = port
	p.stderrOut = writer
}

func (p *Ports) ReadPort(address uint16) byte {
	if p.profiler != nil {
		p.profiler.OnIORead(address)
	}

	// Console port input
	if p.consolePort != 0 && byte(address&0xFF) == p.consolePort {
		if p.consoleIn != nil {
			select {
			case b := <-p.consoleIn:
				return 0x80 | b // bit 7 = data ready
			default:
				return 0x00 // no data
			}
		}
		return 0x00
	}

	// Stderr port is write-only — IN always returns 0x00
	if p.stderrPort != 0 && byte(address&0xFF) == p.stderrPort {
		return 0x00
	}

	if p.ioRead != nil {
		return p.ioRead(address)
	}
	return 0xFF
}

func (p *Ports) WritePort(address uint16, b byte) {
	if p.profiler != nil {
		p.profiler.OnIOWrite(address)
	}

	// Console port output
	if p.consolePort != 0 && byte(address&0xFF) == p.consolePort {
		if p.consoleOut != nil {
			p.consoleOut.Write([]byte{b})
		}
		return
	}

	// Stderr port output
	if p.stderrPort != 0 && byte(address&0xFF) == p.stderrPort {
		if p.stderrOut != nil {
			p.stderrOut.Write([]byte{b})
		}
		return
	}

	// MIR2 console output port ($23 = stdout, $25 = stderr — mze/mzx standard)
	if address&0xFF == 0x23 || address&0xFF == 0x25 {
		*p.output = append(*p.output, b)
	}

	if p.ioWrite != nil {
		p.ioWrite(address, b)
	}
}

func (p *Ports) ReadPortInternal(address uint16, contend bool) byte {
	return p.ReadPort(address)
}

func (p *Ports) WritePortInternal(address uint16, b byte, contend bool) {
	p.WritePort(address, b)
}

func (p *Ports) ContendPortPreio(address uint16)  {}
func (p *Ports) ContendPortPostio(address uint16) {}

// NewRemogattoZ80 creates a new Z80 with full instruction coverage
func NewRemogattoZ80() *RemogattoZ80 {
	memory := NewMemory()
	output := make([]byte, 0)
	ports := NewPorts(&output)
	cpu := z80.NewZ80(memory, ports)

	return &RemogattoZ80{
		cpu:          cpu,
		memory:       memory,
		ports:        ports,
		output:       output,
		exitOnRST38:  true,
		exitOnRET0:   true,
		exitOnDIHalt: true,
	}
}

// Reset resets the CPU to initial state
func (z *RemogattoZ80) Reset() {
	z.cpu.Reset()
	z.cycles = 0
	z.halted = false
	z.haltWarned = false
	z.output = z.output[:0]
}

// LoadMemory loads data into memory at the specified address
func (z *RemogattoZ80) LoadMemory(address uint16, data []byte) error {
	for i, b := range data {
		if int(address)+i >= 65536 {
			return fmt.Errorf("memory overflow at %04X", address+uint16(i))
		}
		z.memory.data[int(address)+i] = b
	}
	return nil
}

// safeDoOpcode wraps cpu.DoOpcode with panic recovery.
// Undefined ED-prefix opcodes cause panics in the remogatto/z80 library;
// we treat them as NOP with 8 T-states (matching MZX behavior).
func (z *RemogattoZ80) safeDoOpcode() {
	defer func() {
		if r := recover(); r != nil {
			z.cpu.Tstates += 8 // Treat undefined opcode as NOP
		}
	}()
	z.cpu.DoOpcode()
}

// SetProfiler attaches a profiler to the emulator, hooking into memory and I/O.
func (z *RemogattoZ80) SetProfiler(p *Profiler) {
	z.Profiler = p
	z.memory.profiler = p
	z.ports.profiler = p
}

// SetTStateTrap sets a one-shot breakpoint at the given T-state count.
// When the CPU's cycle counter reaches or exceeds the target, cb is called
// and the trap is cleared.
func (z *RemogattoZ80) SetTStateTrap(target int64, cb func(int64)) {
	z.tstateTrapTarget = target
	z.tstateTrapCB = cb
}

// Run executes instructions until a termination condition
func (z *RemogattoZ80) Run() error {
	prof := z.Profiler // local copy for zero-cost nil check
	maxCycles := z.MaxCycles
	if maxCycles == 0 {
		maxCycles = 10000000
	}

	for {
		// Check if halted
		if z.halted {
			return nil
		}

		// Get current PC for exit detection
		pc := z.cpu.PC()

		// CP/M BDOS intercept at 0x0005
		if pc == 0x0005 && z.bdosHandler != nil {
			function := z.cpu.C
			de := z.cpu.DE()
			a, hl, handled := z.bdosHandler(function, de)
			if handled {
				z.cpu.A = a
				z.cpu.H = byte(hl >> 8)
				z.cpu.L = byte(hl & 0xFF)
				// Simulate RET - pop return address from stack
				sp := z.cpu.SP()
				retLo := z.memory.ReadByteInternal(sp)
				retHi := z.memory.ReadByteInternal(sp + 1)
				z.cpu.SetSP(sp + 2)
				z.cpu.SetPC(uint16(retHi)<<8 | uint16(retLo))
				continue
			}
		}

		// Agon MOS RST intercept (RST 0x00, 0x08, 0x10, 0x18, etc.)
		if z.rstHandler != nil && (pc == 0x00 || pc == 0x08 || pc == 0x10 || pc == 0x18 || pc == 0x20 || pc == 0x28 || pc == 0x30) {
			regs := RSTRegisters{
				A: z.cpu.A, B: z.cpu.B, C: z.cpu.C,
				D: z.cpu.D, E: z.cpu.E, H: z.cpu.H, L: z.cpu.L,
				HL: z.cpu.HL(),
			}
			newRegs, handled := z.rstHandler(byte(pc), regs)
			if handled {
				z.cpu.A = newRegs.A
				z.cpu.H = newRegs.H
				z.cpu.L = newRegs.L
				// Simulate RET
				sp := z.cpu.SP()
				retLo := z.memory.ReadByteInternal(sp)
				retHi := z.memory.ReadByteInternal(sp + 1)
				z.cpu.SetSP(sp + 2)
				z.cpu.SetPC(uint16(retHi)<<8 | uint16(retLo))
				continue
			}
		}

		// Profiler: record before instruction
		if prof != nil {
			prof.BeforeOpcode(pc, int64(z.cycles))
		}

		// Execute one instruction (with panic recovery for undefined opcodes)
		z.safeDoOpcode()
		z.cycles += int(z.cpu.Tstates)

		// Profiler: track SP changes (stack push/pop detection)
		if prof != nil {
			prof.TrackSP(z.cpu.SP())
		}

		// T-state trap check
		if z.tstateTrapTarget > 0 && int64(z.cycles) >= z.tstateTrapTarget {
			cb := z.tstateTrapCB
			z.tstateTrapTarget = 0
			z.tstateTrapCB = nil
			if cb != nil {
				cb(int64(z.cycles))
			}
		}

		// Check exit conditions
		newPC := z.cpu.PC()

		// RST 38h exit convention
		if z.exitOnRST38 && pc != newPC && z.memory.data[pc] == 0xFF {
			z.exitCode = uint16(z.cpu.A)
			return nil
		}

		// RET to 0x0000 exit (ZX Spectrum)
		if z.exitOnRET0 && newPC == 0x0000 && pc != 0x0000 {
			z.exitCode = uint16(z.cpu.HL())
			return nil
		}

		// DI:HALT sequence — A register is the exit code
		if z.exitOnDIHalt && z.cpu.Halted && z.cpu.IFF1 == 0 {
			z.halted = true
			z.exitCode = uint16(z.cpu.A)
			// WarnOnHalt: warn about stuck CPU
			if z.WarnOnHalt && !z.haltWarned {
				fmt.Fprintf(os.Stderr, "WARNING: HALT with interrupts disabled at PC=$%04X (CPU stuck)\n", pc)
				z.haltWarned = true
			}
			return nil
		}

		// Safety: limit execution
		if z.cycles > maxCycles {
			return fmt.Errorf("execution limit exceeded")
		}
	}
}

// Step executes a single instruction
func (z *RemogattoZ80) Step() int {
	oldCycles := z.cpu.Tstates

	// Profiler: record before instruction
	if z.Profiler != nil {
		z.Profiler.BeforeOpcode(z.cpu.PC(), int64(z.cycles))
	}

	z.safeDoOpcode()
	cyclesUsed := int(z.cpu.Tstates - oldCycles)
	z.cycles += cyclesUsed

	// Check halt
	if z.cpu.Halted {
		z.halted = true
	}

	return cyclesUsed
}

// GetRegisters returns current register values
func (z *RemogattoZ80) GetRegisters() Registers {
	return Registers{
		A:  z.cpu.A,
		F:  z.cpu.F,
		BC: z.cpu.BC(),
		DE: z.cpu.DE(),
		HL: z.cpu.HL(),
		IX: z.cpu.IX(),
		IY: z.cpu.IY(),
		SP: z.cpu.SP(),
		PC: z.cpu.PC(),
	}
}

// SetPC sets the program counter
func (z *RemogattoZ80) SetPC(pc uint16) {
	z.cpu.SetPC(pc)
}

// SetSP sets the stack pointer
func (z *RemogattoZ80) SetSP(sp uint16) {
	z.cpu.SetSP(sp)
}

// GetPC returns the program counter
func (z *RemogattoZ80) GetPC() uint16 {
	return z.cpu.PC()
}

// GetSP returns the stack pointer
func (z *RemogattoZ80) GetSP() uint16 {
	return z.cpu.SP()
}

// GetOutput returns captured output
func (z *RemogattoZ80) GetOutput() []byte {
	return *z.ports.output
}

// GetExitCode returns the exit code
func (z *RemogattoZ80) GetExitCode() uint16 {
	return z.exitCode
}

// GetCycles returns total cycles executed
func (z *RemogattoZ80) GetCycles() int {
	return z.cycles
}

// IsHalted returns true if CPU is halted
func (z *RemogattoZ80) IsHalted() bool {
	return z.halted
}

// SetMemory sets a memory location
func (z *RemogattoZ80) SetMemory(address uint16, value byte) {
	z.memory.data[address] = value
}

// GetMemory reads a memory location
func (z *RemogattoZ80) GetMemory(address uint16) byte {
	return z.memory.data[address]
}

// Memory returns the full 64KB memory as a slice (for snapshots/profiling).
func (z *RemogattoZ80) Memory() []byte {
	return z.memory.data[:]
}

// GetI returns the interrupt vector register
func (z *RemogattoZ80) GetI() byte {
	return z.cpu.I
}

// GetR returns the memory refresh register
func (z *RemogattoZ80) GetR() byte {
	return byte((z.cpu.R7 & 0x80) | byte(z.cpu.R&0x7f))
}

// GetIM returns the interrupt mode
func (z *RemogattoZ80) GetIM() byte {
	return z.cpu.IM
}

// GetIFF1 returns interrupt flip-flop 1
func (z *RemogattoZ80) GetIFF1() bool {
	return z.cpu.IFF1 != 0
}

// GetIFF2 returns interrupt flip-flop 2
func (z *RemogattoZ80) GetIFF2() bool {
	return z.cpu.IFF2 != 0
}

// SetSMCTracker sets the SMC tracking callback
func (z *RemogattoZ80) SetSMCTracker(tracker func(addr uint16, oldVal, newVal byte)) {
	z.memory.smcTracker = tracker
}

// SetRSTHandler sets the RST vector handler for MOS emulation
func (z *RemogattoZ80) SetRSTHandler(handler func(vector byte, regs RSTRegisters) (RSTRegisters, bool)) {
	z.rstHandler = handler
}

// SetIOHandlers sets custom I/O handlers
func (z *RemogattoZ80) SetIOHandlers(read func(port uint16) byte, write func(port uint16, value byte)) {
	z.ports.ioRead = read
	z.ports.ioWrite = write
}

// SetBDOSHandler sets the CP/M BDOS handler for intercepting CALL 0x0005
func (z *RemogattoZ80) SetBDOSHandler(handler func(function byte, de uint16) (a byte, hl uint16, handled bool)) {
	z.bdosHandler = handler
}

// SetConsolePort configures bidirectional console I/O on a specific port.
func (z *RemogattoZ80) SetConsolePort(port byte, reader io.Reader, writer io.Writer) {
	z.ports.SetConsolePort(port, reader, writer)
}

// SetStderrPort configures a write-only port for stderr output.
func (z *RemogattoZ80) SetStderrPort(port byte, writer io.Writer) {
	z.ports.SetStderrPort(port, writer)
}

// DumpState returns a string representation of CPU state
func (z *RemogattoZ80) DumpState() string {
	r := z.GetRegisters()
	return fmt.Sprintf(
		"PC=%04X SP=%04X AF=%02X%02X BC=%04X DE=%04X HL=%04X IX=%04X IY=%04X\n"+
			"Cycles=%d Halted=%v ExitCode=%04X",
		r.PC, r.SP, r.A, r.F, r.BC, r.DE, r.HL, r.IX, r.IY,
		z.cycles, z.halted, z.exitCode,
	)
}

// DiagString returns a detailed diagnostic dump of CPU state.
func (z *RemogattoZ80) DiagString() string {
	r := z.GetRegisters()
	iff1 := byte(0)
	if z.cpu.IFF1 != 0 {
		iff1 = 1
	}
	iff2 := byte(0)
	if z.cpu.IFF2 != 0 {
		iff2 = 1
	}

	exitReason := "unknown"
	if z.halted {
		exitReason = "DI:HALT"
	} else if r.PC == 0x0000 {
		exitReason = "RET0"
	} else if z.exitCode != 0 || z.memory.data[r.PC] == 0xFF {
		exitReason = "RST38"
	}

	return fmt.Sprintf(
		"=== MZE Diagnostics ===\n"+
			"CPU:    PC=$%04X SP=$%04X AF=$%02X%02X BC=$%04X DE=$%04X HL=$%04X\n"+
			"        IX=$%04X IY=$%04X I=$%02X R=$%02X IM=%d IFF1=%d IFF2=%d\n"+
			"Cycles: %d T-states\n"+
			"Exit:   code=$%04X (via %s)",
		r.PC, r.SP, r.A, r.F, r.BC, r.DE, r.HL,
		r.IX, r.IY, z.cpu.I, z.cpu.R, z.cpu.IM, iff1, iff2,
		z.cycles,
		z.exitCode, exitReason,
	)
}
