// Package emulator provides Z80 CPU emulation using remogatto/z80
//
// This replaces the basic emulator with a full-featured implementation
// that supports 100% of Z80 instructions including undocumented opcodes.

package emulator

import (
	"fmt"
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
	exitOnRST38 bool
	exitOnRET0  bool
	exitOnDIHalt bool
	
	// Output capture
	output []byte
}

// Memory implements z80.MemoryAccessor interface
type Memory struct {
	data     [65536]byte
	romEnd   uint16
	smcTracker func(addr uint16, oldVal, newVal byte) // Optional SMC tracking
}

func NewMemory() *Memory {
	return &Memory{
		romEnd: 0x4000, // Default ROM boundary
	}
}

func (m *Memory) ReadByte(address uint16) byte {
	return m.data[address]
}

func (m *Memory) WriteByte(address uint16, value byte) {
	if address < m.romEnd {
		return // ROM protection
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
	return m.ReadByte(address)
}

func (m *Memory) WriteByteInternal(address uint16, value byte) {
	m.WriteByte(address, value)
}

func (m *Memory) ContendRead(address uint16, time int) {}
func (m *Memory) ContendReadNoMreq(address uint16, time int) {}
func (m *Memory) ContendReadNoMreq_loop(address uint16, time int, count uint) {}
func (m *Memory) ContendWriteNoMreq(address uint16, time int) {}
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
}

func NewPorts(output *[]byte) *Ports {
	return &Ports{
		output: output,
	}
}

func (p *Ports) ReadPort(address uint16) byte {
	if p.ioRead != nil {
		return p.ioRead(address)
	}
	return 0xFF
}

func (p *Ports) WritePort(address uint16, b byte) {
	// Console output port
	if address&0xFF == 0x01 {
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

func (p *Ports) ContendPortPreio(address uint16) {}
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

// Run executes instructions until a termination condition
func (z *RemogattoZ80) Run() error {
	for {
		// Check if halted
		if z.halted {
			return nil
		}
		
		// Get current PC for exit detection
		pc := z.cpu.PC()
		
		// Execute one instruction
		z.cpu.DoOpcode()
		z.cycles += int(z.cpu.Tstates)
		
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
		
		// DI:HALT sequence
		if z.exitOnDIHalt && z.cpu.Halted && z.cpu.IFF1 == 0 {
			z.halted = true
			return nil
		}
		
		// Safety: limit execution
		if z.cycles > 10000000 {
			return fmt.Errorf("execution limit exceeded")
		}
	}
}

// Step executes a single instruction
func (z *RemogattoZ80) Step() int {
	oldCycles := z.cpu.Tstates
	z.cpu.DoOpcode()
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
	return z.output
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

// SetSMCTracker sets the SMC tracking callback
func (z *RemogattoZ80) SetSMCTracker(tracker func(addr uint16, oldVal, newVal byte)) {
	z.memory.smcTracker = tracker
}

// SetIOHandlers sets custom I/O handlers
func (z *RemogattoZ80) SetIOHandlers(read func(port uint16) byte, write func(port uint16, value byte)) {
	z.ports.ioRead = read
	z.ports.ioWrite = write
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