package emulator

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/remogatto/z80"
)

// FUSE Z80 test suite integration
// Tests the remogatto/z80 CPU core against the gold-standard FUSE test vectors.
// Test data from: https://github.com/ggambetta/libz80/tree/master/fuse_tests/fuse_files
//
// Key insight from the library's own (broken) z80_test.go reference implementation:
// - ReadByte/WriteByte add 3 T-states (memory access timing)
// - ReadByteInternal/WriteByteInternal add 0 T-states (raw access)
// - ContendRead/etc add their time parameter to Tstates
// - ReadPort/WritePort handle all port timing (1 pre + 3 post = 4 T-states)
// - Port reads return high byte of address (FUSE convention)

// fuseTestInput represents a single FUSE test input
type fuseTestInput struct {
	Name string

	// Registers: AF BC DE HL AF' BC' DE' HL' IX IY SP PC
	AF, BC, DE, HL    uint16
	AF_, BC_, DE_, HL_ uint16
	IX, IY, SP, PC    uint16

	// Other state
	I, R       byte
	IFF1, IFF2 byte
	IM         byte
	Halted     bool
	Tstates    int // EventNextEvent from FUSE format (not initial Tstates)

	// Memory blocks: addr -> bytes
	MemBlocks []memBlock
}

// fuseTestExpected represents expected output after executing one instruction
type fuseTestExpected struct {
	Name string

	// Registers: AF BC DE HL AF' BC' DE' HL' IX IY SP PC
	AF, BC, DE, HL    uint16
	AF_, BC_, DE_, HL_ uint16
	IX, IY, SP, PC    uint16

	// Other state
	I, R       byte
	IFF1, IFF2 byte
	IM         byte
	Halted     bool
	Tstates    int

	// Memory blocks that should have changed
	MemBlocks []memBlock
}

type memBlock struct {
	Addr  uint16
	Bytes []byte
}

// fuseMemory implements z80.MemoryAccessor for FUSE tests.
// ReadByte/WriteByte include 3 T-state timing (memory access cycle).
// ContendRead/etc add their time parameter for contention.
type fuseMemory struct {
	cpu  *z80.Z80
	data [65536]byte
}

func (m *fuseMemory) ReadByte(address uint16) byte {
	m.cpu.Tstates += 3
	return m.data[address]
}
func (m *fuseMemory) WriteByte(address uint16, value byte) {
	m.cpu.Tstates += 3
	m.data[address] = value
}
func (m *fuseMemory) ReadByteInternal(address uint16) byte         { return m.data[address] }
func (m *fuseMemory) WriteByteInternal(address uint16, value byte) { m.data[address] = value }
func (m *fuseMemory) Read(address uint16) byte                     { return m.data[address] }
func (m *fuseMemory) Write(address uint16, value byte, protectROM bool) {
	m.data[address] = value
}
func (m *fuseMemory) Data() []byte { return m.data[:] }
func (m *fuseMemory) ContendRead(address uint16, time int) {
	m.cpu.Tstates += time
}
func (m *fuseMemory) ContendReadNoMreq(address uint16, time int) {
	m.cpu.Tstates += time
}
func (m *fuseMemory) ContendWriteNoMreq(address uint16, time int) {
	m.cpu.Tstates += time
}
func (m *fuseMemory) ContendReadNoMreq_loop(address uint16, time int, count uint) {
	for i := uint(0); i < count; i++ {
		m.cpu.Tstates += time
	}
}
func (m *fuseMemory) ContendWriteNoMreq_loop(address uint16, time int, count uint) {
	for i := uint(0); i < count; i++ {
		m.cpu.Tstates += time
	}
}

// fusePorts implements z80.PortAccessor for FUSE tests.
// The library only calls ReadPort/WritePort (never the Contend/Internal variants directly).
// ReadPort returns high byte of address (FUSE convention for floating bus).
// Port timing: 1 T-state pre-io + 3 T-states post-io = 4 T-states total.
type fusePorts struct {
	cpu *z80.Z80
}

