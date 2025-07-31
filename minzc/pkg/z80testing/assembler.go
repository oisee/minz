package z80testing

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Assembler interface for assembling Z80 assembly code
type Assembler interface {
	Assemble(sourceFile string) ([]byte, error)
	AssembleWithSymbols(sourceFile string) ([]byte, map[string]uint16, error)
}

// SjasmPlusAssembler wraps the sjasmplus Z80 assembler
type SjasmPlusAssembler struct {
	workDir    string
	sjasmplusPath string
}

// NewSjasmPlusAssembler creates a new sjasmplus assembler wrapper
func NewSjasmPlusAssembler() (*SjasmPlusAssembler, error) {
	// Create temporary work directory
	workDir, err := os.MkdirTemp("", "minz-asm-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create work directory: %w", err)
	}

	// Find sjasmplus executable
	sjasmplusPath, err := exec.LookPath("sjasmplus")
	if err != nil {
		os.RemoveAll(workDir)
		return nil, fmt.Errorf("sjasmplus not found in PATH: %w", err)
	}

	return &SjasmPlusAssembler{
		workDir:    workDir,
		sjasmplusPath: sjasmplusPath,
	}, nil
}

// Cleanup removes temporary files
func (a *SjasmPlusAssembler) Cleanup() {
	if a.workDir != "" {
		os.RemoveAll(a.workDir)
	}
}

// Assemble assembles a .a80 file to binary
func (a *SjasmPlusAssembler) Assemble(sourceFile string) ([]byte, error) {
	// Create output filename
	baseName := strings.TrimSuffix(filepath.Base(sourceFile), filepath.Ext(sourceFile))
	outFile := filepath.Join(a.workDir, baseName+".bin")

	// Run sjasmplus
	cmd := exec.Command(a.sjasmplusPath,
		"--raw="+outFile,    // Output raw binary
		"--lst=off",         // No listing file
		sourceFile,
	)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("assembly failed: %v\nstderr: %s", err, stderr.String())
	}

	// Read the binary output
	binary, err := os.ReadFile(outFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read output binary: %w", err)
	}

	return binary, nil
}

// AssembleWithSymbols assembles and extracts symbol table
func (a *SjasmPlusAssembler) AssembleWithSymbols(sourceFile string) ([]byte, map[string]uint16, error) {
	// Create output filenames
	baseName := strings.TrimSuffix(filepath.Base(sourceFile), filepath.Ext(sourceFile))
	outFile := filepath.Join(a.workDir, baseName+".bin")
	lstFile := filepath.Join(a.workDir, baseName+".lst")

	// Run sjasmplus with listing for symbols
	cmd := exec.Command(a.sjasmplusPath,
		"--raw="+outFile,    // Output raw binary
		"--lst="+lstFile,    // Generate listing file
		sourceFile,
	)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, nil, fmt.Errorf("assembly failed: %v\nstderr: %s", err, stderr.String())
	}

	// Read the binary output
	binary, err := os.ReadFile(outFile)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read output binary: %w", err)
	}

	// Parse symbols from listing file
	symbols, err := parseSjasmPlusListing(lstFile)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse symbols: %w", err)
	}

	return binary, symbols, nil
}

// parseSjasmPlusListing extracts symbols from sjasmplus listing file
func parseSjasmPlusListing(listingFile string) (map[string]uint16, error) {
	file, err := os.Open(listingFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	symbols := make(map[string]uint16)
	scanner := bufio.NewScanner(file)

	// Regular expressions for parsing
	labelRe := regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*):`)
	equRe := regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)\s+EQU\s+(?:\$|0x)?([0-9A-Fa-f]+)`)
	addressRe := regexp.MustCompile(`^([0-9A-Fa-f]{4})\s+`)

	var currentAddress uint16

	for scanner.Scan() {
		line := scanner.Text()

		// Extract address from line
		if matches := addressRe.FindStringSubmatch(line); matches != nil {
			addr, _ := strconv.ParseUint(matches[1], 16, 16)
			currentAddress = uint16(addr)
		}

		// Check for labels
		if matches := labelRe.FindStringSubmatch(line); matches != nil {
			symbols[matches[1]] = currentAddress
		}

		// Check for EQU directives
		if matches := equRe.FindStringSubmatch(line); matches != nil {
			value, _ := strconv.ParseUint(matches[2], 16, 16)
			symbols[matches[1]] = uint16(value)
		}
	}

	return symbols, scanner.Err()
}

