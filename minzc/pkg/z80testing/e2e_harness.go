package z80testing

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/remogatto/z80"
)

// E2ETestHarness provides complete end-to-end testing for MinZ compiled programs
type E2ETestHarness struct {
	t              *testing.T
	workDir        string
	minzcPath      string
	sjasmplusPath  string
	cpu            *z80.Z80
	memory         *SMCMemory
	ports          *TestPorts
	cpuWrapper     *E2ECPUWrapper
	smcTracker     *SMCTracker
	cycleCount     int
	enableTSMC     bool
}

// E2ECPUWrapper provides cycle tracking for the Z80 emulator
type E2ECPUWrapper struct {
	z80        *z80.Z80
	baseStates int
}

func (w *E2ECPUWrapper) PC() uint16 {
	return w.z80.PC()
}

func (w *E2ECPUWrapper) Tstates() int {
	return w.z80.Tstates + w.baseStates
}

func (w *E2ECPUWrapper) AddStates(states int) {
	w.baseStates += states
}

// NewE2ETestHarness creates a new end-to-end test harness
func NewE2ETestHarness(t *testing.T) (*E2ETestHarness, error) {
	// Create temporary work directory
	workDir, err := ioutil.TempDir("", "minz-e2e-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create work directory: %w", err)
	}

	// Find MinZ compiler
	minzcPath := filepath.Join(os.Getenv("PWD"), "minzc")
	if _, err := os.Stat(minzcPath); err != nil {
		// Try relative path from test directory
		minzcPath = "./minzc"
		if _, err := os.Stat(minzcPath); err != nil {
			os.RemoveAll(workDir)
			return nil, fmt.Errorf("minzc compiler not found")
		}
	}

	// Use the known sjasmplus path
	sjasmplusPath := "/Users/alice/dev/bin/sjasmplus"
	if _, err := os.Stat(sjasmplusPath); err != nil {
		os.RemoveAll(workDir)
		return nil, fmt.Errorf("sjasmplus not found at %s: %w", sjasmplusPath, err)
	}

	// Create memory with SMC tracking
	memory := NewSMCMemory(0x8000, 0xFFFF) // Code from 0x8000 onwards
	ports := NewTestPorts()
	cpu := z80.NewZ80(memory, ports)
	cpuWrapper := &E2ECPUWrapper{z80: cpu}
	
	// Set up SMC tracking
	memory.SetCPU(cpuWrapper)

	h := &E2ETestHarness{
		t:             t,
		workDir:       workDir,
		minzcPath:     minzcPath,
		sjasmplusPath: sjasmplusPath,
		cpu:           cpu,
		memory:        memory,
		ports:         ports,
		cpuWrapper:    cpuWrapper,
		smcTracker:    memory.GetTracker(),
	}

	return h, nil
}

// Cleanup removes temporary files
func (h *E2ETestHarness) Cleanup() {
	if h.workDir != "" {
		os.RemoveAll(h.workDir)
	}
}

// CompileMinZ compiles a MinZ source file to assembly
func (h *E2ETestHarness) CompileMinZ(sourceFile string, enableTSMC bool) (string, error) {
	baseName := strings.TrimSuffix(filepath.Base(sourceFile), ".minz")
	outputFile := filepath.Join(h.workDir, baseName+".a80")

	// Build compiler arguments
	args := []string{
		sourceFile,
		"-o", outputFile,
		"-O", // Enable optimizations
	}

	if enableTSMC {
		args = append(args, "--enable-true-smc")
	}

	// Run MinZ compiler
	cmd := exec.Command(h.minzcPath, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("MinZ compilation failed: %v\nstdout: %s\nstderr: %s", 
			err, stdout.String(), stderr.String())
	}

	h.enableTSMC = enableTSMC
	return outputFile, nil
}