func (p *fusePorts) ReadPort(address uint16) byte {
	p.cpu.Tstates += 1 // pre-io contention
	val := byte(address >> 8)
	p.cpu.Tstates += 3 // post-io contention
	return val
}
func (p *fusePorts) WritePort(address uint16, b byte) {
	p.cpu.Tstates += 1 // pre-io contention
	p.cpu.Tstates += 3 // post-io contention
}
func (p *fusePorts) ReadPortInternal(address uint16, contend bool) byte { return byte(address >> 8) }
func (p *fusePorts) WritePortInternal(address uint16, b byte, contend bool) {}
func (p *fusePorts) ContendPortPreio(address uint16)                       {}
func (p *fusePorts) ContendPortPostio(address uint16)                      {}

// parseFuseInputs parses the FUSE tests.in file
func parseFuseInputs(filename string) ([]fuseTestInput, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var tests []fuseTestInput
	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var test fuseTestInput
		test.Name = line

		// Read registers line: AF BC DE HL AF' BC' DE' HL' IX IY SP PC
		if !scanner.Scan() {
			break
		}
		regs := strings.Fields(scanner.Text())
		if len(regs) < 12 {
			return nil, fmt.Errorf("test %s: expected 12 register values, got %d", test.Name, len(regs))
		}
		test.AF = parseHex16(regs[0])
		test.BC = parseHex16(regs[1])
		test.DE = parseHex16(regs[2])
		test.HL = parseHex16(regs[3])
		test.AF_ = parseHex16(regs[4])
		test.BC_ = parseHex16(regs[5])
		test.DE_ = parseHex16(regs[6])
		test.HL_ = parseHex16(regs[7])
		test.IX = parseHex16(regs[8])
		test.IY = parseHex16(regs[9])
		test.SP = parseHex16(regs[10])
		test.PC = parseHex16(regs[11])

		// Read state line: I R IFF1 IFF2 IM halted tstates
		if !scanner.Scan() {
			break
		}
		state := strings.Fields(scanner.Text())
		if len(state) < 7 {
			return nil, fmt.Errorf("test %s: expected 7 state values, got %d", test.Name, len(state))
		}
		test.I = parseHex8(state[0])
		test.R = parseHex8(state[1])
		test.IFF1 = parseHex8(state[2])
		test.IFF2 = parseHex8(state[3])
		test.IM = parseHex8(state[4])
		test.Halted = state[5] != "0"
		test.Tstates, _ = strconv.Atoi(state[6])

		// Read memory blocks until -1
		for scanner.Scan() {
			memLine := strings.TrimSpace(scanner.Text())
			if memLine == "-1" {
				break
			}
			fields := strings.Fields(memLine)
			if len(fields) < 2 {
				continue
			}

			addr := parseHex16(fields[0])
			var bytes []byte
			for _, f := range fields[1:] {
				if f == "-1" {
					break
				}
				bytes = append(bytes, parseHex8(f))
			}
			if len(bytes) > 0 {
				test.MemBlocks = append(test.MemBlocks, memBlock{Addr: addr, Bytes: bytes})
			}
		}

		tests = append(tests, test)
	}

	return tests, scanner.Err()
}

