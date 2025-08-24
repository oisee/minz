package tas

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
)

// ProfileMetrics contains aggregated profile data from TAS recordings
type ProfileMetrics struct {
	BlockFrequency   map[uint16]float64  // PC -> normalized frequency (0-1)
	BranchBias       map[uint16]float64  // PC -> branch bias (-1 never, +1 always)
	LoopDepth        map[uint16]int      // PC -> loop nesting depth
	WorkingSet       []uint16            // Hot memory pages
	SMCHotspots      []uint16            // Self-modifying code locations
	TotalCycles      uint64              // Total cycles in profile
	HotThreshold     uint64              // Cycles to be considered "hot"
}

// ProfileLoader loads and analyzes TAS profile data
type ProfileLoader struct {
	metrics *ProfileMetrics
}

// NewProfileLoader creates a new profile loader
func NewProfileLoader() *ProfileLoader {
	return &ProfileLoader{
		metrics: &ProfileMetrics{
			BlockFrequency: make(map[uint16]float64),
			BranchBias:     make(map[uint16]float64),
			LoopDepth:      make(map[uint16]int),
			WorkingSet:     make([]uint16, 0),
			SMCHotspots:    make([]uint16, 0),
		},
	}
}

// LoadTASProfile loads profile data from a TAS file
func (p *ProfileLoader) LoadTASProfile(filename string) (*ProfileMetrics, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open TAS file: %w", err)
	}
	defer file.Close()

	// Parse TAS file format
	tas, err := LoadFromReader(file)
	if err != nil {
		return nil, fmt.Errorf("failed to parse TAS file: %w", err)
	}

	// Analyze execution patterns
	p.analyzeExecutionHistory(tas)
	p.analyzeBranchPatterns(tas)
	p.analyzeLoopStructure(tas)
	p.analyzeSMCEvents(tas)
	p.identifyWorkingSet(tas)

	// Normalize frequencies
	p.normalizeMetrics()

	return p.metrics, nil
}

// LoadMockProfile creates mock profile data for testing
func (p *ProfileLoader) LoadMockProfile() *ProfileMetrics {
	// Mock data matching our test programs
	p.metrics.BlockFrequency = map[uint16]float64{
		0x8000: 1.0,    // hot_function entry - very hot
		0x8010: 0.8,    // main function - hot
		0x8020: 0.1,    // print routine - warm
		0x8030: 0.01,   // error handler - cold
	}
	
	p.metrics.BranchBias = map[uint16]float64{
		0x8015: 0.9,    // Loop condition - usually taken
		0x8025: -0.8,   // Error check - rarely taken
	}
	
	p.metrics.LoopDepth = map[uint16]int{
		0x8000: 0,      // Top level
		0x8015: 1,      // Inside loop
		0x8018: 2,      // Nested loop
	}
	
	p.metrics.SMCHotspots = []uint16{
		0x8001,         // Parameter patching location
		0x8005,         // Loop counter patch
	}
	
	p.metrics.WorkingSet = []uint16{
		0x8000, 0x8100, 0x8200, // Hot code pages
	}
	
	p.metrics.TotalCycles = 1000000
	p.metrics.HotThreshold = 100000
	
	return p.metrics
}

// analyzeExecutionHistory builds block frequency map from state snapshots
func (p *ProfileLoader) analyzeExecutionHistory(tas *TASFile) {
	execCounts := make(map[uint16]uint64)
	
	// Count PC occurrences in state history
	for _, state := range tas.States {
		execCounts[state.PC]++
	}
	
	// Calculate total for normalization
	var total uint64
	for _, count := range execCounts {
		total += count
		p.metrics.TotalCycles += count
	}
	
	// Convert to normalized frequencies
	if total > 0 {
		for pc, count := range execCounts {
			p.metrics.BlockFrequency[pc] = float64(count) / float64(total)
		}
	}
	
	// Determine hot threshold (top 10%)
	p.metrics.HotThreshold = total / 10
}

// analyzeBranchPatterns tracks branch taken/not-taken ratios
func (p *ProfileLoader) analyzeBranchPatterns(tas *TASFile) {
	branchTaken := make(map[uint16]uint64)
	branchTotal := make(map[uint16]uint64)
	
	// Simplified branch detection - look for JP/JR instructions
	for i := 0; i < len(tas.States)-1; i++ {
		curr := tas.States[i]
		next := tas.States[i+1]
		
		// Check if PC jumped (non-sequential)
		if next.PC != curr.PC+1 && next.PC != curr.PC+2 && next.PC != curr.PC+3 {
			branchTotal[curr.PC]++
			if next.PC != curr.PC+1 {
				branchTaken[curr.PC]++
			}
		}
	}
	
	// Calculate branch bias (-1 to +1)
	for pc, total := range branchTotal {
		if total > 0 {
			taken := branchTaken[pc]
			ratio := float64(taken) / float64(total)
			// Convert to -1..+1 scale
			p.metrics.BranchBias[pc] = (ratio * 2.0) - 1.0
		}
	}
}