// AssembleA80 assembles a .a80 file to binary using sjasmplus
func (h *E2ETestHarness) AssembleA80(a80File string) ([]byte, map[string]uint16, error) {
	baseName := strings.TrimSuffix(filepath.Base(a80File), ".a80")
	binFile := filepath.Join(h.workDir, baseName+".bin")
	lstFile := filepath.Join(h.workDir, baseName+".lst")
	labFile := filepath.Join(h.workDir, baseName+".lab")

	// Run sjasmplus
	cmd := exec.Command(h.sjasmplusPath,
		"--raw="+binFile,    // Output raw binary
		"--lst="+lstFile,    // Generate listing
		"--sym="+labFile,    // Generate symbol file
		a80File,
	)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, nil, fmt.Errorf("assembly failed: %v\nstderr: %s", err, stderr.String())
	}

	// Read the binary
	binary, err := ioutil.ReadFile(binFile)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read binary: %w", err)
	}

	// Parse symbols from label file
	symbols, err := h.parseLabels(labFile)
	if err != nil {
		// Non-fatal, just log it
		h.t.Logf("Warning: failed to parse symbols: %v", err)
		symbols = make(map[string]uint16)
	}

	return binary, symbols, nil
}

// parseLabels parses sjasmplus .lab file format
func (h *E2ETestHarness) parseLabels(labFile string) (map[string]uint16, error) {
	content, err := ioutil.ReadFile(labFile)
	if err != nil {
		return nil, err
	}

	symbols := make(map[string]uint16)
	lines := strings.Split(string(content), "\n")
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, ";") {
			continue
		}

		// Format: 0x8000 label_name
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			addrStr := parts[0]
			label := parts[1]
			
			// Parse address
			addrStr = strings.TrimPrefix(addrStr, "0x")
			addrStr = strings.TrimPrefix(addrStr, "0X")
			addrStr = strings.TrimPrefix(addrStr, "$")
			
			var addr uint64
			if _, err := fmt.Sscanf(addrStr, "%x", &addr); err == nil {
				symbols[label] = uint16(addr)
			}
		}
	}

	return symbols, nil
}

// LoadBinary loads a binary into memory at the specified address
func (h *E2ETestHarness) LoadBinary(binary []byte, address uint16) {
	for i, b := range binary {
		h.memory.WriteByte(address+uint16(i), b)
	}
}

// Execute runs the loaded program
func (h *E2ETestHarness) Execute(startAddress uint16, maxCycles int) error {
	// Reset CPU state
	h.cpu.Reset()
	h.cpu.SetPC(startAddress)
	h.cpuWrapper.baseStates = 0
	h.cycleCount = 0
	
	// Clear SMC tracker
	h.smcTracker.Clear()
	h.smcTracker.Enable()

	// Execute until HALT or max cycles
	startTime := time.Now()
	for h.cycleCount < maxCycles && !h.cpu.Halted {
		prevStates := h.cpu.Tstates
		h.cpu.DoOpcode()
		h.cycleCount += (h.cpu.Tstates - prevStates)
		
		// Safety timeout
		if time.Since(startTime) > 5*time.Second {
			return fmt.Errorf("execution timeout after %d cycles", h.cycleCount)
		}
	}

	if h.cycleCount >= maxCycles {
		return fmt.Errorf("execution exceeded max cycles (%d)", maxCycles)
	}

	return nil
}