// parseFuseExpected parses the FUSE tests.expected file
func parseFuseExpected(filename string) (map[string]*fuseTestExpected, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	results := make(map[string]*fuseTestExpected)
	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		test := &fuseTestExpected{Name: line}

		// Skip event lines (start with spaces and contain MC/MR/MW/PC/PR/PW)
		for scanner.Scan() {
			eventLine := scanner.Text()
			trimmed := strings.TrimSpace(eventLine)
			// Event lines start with whitespace and contain timing info
			if len(eventLine) > 0 && (eventLine[0] == ' ' || eventLine[0] == '\t') {
				continue // skip event line
			}
			// This is the register line
			regs := strings.Fields(trimmed)
			if len(regs) < 12 {
				break
			}
			test.AF = parseHex16(regs[0])
			test.BC = parseHex16(regs[1])
			test.DE = parseHex16(regs[2])
			test.HL = parseHex16(regs[3])
			test.AF_ = parseHex16(regs[4])
			test.BC_ = parseHex16(regs[5])
			test.DE_ = parseHex16(regs[6])
			test.HL_ = parseHex16(regs[7])
			test.IX = parseHex16(regs[8])
			test.IY = parseHex16(regs[9])
			test.SP = parseHex16(regs[10])
			test.PC = parseHex16(regs[11])
			break
		}

		// Read state line: I R IFF1 IFF2 IM halted tstates
		if !scanner.Scan() {
			break
		}
		state := strings.Fields(strings.TrimSpace(scanner.Text()))
		if len(state) < 7 {
			continue
		}
		test.I = parseHex8(state[0])
		test.R = parseHex8(state[1])
		test.IFF1 = parseHex8(state[2])
		test.IFF2 = parseHex8(state[3])
		test.IM = parseHex8(state[4])
		test.Halted = state[5] != "0"
		test.Tstates, _ = strconv.Atoi(state[6])

		// Read memory blocks until blank line or next test
		for scanner.Scan() {
			memLine := strings.TrimSpace(scanner.Text())
			if memLine == "" {
				break
			}
			fields := strings.Fields(memLine)
			if len(fields) == 0 {
				break
			}
			// Check for end marker
			if fields[0] == "-1" {
				// Read until blank line
				for scanner.Scan() {
					if strings.TrimSpace(scanner.Text()) == "" {
						break
					}
				}
				break
			}

			addr := parseHex16(fields[0])
			var bytes []byte
			for _, f := range fields[1:] {
				if f == "-1" {
					break
				}
				bytes = append(bytes, parseHex8(f))
			}
			if len(bytes) > 0 {
				test.MemBlocks = append(test.MemBlocks, memBlock{Addr: addr, Bytes: bytes})
			}
		}

		results[test.Name] = test
	}

	return results, scanner.Err()
}

func parseHex16(s string) uint16 {
	v, _ := strconv.ParseUint(s, 16, 16)
	return uint16(v)
}

func parseHex8(s string) byte {
	v, _ := strconv.ParseUint(s, 16, 8)
	return byte(v)
}

// setupCPU configures Z80 state from a FUSE test input
func setupCPU(cpu *z80.Z80, mem *fuseMemory, input *fuseTestInput) {
	// Set registers
	cpu.A = byte(input.AF >> 8)
	cpu.F = byte(input.AF & 0xFF)
	cpu.B = byte(input.BC >> 8)
	cpu.C = byte(input.BC & 0xFF)
	cpu.D = byte(input.DE >> 8)
	cpu.E = byte(input.DE & 0xFF)
	cpu.H = byte(input.HL >> 8)
	cpu.L = byte(input.HL & 0xFF)

	// Shadow registers
	cpu.A_ = byte(input.AF_ >> 8)
	cpu.F_ = byte(input.AF_ & 0xFF)
	cpu.B_ = byte(input.BC_ >> 8)
	cpu.C_ = byte(input.BC_ & 0xFF)
	cpu.D_ = byte(input.DE_ >> 8)
	cpu.E_ = byte(input.DE_ & 0xFF)
	cpu.H_ = byte(input.HL_ >> 8)
	cpu.L_ = byte(input.HL_ & 0xFF)

	// Index registers
	cpu.IXH = byte(input.IX >> 8)
	cpu.IXL = byte(input.IX & 0xFF)
	cpu.IYH = byte(input.IY >> 8)
	cpu.IYL = byte(input.IY & 0xFF)

	// SP, PC
	cpu.SetSP(input.SP)
	cpu.SetPC(input.PC)

	// I, R
	cpu.I = input.I
	cpu.R7 = input.R & 0x80
	cpu.R = uint16(input.R & 0x7F)

	// IFF, IM
	cpu.IFF1 = input.IFF1
	cpu.IFF2 = input.IFF2
	cpu.IM = input.IM
	cpu.Halted = input.Halted

	// Tstates starts at 0; input.Tstates is EventNextEvent (not used)
	cpu.Tstates = 0

	// Load memory
	for _, mb := range input.MemBlocks {
		for i, b := range mb.Bytes {
			mem.data[mb.Addr+uint16(i)] = b
		}
	}
}

