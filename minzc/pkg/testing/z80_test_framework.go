package testing

import (
	"fmt"
	"github.com/remogatto/z80"
	"strings"
	"testing"
)

// TestContext represents a Z80 test environment
type TestContext struct {
	cpu    *z80.Z80
	memory *TestMemory
	ports  *TestPorts
	t      *testing.T
}

// TestMemory implements MemoryAccessor with test utilities
type TestMemory struct {
	data    [65536]byte
	writes  map[uint16][]byte
	reads   map[uint16]int
	romEnd  uint16
}

func NewTestMemory() *TestMemory {
	return &TestMemory{
		writes: make(map[uint16][]byte),
		reads:  make(map[uint16]int),
		romEnd: 0x4000, // Default ROM boundary
	}
}

func (m *TestMemory) ReadByte(address uint16) byte {
	m.reads[address]++
	return m.data[address]
}

func (m *TestMemory) WriteByte(address uint16, value byte) {
	if address < m.romEnd {
		return // ROM protection
	}
	m.data[address] = value
	m.writes[address] = append(m.writes[address], value)
}

// Implement other MemoryAccessor methods...
func (m *TestMemory) ReadByteInternal(address uint16) byte { return m.ReadByte(address) }
func (m *TestMemory) WriteByteInternal(address uint16, value byte) { m.WriteByte(address, value) }
func (m *TestMemory) ContendRead(address uint16, time int) {}
func (m *TestMemory) ContendReadNoMreq(address uint16, time int) {}
func (m *TestMemory) ContendReadNoMreq_loop(address uint16, time int, count uint) {}
func (m *TestMemory) ContendWriteNoMreq(address uint16, time int) {}
func (m *TestMemory) ContendWriteNoMreq_loop(address uint16, time int, count uint) {}
func (m *TestMemory) Read(address uint16) byte { return m.ReadByte(address) }
func (m *TestMemory) Write(address uint16, value byte, protectROM bool) { m.WriteByte(address, value) }
func (m *TestMemory) Data() []byte { return m.data[:] }

// TestPorts implements PortAccessor for testing
type TestPorts struct {
	in  map[uint16]byte
	out map[uint16][]byte
}

func NewTestPorts() *TestPorts {
	return &TestPorts{
		in:  make(map[uint16]byte),
		out: make(map[uint16][]byte),
	}
}

func (p *TestPorts) ReadPort(address uint16) byte {
	return p.in[address]
}

func (p *TestPorts) WritePort(address uint16, b byte) {
	p.out[address] = append(p.out[address], b)
}

func (p *TestPorts) ReadPortInternal(address uint16, contend bool) byte { return p.ReadPort(address) }
func (p *TestPorts) WritePortInternal(address uint16, b byte, contend bool) { p.WritePort(address, b) }
func (p *TestPorts) ContendPortPreio(address uint16) {}
func (p *TestPorts) ContendPortPostio(address uint16) {}

// DSL Functions

// NewTest creates a new test context
func NewTest(t *testing.T) *TestContext {
	memory := NewTestMemory()
	ports := NewTestPorts()
	cpu := z80.NewZ80(memory, ports)
	return &TestContext{
		cpu:    cpu,
		memory: memory,
		ports:  ports,
		t:      t,
	}
}

// Given sets up initial state
func (tc *TestContext) Given() *GivenContext {
	return &GivenContext{tc: tc}
}

// When executes code
func (tc *TestContext) When() *WhenContext {
	return &WhenContext{tc: tc}
}

// Then verifies results
func (tc *TestContext) Then() *ThenContext {
	return &ThenContext{tc: tc}
}

// GivenContext for setting up state
type GivenContext struct {
	tc *TestContext
}

func (g *GivenContext) Register(reg string, value uint16) *GivenContext {
	switch strings.ToUpper(reg) {
	case "A":
		g.tc.cpu.A = byte(value)
	case "B":
		g.tc.cpu.B = byte(value)
	case "C":
		g.tc.cpu.C = byte(value)
	case "HL":
		g.tc.cpu.SetHL(value)
	case "BC":
		g.tc.cpu.SetBC(value)
	case "DE":
		g.tc.cpu.SetDE(value)
	case "SP":
		g.tc.cpu.SP = value
	case "PC":
		g.tc.cpu.PC = value
	}
	return g
}

func (g *GivenContext) Memory(address uint16, values ...byte) *GivenContext {
	for i, v := range values {
		g.tc.memory.data[address+uint16(i)] = v
	}
	return g
}

func (g *GivenContext) Code(address uint16, opcodes ...byte) *GivenContext {
	g.Memory(address, opcodes...)
	g.tc.cpu.PC = address
	return g
}