// CallFunction calls a function at a specific address with arguments
func (h *E2ETestHarness) CallFunction(address uint16, args ...uint16) error {
	// Set up arguments according to MinZ calling convention
	if len(args) > 0 {
		h.cpu.SetHL(args[0]) // First arg in HL
	}
	if len(args) > 1 {
		h.cpu.SetDE(args[1]) // Second arg in DE
	}
	if len(args) > 2 {
		// Additional args on stack
		sp := h.cpu.SP()
		for i := len(args) - 1; i >= 2; i-- {
			sp -= 2
			h.memory.WriteByte(sp, byte(args[i]&0xFF))
			h.memory.WriteByte(sp+1, byte(args[i]>>8))
		}
		h.cpu.SetSP(sp)
	}

	// Set return address (simulate CALL)
	sp := h.cpu.SP() - 2
	h.memory.WriteByte(sp, 0x00)   // Low byte of return address
	h.memory.WriteByte(sp+1, 0x00) // High byte of return address
	h.cpu.SetSP(sp)
	
	// Jump to function
	h.cpu.SetPC(address)
	
	// Clear cycle count
	h.cycleCount = 0
	h.cpuWrapper.baseStates = 0
	
	// Execute until RET (when SP returns to original value + 2)
	origSP := sp + 2
	maxCycles := 10000
	
	for h.cycleCount < maxCycles && !h.cpu.Halted {
		prevStates := h.cpu.Tstates
		h.cpu.DoOpcode()
		h.cycleCount += (h.cpu.Tstates - prevStates)
		
		// Check if we've returned
		if h.cpu.SP() == origSP && h.cpu.PC() == 0x0000 {
			break
		}
	}
	
	if h.cycleCount >= maxCycles {
		return fmt.Errorf("function execution exceeded max cycles (%d)", maxCycles)
	}
	
	return nil
}

// GetResult returns the function result from HL register
func (h *E2ETestHarness) GetResult() uint16 {
	return h.cpu.HL()
}

// GetCycles returns the number of cycles executed
func (h *E2ETestHarness) GetCycles() int {
	return h.cycleCount
}

// GetSMCStats returns SMC statistics
func (h *E2ETestHarness) GetSMCStats() SMCStats {
	return h.smcTracker.GetStats()
}

// GetSMCSummary returns a human-readable SMC summary
func (h *E2ETestHarness) GetSMCSummary() string {
	return h.smcTracker.Summary()
}

// RunE2ETest runs a complete end-to-end test
func RunE2ETest(t *testing.T, sourceFile string, testFunc func(*E2ETestHarness)) {
	h, err := NewE2ETestHarness(t)
	if err != nil {
		t.Fatalf("Failed to create test harness: %v", err)
	}
	defer h.Cleanup()

	// Test both with and without TSMC
	t.Run("without_TSMC", func(t *testing.T) {
		h.t = t
		h.enableTSMC = false
		testFunc(h)
	})

	t.Run("with_TSMC", func(t *testing.T) {
		h.t = t
		h.enableTSMC = true
		testFunc(h)
	})
}

// ComparePerformance runs the same test with and without TSMC and compares performance
func (h *E2ETestHarness) ComparePerformance(sourceFile string, funcName string, args ...uint16) (*PerformanceComparison, error) {
	result := &PerformanceComparison{
		FunctionName: funcName,
		Arguments:    args,
	}

	// Test without TSMC
	a80File, err := h.CompileMinZ(sourceFile, false)
	if err != nil {
		return nil, fmt.Errorf("compilation without TSMC failed: %w", err)
	}

	binary, symbols, err := h.AssembleA80(a80File)
	if err != nil {
		return nil, fmt.Errorf("assembly without TSMC failed: %w", err)
	}

	h.LoadBinary(binary, 0x8000)
	
	funcAddr, ok := symbols[funcName]
	if !ok {
		return nil, fmt.Errorf("function %s not found in symbols", funcName)
	}

	// Clear SMC tracker
	h.smcTracker.Clear()
	
	if err := h.CallFunction(funcAddr, args...); err != nil {
		return nil, fmt.Errorf("execution without TSMC failed: %w", err)
	}

	result.NoTSMCCycles = h.GetCycles()
	result.NoTSMCResult = h.GetResult()
	result.NoTSMCSMCEvents = h.smcTracker.CodeEventCount()

	// Test with TSMC
	a80File, err = h.CompileMinZ(sourceFile, true)
	if err != nil {
		return nil, fmt.Errorf("compilation with TSMC failed: %w", err)
	}

	binary, symbols, err = h.AssembleA80(a80File)
	if err != nil {
		return nil, fmt.Errorf("assembly with TSMC failed: %w", err)
	}

	h.LoadBinary(binary, 0x8000)
	
	funcAddr, ok = symbols[funcName]
	if !ok {
		return nil, fmt.Errorf("function %s not found in symbols", funcName)
	}

	// Clear SMC tracker
	h.smcTracker.Clear()
	
	if err := h.CallFunction(funcAddr, args...); err != nil {
		return nil, fmt.Errorf("execution with TSMC failed: %w", err)
	}

	result.TSMCCycles = h.GetCycles()
	result.TSMCResult = h.GetResult()
	result.TSMCSMCEvents = h.smcTracker.CodeEventCount()

	// Calculate improvement
	if result.NoTSMCCycles > 0 {
		result.CycleReduction = float64(result.NoTSMCCycles-result.TSMCCycles) / float64(result.NoTSMCCycles) * 100
		result.SpeedupFactor = float64(result.NoTSMCCycles) / float64(result.TSMCCycles)
	}

	return result, nil
}