// getCPUAF returns the AF register value
func getCPUAF(cpu *z80.Z80) uint16 {
	return uint16(cpu.A)<<8 | uint16(cpu.F)
}

// getCPUR returns the full R register value (R7 bit 7 + R bits 0-6)
func getCPUR(cpu *z80.Z80) byte {
	return (cpu.R7 & 0x80) | byte(cpu.R&0x7F)
}

// compareCPU compares CPU state with expected output
func compareCPU(t *testing.T, name string, cpu *z80.Z80, mem *fuseMemory, expected *fuseTestExpected) {
	t.Helper()

	check := func(regName string, got, want uint16) {
		if got != want {
			t.Errorf("  %s: got %04X, want %04X", regName, got, want)
		}
	}

	check8 := func(regName string, got, want byte) {
		if got != want {
			t.Errorf("  %s: got %02X, want %02X", regName, got, want)
		}
	}

	check("AF", getCPUAF(cpu), expected.AF)
	check("BC", cpu.BC(), expected.BC)
	check("DE", cpu.DE(), expected.DE)
	check("HL", cpu.HL(), expected.HL)
	check("AF'", uint16(cpu.A_)<<8|uint16(cpu.F_), expected.AF_)
	check("BC'", cpu.BC_(), expected.BC_)
	check("DE'", cpu.DE_(), expected.DE_)
	check("HL'", cpu.HL_(), expected.HL_)
	check("IX", cpu.IX(), expected.IX)
	check("IY", cpu.IY(), expected.IY)
	check("SP", cpu.SP(), expected.SP)
	check("PC", cpu.PC(), expected.PC)
	check8("I", cpu.I, expected.I)
	check8("R", getCPUR(cpu), expected.R)
	check8("IFF1", cpu.IFF1, expected.IFF1)
	check8("IFF2", cpu.IFF2, expected.IFF2)
	check8("IM", cpu.IM, expected.IM)

	if cpu.Halted != expected.Halted {
		t.Errorf("  Halted: got %v, want %v", cpu.Halted, expected.Halted)
	}

	if cpu.Tstates != expected.Tstates {
		t.Errorf("  Tstates: got %d, want %d", cpu.Tstates, expected.Tstates)
	}

	// Check memory
	for _, mb := range expected.MemBlocks {
		for i, want := range mb.Bytes {
			addr := mb.Addr + uint16(i)
			got := mem.data[addr]
			if got != want {
				t.Errorf("  mem[%04X]: got %02X, want %02X", addr, got, want)
			}
		}
	}
}

func TestFUSE(t *testing.T) {
	inputs, err := parseFuseInputs("testdata/fuse_tests.in")
	if err != nil {
		t.Fatalf("Failed to parse FUSE inputs: %v", err)
	}

	expected, err := parseFuseExpected("testdata/fuse_tests.expected")
	if err != nil {
		t.Fatalf("Failed to parse FUSE expected: %v", err)
	}

	t.Logf("Loaded %d FUSE test inputs, %d expected results", len(inputs), len(expected))

	passed := 0
	failed := 0
	skipped := 0

	for _, input := range inputs {
		exp, ok := expected[input.Name]
		if !ok {
			skipped++
			continue
		}

		input := input // capture loop var
		t.Run(input.Name, func(t *testing.T) {
			mem := &fuseMemory{}
			ports := &fusePorts{}
			cpu := z80.NewZ80(mem, ports)

			// Wire up CPU references for contention tracking
			mem.cpu = cpu
			ports.cpu = cpu

			setupCPU(cpu, mem, &input)

			// Execute instructions until Tstates reaches EventNextEvent.
			// Most tests have EventNextEvent=1 so only one instruction runs
			// (every Z80 instruction takes >= 4 T-states).
			// Multi-instruction tests (DJNZ loops, LDIR repeats) have higher values.
			eventNextEvent := input.Tstates
			for cpu.Tstates < eventNextEvent {
				cpu.DoOpcode()
			}

			compareCPU(t, input.Name, cpu, mem, exp)
		})
	}

	t.Logf("FUSE results: %d passed, %d failed, %d skipped (no expected data)",
		passed, failed, skipped)
}