// analyzeLoopStructure detects loop nesting depths
func (p *ProfileLoader) analyzeLoopStructure(tas *TASFile) {
	// Track backward jumps to detect loops
	loopHeaders := make(map[uint16]bool)
	loopBackedges := make(map[uint16]uint16) // Jump source -> target
	
	for i := 0; i < len(tas.States)-1; i++ {
		curr := tas.States[i]
		next := tas.States[i+1]
		
		// Detect backward jump (potential loop)
		if next.PC < curr.PC && next.PC >= curr.PC-256 {
			loopHeaders[next.PC] = true
			loopBackedges[curr.PC] = next.PC
		}
	}
	
	// Calculate loop depths (simplified - doesn't handle all cases)
	for pc := range p.metrics.BlockFrequency {
		depth := 0
		for backedge, header := range loopBackedges {
			if pc >= header && pc <= backedge {
				depth++
			}
		}
		if depth > 0 {
			p.metrics.LoopDepth[pc] = depth
		}
	}
}

// analyzeSMCEvents identifies self-modifying code hotspots
func (p *ProfileLoader) analyzeSMCEvents(tas *TASFile) {
	smcCounts := make(map[uint16]int)
	
	// Count SMC events by location
	for _, event := range tas.Events.SMCEvents {
		smcCounts[event.Address]++
	}
	
	// Find hotspots (>10 modifications)
	for addr, count := range smcCounts {
		if count > 10 {
			p.metrics.SMCHotspots = append(p.metrics.SMCHotspots, addr)
		}
	}
}

// identifyWorkingSet finds frequently accessed memory pages
func (p *ProfileLoader) identifyWorkingSet(tas *TASFile) {
	pageCounts := make(map[uint16]uint64)
	
	// Count accesses per 256-byte page
	for _, state := range tas.States {
		page := state.PC & 0xFF00
		pageCounts[page]++
	}
	
	// Find hot pages (top 20%)
	threshold := p.metrics.TotalCycles / 5
	for page, count := range pageCounts {
		if count > threshold {
			p.metrics.WorkingSet = append(p.metrics.WorkingSet, page)
		}
	}
}

// normalizeMetrics ensures all metrics are in proper ranges
func (p *ProfileLoader) normalizeMetrics() {
	// Find max frequency for normalization
	var maxFreq float64
	for _, freq := range p.metrics.BlockFrequency {
		if freq > maxFreq {
			maxFreq = freq
		}
	}
	
	// Normalize to 0-1 range
	if maxFreq > 0 {
		for pc := range p.metrics.BlockFrequency {
			p.metrics.BlockFrequency[pc] /= maxFreq
		}
	}
	
	// Clamp branch bias to -1..+1
	for pc := range p.metrics.BranchBias {
		p.metrics.BranchBias[pc] = math.Max(-1.0, math.Min(1.0, p.metrics.BranchBias[pc]))
	}
}