// PerformanceComparison holds results of TSMC vs non-TSMC comparison
type PerformanceComparison struct {
	FunctionName    string
	Arguments       []uint16
	NoTSMCCycles    int
	TSMCCycles      int
	NoTSMCResult    uint16
	TSMCResult      uint16
	NoTSMCSMCEvents int
	TSMCSMCEvents   int
	CycleReduction  float64 // Percentage reduction in cycles
	SpeedupFactor   float64 // How many times faster
}

// String returns a human-readable summary of the comparison
func (p *PerformanceComparison) String() string {
	var buf strings.Builder
	
	fmt.Fprintf(&buf, "Performance Comparison for %s(", p.FunctionName)
	for i, arg := range p.Arguments {
		if i > 0 {
			buf.WriteString(", ")
		}
		fmt.Fprintf(&buf, "0x%04X", arg)
	}
	buf.WriteString(")\n")
	fmt.Fprintf(&buf, "========================================\n")
	fmt.Fprintf(&buf, "Without TSMC: %d cycles, result=0x%04X, SMC events=%d\n", 
		p.NoTSMCCycles, p.NoTSMCResult, p.NoTSMCSMCEvents)
	fmt.Fprintf(&buf, "With TSMC:    %d cycles, result=0x%04X, SMC events=%d\n", 
		p.TSMCCycles, p.TSMCResult, p.TSMCSMCEvents)
	fmt.Fprintf(&buf, "----------------------------------------\n")
	fmt.Fprintf(&buf, "Cycle Reduction: %.1f%%\n", p.CycleReduction)
	fmt.Fprintf(&buf, "Speedup Factor:  %.2fx\n", p.SpeedupFactor)
	
	if p.NoTSMCResult != p.TSMCResult {
		fmt.Fprintf(&buf, "WARNING: Results differ!\n")
	}
	
	return buf.String()
}

// AssertPerformanceImprovement checks that TSMC provides expected improvement
func (p *PerformanceComparison) AssertPerformanceImprovement(t *testing.T, minImprovement float64) {
	if p.NoTSMCResult != p.TSMCResult {
		t.Errorf("Results differ: no-TSMC=0x%04X, TSMC=0x%04X", p.NoTSMCResult, p.TSMCResult)
	}
	
	if p.CycleReduction < minImprovement {
		t.Errorf("Insufficient performance improvement: got %.1f%%, want at least %.1f%%",
			p.CycleReduction, minImprovement)
	}
	
	if p.TSMCSMCEvents == 0 && p.TSMCCycles < p.NoTSMCCycles {
		t.Logf("Warning: TSMC improved performance but no SMC events detected")
	}
}

// WriteTestFile writes content to a test file
func WriteTestFile(filename string, content string) error {
	return ioutil.WriteFile(filename, []byte(content), 0644)
}

// ReadTestFile reads content from a test file
func ReadTestFile(filename string) (string, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return "", err
	}
	return string(data), nil
}