func (g *GivenContext) Stack(values ...uint16) *GivenContext {
	sp := uint16(0xFFFF)
	for _, v := range values {
		g.tc.memory.data[sp] = byte(v >> 8)
		g.tc.memory.data[sp-1] = byte(v & 0xFF)
		sp -= 2
	}
	g.tc.cpu.SP = sp
	return g
}

func (g *GivenContext) Port(port uint16, value byte) *GivenContext {
	g.tc.ports.in[port] = value
	return g
}

// WhenContext for executing code
type WhenContext struct {
	tc     *TestContext
	cycles int
}

func (w *WhenContext) Execute(cycles int) *WhenContext {
	for i := 0; i < cycles && !w.tc.cpu.Halted; i++ {
		w.tc.cpu.DoOpcode()
	}
	w.cycles = w.tc.cpu.Tstates
	return w
}

func (w *WhenContext) Call(address uint16) *WhenContext {
	// CALL instruction: CD low high
	w.tc.cpu.PC = address
	// Execute until RET (C9) is hit
	for {
		pc := w.tc.cpu.PC
		opcode := w.tc.memory.ReadByte(pc)
		w.tc.cpu.DoOpcode()
		if opcode == 0xC9 { // RET
			break
		}
		if w.tc.cpu.Halted {
			w.tc.t.Fatal("CPU halted during call")
		}
	}
	w.cycles = w.tc.cpu.Tstates
	return w
}

func (w *WhenContext) ExecuteUntil(address uint16) *WhenContext {
	for w.tc.cpu.PC != address && !w.tc.cpu.Halted {
		w.tc.cpu.DoOpcode()
	}
	w.cycles = w.tc.cpu.Tstates
	return w
}

// ThenContext for assertions
type ThenContext struct {
	tc *TestContext
}

func (t *ThenContext) Register(reg string, expected uint16) *ThenContext {
	actual := t.getRegister(reg)
	if actual != expected {
		t.tc.t.Errorf("Register %s: expected %04X, got %04X", reg, expected, actual)
	}
	return t
}

func (t *ThenContext) Memory(address uint16, expected ...byte) *ThenContext {
	for i, exp := range expected {
		actual := t.tc.memory.data[address+uint16(i)]
		if actual != exp {
			t.tc.t.Errorf("Memory[%04X]: expected %02X, got %02X", address+uint16(i), exp, actual)
		}
	}
	return t
}

func (t *ThenContext) Flag(flag string, expected bool) *ThenContext {
	f := t.tc.cpu.F
	var actual bool
	switch strings.ToUpper(flag) {
	case "Z", "ZERO":
		actual = (f & 0x40) != 0
	case "C", "CARRY":
		actual = (f & 0x01) != 0
	case "S", "SIGN":
		actual = (f & 0x80) != 0
	case "P", "PARITY":
		actual = (f & 0x04) != 0
	}
	if actual != expected {
		t.tc.t.Errorf("Flag %s: expected %v, got %v", flag, expected, actual)
	}
	return t
}

func (t *ThenContext) Port(port uint16, expected ...byte) *ThenContext {
	actual := t.tc.ports.out[port]
	if len(actual) != len(expected) {
		t.tc.t.Errorf("Port %04X: expected %d writes, got %d", port, len(expected), len(actual))
		return t
	}
	for i, exp := range expected {
		if actual[i] != exp {
			t.tc.t.Errorf("Port %04X write %d: expected %02X, got %02X", port, i, exp, actual[i])
		}
	}
	return t
}

func (t *ThenContext) Cycles(min, max int) *ThenContext {
	// Access cycles from the last When operation
	actual := t.tc.cpu.Tstates
	if actual < min || actual > max {
		t.tc.t.Errorf("Cycles: expected %d-%d, got %d", min, max, actual)
	}
	return t
}

func (t *ThenContext) getRegister(reg string) uint16 {
	switch strings.ToUpper(reg) {
	case "A":
		return uint16(t.tc.cpu.A)
	case "B":
		return uint16(t.tc.cpu.B)
	case "C":
		return uint16(t.tc.cpu.C)
	case "HL":
		return t.tc.cpu.HL()
	case "BC":
		return t.tc.cpu.BC()
	case "DE":
		return t.tc.cpu.DE()
	case "SP":
		return t.tc.cpu.SP
	case "PC":
		return t.tc.cpu.PC
	default:
		return 0
	}
}

// Helper to load assembled code
func (tc *TestContext) LoadBinary(address uint16, filename string) error {
	// Load .bin or .a80 file
	// Parse and load into memory at address
	return nil
}

// Helper to load MinZ symbols
func (tc *TestContext) LoadSymbols(filename string) error {
	// Load symbol table from MinZ compiler output
	return nil
}