// MergeProfiles combines multiple profile runs with weighting
func MergeProfiles(profiles []*ProfileMetrics, weights []float64) *ProfileMetrics {
	if len(profiles) == 0 {
		return nil
	}
	
	// Default equal weights if not provided
	if len(weights) != len(profiles) {
		weights = make([]float64, len(profiles))
		for i := range weights {
			weights[i] = 1.0 / float64(len(profiles))
		}
	}
	
	merged := &ProfileMetrics{
		BlockFrequency: make(map[uint16]float64),
		BranchBias:     make(map[uint16]float64),
		LoopDepth:      make(map[uint16]int),
		WorkingSet:     make([]uint16, 0),
		SMCHotspots:    make([]uint16, 0),
	}
	
	// Merge block frequencies
	for i, profile := range profiles {
		for pc, freq := range profile.BlockFrequency {
			merged.BlockFrequency[pc] += freq * weights[i]
		}
	}
	
	// Merge branch biases
	for i, profile := range profiles {
		for pc, bias := range profile.BranchBias {
			merged.BranchBias[pc] += bias * weights[i]
		}
	}
	
	// Take maximum loop depth
	for _, profile := range profiles {
		for pc, depth := range profile.LoopDepth {
			if depth > merged.LoopDepth[pc] {
				merged.LoopDepth[pc] = depth
			}
		}
	}
	
	// Union of working sets
	workingSetMap := make(map[uint16]bool)
	for _, profile := range profiles {
		for _, page := range profile.WorkingSet {
			workingSetMap[page] = true
		}
	}
	for page := range workingSetMap {
		merged.WorkingSet = append(merged.WorkingSet, page)
	}
	
	// Union of SMC hotspots
	smcMap := make(map[uint16]bool)
	for _, profile := range profiles {
		for _, addr := range profile.SMCHotspots {
			smcMap[addr] = true
		}
	}
	for addr := range smcMap {
		merged.SMCHotspots = append(merged.SMCHotspots, addr)
	}
	
	// Sum total cycles
	for i, profile := range profiles {
		merged.TotalCycles += uint64(float64(profile.TotalCycles) * weights[i])
	}
	
	// Average hot threshold
	for i, profile := range profiles {
		merged.HotThreshold += uint64(float64(profile.HotThreshold) * weights[i])
	}
	
	return merged
}

// ClassifyBlock returns hot/warm/cold classification for a program counter
func (p *ProfileMetrics) ClassifyBlock(pc uint16) string {
	freq := p.BlockFrequency[pc]
	
	if freq > 0.8 {
		return "hot"
	} else if freq > 0.2 {
		return "warm"
	} else {
		return "cold"
	}
}

// GetLoopInfo returns loop optimization hints for a PC
func (p *ProfileMetrics) GetLoopInfo(pc uint16) (isLoop bool, depth int, isHot bool) {
	depth = p.LoopDepth[pc]
	isLoop = depth > 0
	
	freq := p.BlockFrequency[pc]
	isHot = freq > 0.5
	
	return
}

// ShouldInline provides inlining recommendation based on profile
func (p *ProfileMetrics) ShouldInline(callSite uint16, fnSize int) bool {
	freq := p.BlockFrequency[callSite]
	
	// Hot + Small = Always inline
	if freq > 0.8 && fnSize < 10 {
		return true
	}
	
	// Cold = Never inline (code size matters)
	if freq < 0.1 {
		return false
	}
	
	// Cost/benefit analysis
	callCost := 17.0  // CALL + RET T-states
	benefit := freq * callCost
	cost := float64(fnSize * 4)  // Average instruction size
	
	return benefit > cost
}

// LoadFromReader reads a TAS file from an io.Reader
func LoadFromReader(r io.Reader) (*TASFile, error) {
	// This is a simplified loader - real implementation would handle
	// the full TAS format with compression, versioning, etc.
	
	tas := &TASFile{
		Header: TASHeader{
			Magic:   [8]byte{'T', 'A', 'S', 'Z', '8', '0', 'v', '1'},
			Version: 1,
			Format:  TASFormatBinary,
		},
		States: make([]StateSnapshot, 0),
		Events: TASEvents{
			Inputs:    make([]InputEvent, 0),
			SMCEvents: make([]SMCEvent, 0),
		},
	}
	
	// Read header
	if err := binary.Read(r, binary.LittleEndian, &tas.Header); err != nil {
		return nil, fmt.Errorf("failed to read TAS header: %w", err)
	}
	
	// Verify magic
	expectedMagic := [8]byte{'T', 'A', 'S', 'Z', '8', '0', 'v', '1'}
	if tas.Header.Magic != expectedMagic {
		return nil, fmt.Errorf("invalid TAS file magic")
	}
	
	// Read metadata
	var metaLen uint32
	if err := binary.Read(r, binary.LittleEndian, &metaLen); err != nil {
		return nil, err
	}
	
	metaBytes := make([]byte, metaLen)
	if _, err := io.ReadFull(r, metaBytes); err != nil {
		return nil, err
	}
	
	// Read states count
	var stateCount uint32
	if err := binary.Read(r, binary.LittleEndian, &stateCount); err != nil {
		return nil, err
	}
	
	// Read states (simplified - would be compressed in real format)
	tas.States = make([]StateSnapshot, stateCount)
	for i := uint32(0); i < stateCount; i++ {
		// Read only essential fields for profile analysis
		if err := binary.Read(r, binary.LittleEndian, &tas.States[i].PC); err != nil {
			return nil, err
		}
		if err := binary.Read(r, binary.LittleEndian, &tas.States[i].Cycle); err != nil {
			return nil, err
		}
	}
	
	return tas, nil
}