package testing

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// MinZTest extends TestContext with MinZ-specific features
type MinZTest struct {
	*TestContext
	symbols map[string]uint16
	source  map[uint16]string // Address to source line mapping
}

// NewMinZTest creates a test context for MinZ code
func NewMinZTest(t *testing.T) *MinZTest {
	return &MinZTest{
		TestContext: NewTest(t),
		symbols:     make(map[string]uint16),
		source:      make(map[uint16]string),
	}
}

// LoadA80 loads a MinZ compiler output file
func (m *MinZTest) LoadA80(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	currentAddr := uint16(0)
	inData := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		
		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, ";") {
			continue
		}

		// Handle ORG directive
		if strings.HasPrefix(line, "ORG") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				addr, err := parseAddress(parts[1])
				if err == nil {
					currentAddr = addr
				}
			}
			continue
		}

		// Handle labels
		if strings.HasSuffix(line, ":") {
			label := strings.TrimSuffix(line, ":")
			m.symbols[label] = currentAddr
			continue
		}

		// Handle DB/DEFB directives
		if strings.HasPrefix(line, "DB") || strings.HasPrefix(line, "DEFB") {
			data := parseDataDirective(line)
			for _, b := range data {
				m.memory.data[currentAddr] = b
				currentAddr++
			}
			continue
		}

		// Handle regular instructions
		if opcode, size := parseInstruction(line); opcode != nil {
			for i, b := range opcode {
				m.memory.data[currentAddr+uint16(i)] = b
			}
			m.source[currentAddr] = line
			currentAddr += uint16(size)
		}
	}

	return scanner.Err()
}

// LoadSymbols loads a symbol table file
func (m *MinZTest) LoadSymbols(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Expected format: "symbol_name = 0x1234"
		parts := strings.Split(line, "=")
		if len(parts) == 2 {
			symbol := strings.TrimSpace(parts[0])
			addr, err := parseAddress(strings.TrimSpace(parts[1]))
			if err == nil {
				m.symbols[symbol] = addr
			}
		}
	}

	return scanner.Err()
}

// CallFunction calls a MinZ function by name
func (m *MinZTest) CallFunction(name string, args ...uint16) {
	addr, ok := m.symbols[name]
	if !ok {
		m.t.Fatalf("Function '%s' not found in symbol table", name)
	}

	// Set up arguments according to MinZ calling convention
	if len(args) > 0 {
		m.cpu.SetHL(args[0]) // First arg in HL
	}
	if len(args) > 1 {
		m.cpu.SetDE(args[1]) // Second arg in DE
	}
	if len(args) > 2 {
		// Additional args on stack
		sp := m.cpu.SP
		for i := len(args) - 1; i >= 2; i-- {
			sp -= 2
			m.memory.data[sp] = byte(args[i] & 0xFF)
			m.memory.data[sp+1] = byte(args[i] >> 8)
		}
		m.cpu.SP = sp
	}

	// Call the function
	m.When().Call(addr)
}

// GetResult returns function result (from HL register)
func (m *MinZTest) GetResult() uint16 {
	return m.cpu.HL()
}

// AssertResult checks function result
func (m *MinZTest) AssertResult(expected uint16) {
	actual := m.GetResult()
	if actual != expected {
		m.t.Errorf("Function result: expected %04X, got %04X", expected, actual)
	}
}

// Helper functions

func parseAddress(s string) (uint16, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "$")
	s = strings.TrimPrefix(s, "0x")
	s = strings.TrimPrefix(s, "0X")
	
	val, err := strconv.ParseUint(s, 16, 16)
	return uint16(val), err
}

func parseDataDirective(line string) []byte {
	// Extract data after DB/DEFB
	re := regexp.MustCompile(`(?:DB|DEFB)\s+(.+)`)
	matches := re.FindStringSubmatch(line)
	if len(matches) < 2 {
		return nil
	}

	data := []byte{}
	parts := strings.Split(matches[1], ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		
		// String literal
		if strings.HasPrefix(part, "\"") && strings.HasSuffix(part, "\"") {
			str := strings.Trim(part, "\"")
			data = append(data, []byte(str)...)
			continue
		}

		// Numeric value
		if val, err := parseNumber(part); err == nil {
			data = append(data, byte(val))
		}
	}

	return data
}

func parseNumber(s string) (uint16, error) {
	s = strings.TrimSpace(s)
	
	// Hex
	if strings.HasPrefix(s, "$") || strings.HasPrefix(s, "0x") {
		return parseAddress(s)
	}
	
	// Binary
	if strings.HasPrefix(s, "%") || strings.HasPrefix(s, "0b") {
		s = strings.TrimPrefix(s, "%")
		s = strings.TrimPrefix(s, "0b")
		val, err := strconv.ParseUint(s, 2, 16)
		return uint16(val), err
	}
	
	// Decimal
	val, err := strconv.ParseUint(s, 10, 16)
	return uint16(val), err
}

func parseInstruction(line string) ([]byte, int) {
	// This is a simplified parser - in practice, you'd use
	// a full Z80 assembler or parse the listing file
	
	// For now, return nil to indicate unsupported
	return nil, 0
}

// DSL Extensions for MinZ

func (m *MinZTest) GivenFunction(name string) *MinZGivenContext {
	return &MinZGivenContext{
		MinZTest: m,
		function: name,
	}
}

type MinZGivenContext struct {
	*MinZTest
	function string
}

func (g *MinZGivenContext) WithArgs(args ...uint16) *MinZGivenContext {
	// Store args for later use when calling
	return g
}

func (g *MinZGivenContext) WithGlobals(globals map[string]interface{}) *MinZGivenContext {
	// Set up global variables
	for name, value := range globals {
		if addr, ok := g.symbols[name]; ok {
			switch v := value.(type) {
			case byte:
				g.memory.data[addr] = v
			case uint16:
				g.memory.data[addr] = byte(v & 0xFF)
				g.memory.data[addr+1] = byte(v >> 8)
			case []byte:
				copy(g.memory.data[addr:], v)
			}
		}
	}
	return g
}

// Example test helper for common MinZ patterns
func (m *MinZTest) TestStandardFunction(name string, testCases []struct {
	Args     []uint16
	Expected uint16
}) {
	for i, tc := range testCases {
		m.t.Run(fmt.Sprintf("%s_case_%d", name, i), func(t *testing.T) {
			// Reset CPU state
			m.cpu.Reset()
			
			// Call function
			m.CallFunction(name, tc.Args...)
			
			// Check result
			m.AssertResult(tc.Expected)
		})
	}
}