// SimpleAssembler for testing without external dependencies
type SimpleAssembler struct {
	org uint16
}

// NewSimpleAssembler creates a basic assembler for testing
func NewSimpleAssembler() *SimpleAssembler {
	return &SimpleAssembler{
		org: 0x8000, // Default origin
	}
}

// Assemble converts simple assembly to machine code (limited subset)
func (s *SimpleAssembler) Assemble(sourceFile string) ([]byte, error) {
	content, err := os.ReadFile(sourceFile)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(content), "\n")
	var output []byte
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, ";") {
			continue
		}

		// Parse ORG directive
		if strings.HasPrefix(line, "ORG") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				addr := parseHexValue(parts[1])
				s.org = addr
			}
			continue
		}

		// Parse basic instructions (very limited)
		if bytes := s.assembleLine(line); bytes != nil {
			output = append(output, bytes...)
		}
	}

	return output, nil
}

// AssembleWithSymbols is not implemented for SimpleAssembler
func (s *SimpleAssembler) AssembleWithSymbols(sourceFile string) ([]byte, map[string]uint16, error) {
	binary, err := s.Assemble(sourceFile)
	if err != nil {
		return nil, nil, err
	}
	// Return empty symbol table
	return binary, make(map[string]uint16), nil
}

// assembleLine converts a single line to machine code
func (s *SimpleAssembler) assembleLine(line string) []byte {
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return nil
	}

	inst := strings.ToUpper(parts[0])
	
	switch inst {
	case "NOP":
		return []byte{0x00}
	case "RET":
		return []byte{0xC9}
	case "LD":
		if len(parts) >= 3 {
			return s.assembleLD(parts[1], parts[2])
		}
	case "CALL":
		if len(parts) >= 2 {
			addr := parseHexValue(parts[1])
			return []byte{0xCD, byte(addr & 0xFF), byte(addr >> 8)}
		}
	case "JP":
		if len(parts) >= 2 {
			addr := parseHexValue(parts[1])
			return []byte{0xC3, byte(addr & 0xFF), byte(addr >> 8)}
		}
	}

	return nil
}

// assembleLD handles LD instructions
func (s *SimpleAssembler) assembleLD(dest, src string) []byte {
	dest = strings.TrimSuffix(strings.ToUpper(dest), ",")
	src = strings.ToUpper(src)

	// LD A, immediate
	if dest == "A" && isImmediate(src) {
		val := parseImmediate(src)
		return []byte{0x3E, val}
	}

	// LD HL, immediate16
	if dest == "HL" && isImmediate(src) {
		val := parseHexValue(src)
		return []byte{0x21, byte(val & 0xFF), byte(val >> 8)}
	}

	// LD B, immediate
	if dest == "B" && isImmediate(src) {
		val := parseImmediate(src)
		return []byte{0x06, val}
	}

	// LD C, immediate
	if dest == "C" && isImmediate(src) {
		val := parseImmediate(src)
		return []byte{0x0E, val}
	}

	// LD A, B
	if dest == "A" && src == "B" {
		return []byte{0x78}
	}
	
	// LD B, A
	if dest == "B" && src == "A" {
		return []byte{0x47}
	}

	// LD A, C
	if dest == "A" && src == "C" {
		return []byte{0x79}
	}

	return nil
}

// Helper functions
func isImmediate(s string) bool {
	return strings.HasPrefix(s, "#") || strings.HasPrefix(s, "$") || 
	       strings.HasPrefix(s, "0X") || (s[0] >= '0' && s[0] <= '9')
}

func parseImmediate(s string) byte {
	s = strings.TrimPrefix(s, "#")
	// Check if it's decimal
	if !strings.HasPrefix(s, "$") && !strings.HasPrefix(s, "0x") && !strings.HasPrefix(s, "0X") {
		val, _ := strconv.ParseUint(s, 10, 8)
		return byte(val)
	}
	val := parseHexValue(s)
	return byte(val)
}

func parseHexValue(s string) uint16 {
	s = strings.TrimPrefix(s, "$")
	s = strings.TrimPrefix(s, "0x")
	s = strings.TrimPrefix(s, "0X")
	
	val, _ := strconv.ParseUint(s, 16, 16)
	return uint16(val)
}