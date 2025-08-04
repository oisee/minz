package tas

import (
	"fmt"
)

// DeterminismDetector identifies deterministic execution sections
type DeterminismDetector struct {
	// Tracking state
	isDeterministic    bool
	lastNonDetCycle    int64
	deterministicStart int64
	
	// Statistics
	totalCycles        int64
	deterministicCycles int64
	sections           []DeterministicSection
	
	// Configuration
	minSectionLength   int64 // Minimum cycles to consider deterministic
	ioGracePeriod      int64 // Cycles without I/O before deterministic
	
	// Pattern recognition
	loopDetector      *LoopDetector
	patternCache      map[uint64]Pattern
}

// DeterministicSection represents a deterministic execution stretch
type DeterministicSection struct {
	StartCycle   int64
	EndCycle     int64
	Length       int64
	Type         DeterminismType
	LoopCount    int      // If it's a loop
	PatternHash  uint64   // Hash of instruction pattern
	Compressible bool     // Can be compressed to just boundary events
}

type DeterminismType byte

const (
	DetLinear     DeterminismType = iota // Straight-line code
	DetLoop                               // Repeated loop
	DetRecursive                          // Recursive pattern
	DetCompute                            // Pure computation
)

// Pattern represents a repeating instruction pattern
type Pattern struct {
	Hash         uint64
	Instructions []byte
	Length       int
	RepeatCount  int
}

// NewDeterminismDetector creates a new determinism detector
func NewDeterminismDetector() *DeterminismDetector {
	return &DeterminismDetector{
		minSectionLength: 100,   // At least 100 cycles
		ioGracePeriod:    50,    // 50 cycles without I/O
		loopDetector:     NewLoopDetector(),
		patternCache:     make(map[uint64]Pattern),
		sections:         make([]DeterministicSection, 0),
	}
}

// ProcessEvent analyzes an event for determinism
func (d *DeterminismDetector) ProcessEvent(event CycleEvent) {
	d.totalCycles = event.Cycle
	
	switch event.Type {
	case EventIORead, EventIOWrite, EventInterrupt:
		// These break determinism
		d.handleNonDeterministicEvent(event.Cycle)
		
	case EventInstruction:
		// Check if we're entering deterministic mode
		if !d.isDeterministic {
			cyclesSinceIO := event.Cycle - d.lastNonDetCycle
			if cyclesSinceIO >= d.ioGracePeriod {
				d.enterDeterministicMode(event.Cycle - cyclesSinceIO)
			}
		}
		
		// Process instruction for pattern detection
		if d.isDeterministic {
			if inst, ok := event.Data.(InstructionData); ok {
				d.loopDetector.ProcessInstruction(inst.Opcode, inst.PC)
			}
		}
		
	case EventMemoryRead, EventMemoryWrite:
		// Memory access is usually deterministic
		// unless it's to I/O-mapped regions
		if d.isDeterministic {
			if mem, ok := event.Data.(MemoryAccessData); ok {
				if d.isIOMemoryMapped(mem.Address) {
					d.handleNonDeterministicEvent(event.Cycle)
				}
			}
		}
		
	case EventSMC:
		// SMC is deterministic but must be recorded
		// It doesn't break determinism but affects replay
		if d.isDeterministic {
			d.markSMCInSection(event.Cycle)
		}
	}
}

// handleNonDeterministicEvent handles events that break determinism
func (d *DeterminismDetector) handleNonDeterministicEvent(cycle int64) {
	if d.isDeterministic {
		// End current deterministic section
		d.exitDeterministicMode(cycle)
	}
	d.lastNonDetCycle = cycle
}

// enterDeterministicMode starts tracking a deterministic section
func (d *DeterminismDetector) enterDeterministicMode(startCycle int64) {
	d.isDeterministic = true
	d.deterministicStart = startCycle
	d.loopDetector.Reset()
	
	fmt.Printf("🟢 Entering deterministic mode at cycle %d\n", startCycle)
}

// exitDeterministicMode ends a deterministic section
func (d *DeterminismDetector) exitDeterministicMode(endCycle int64) {
	if !d.isDeterministic {
		return
	}
	
	length := endCycle - d.deterministicStart
	if length >= d.minSectionLength {
		// Record significant section
		section := DeterministicSection{
			StartCycle:   d.deterministicStart,
			EndCycle:     endCycle,
			Length:       length,
			Type:         d.classifySection(),
			Compressible: true,
		}
		
		// Check for loops
		if loop := d.loopDetector.GetLongestLoop(); loop != nil {
			section.LoopCount = loop.Count
			section.PatternHash = loop.Hash
		}
		
		d.sections = append(d.sections, section)
		d.deterministicCycles += length
		
		fmt.Printf("🔴 Exiting deterministic mode at cycle %d (length: %d, type: %v)\n", 
			endCycle, length, section.Type)
	}
	
	d.isDeterministic = false
}

// classifySection determines the type of deterministic section
func (d *DeterminismDetector) classifySection() DeterminismType {
	if d.loopDetector.HasLoop() {
		return DetLoop
	}
	if d.loopDetector.HasRecursion() {
		return DetRecursive
	}
	// TODO: Add more classification logic
	return DetLinear
}

// markSMCInSection marks that SMC occurred in current section
func (d *DeterminismDetector) markSMCInSection(cycle int64) {
	// SMC means we need to record the modification
	// but the section can still be compressed
	fmt.Printf("⚡ SMC event at cycle %d in deterministic section\n", cycle)
}

// isIOMemoryMapped checks if address is memory-mapped I/O
func (d *DeterminismDetector) isIOMemoryMapped(addr uint16) bool {
	// ZX Spectrum memory-mapped I/O regions
	// This is platform-specific
	return false // Most Z80 systems use port I/O
}

// GetStatistics returns determinism statistics
func (d *DeterminismDetector) GetStatistics() DeterminismStats {
	ratio := float64(0)
	if d.totalCycles > 0 {
		ratio = float64(d.deterministicCycles) / float64(d.totalCycles)
	}
	
	return DeterminismStats{
		TotalCycles:         d.totalCycles,
		DeterministicCycles: d.deterministicCycles,
		Ratio:               ratio,
		SectionCount:        len(d.sections),
		AverageLength:       d.getAverageSectionLength(),
		CompressionPotential: d.calculateCompressionPotential(),
	}
}

// getAverageSectionLength calculates average deterministic section length
func (d *DeterminismDetector) getAverageSectionLength() float64 {
	if len(d.sections) == 0 {
		return 0
	}
	
	total := int64(0)
	for _, section := range d.sections {
		total += section.Length
	}
	
	return float64(total) / float64(len(d.sections))
}

// calculateCompressionPotential estimates compression ratio
func (d *DeterminismDetector) calculateCompressionPotential() float64 {
	if d.totalCycles == 0 {
		return 1.0
	}
	
	// In deterministic sections, we only need:
	// - Start/end markers (2 events)
	// - SMC events within
	// - Loop parameters if looping
	
	compressedSize := int64(len(d.sections) * 2) // Start/end markers
	
	for _, section := range d.sections {
		if section.Type == DetLoop {
			compressedSize += 10 // Loop overhead
		}
		// Add estimated SMC events (rare)
		compressedSize += section.Length / 10000
	}
	
	// Original size would be every cycle
	originalSize := d.totalCycles
	
	return float64(originalSize) / float64(compressedSize)
}

// DeterminismStats contains statistics about deterministic execution
type DeterminismStats struct {
	TotalCycles          int64
	DeterministicCycles  int64
	Ratio                float64
	SectionCount         int
	AverageLength        float64
	CompressionPotential float64
}

// LoopDetector identifies repeating instruction patterns
type LoopDetector struct {
	pcHistory    []uint16
	opcodeHistory []byte
	maxHistory   int
	loops        []Loop
	callStack    []uint16 // For recursion detection
}

// Loop represents a detected loop pattern
type Loop struct {
	StartPC  uint16
	EndPC    uint16
	Length   int
	Count    int
	Hash     uint64
	Pattern  []byte
}

// NewLoopDetector creates a new loop detector
func NewLoopDetector() *LoopDetector {
	return &LoopDetector{
		pcHistory:     make([]uint16, 0, 1000),
		opcodeHistory: make([]byte, 0, 1000),
		maxHistory:    1000,
		loops:         make([]Loop, 0),
		callStack:     make([]uint16, 0, 32),
	}
}

// ProcessInstruction analyzes instruction for loop patterns
func (l *LoopDetector) ProcessInstruction(opcode byte, pc uint16) {
	// Add to history
	l.pcHistory = append(l.pcHistory, pc)
	l.opcodeHistory = append(l.opcodeHistory, opcode)
	
	// Limit history size
	if len(l.pcHistory) > l.maxHistory {
		l.pcHistory = l.pcHistory[1:]
		l.opcodeHistory = l.opcodeHistory[1:]
	}
	
	// Check for CALL/RET for recursion detection
	switch opcode {
	case 0xCD: // CALL
		l.callStack = append(l.callStack, pc)
	case 0xC9: // RET
		if len(l.callStack) > 0 {
			l.callStack = l.callStack[:len(l.callStack)-1]
		}
	}
	
	// Look for backward jumps (potential loops)
	if opcode == 0x10 || // DJNZ
		opcode == 0x18 || // JR
		(opcode >= 0x20 && opcode <= 0x38 && (opcode&0x07) == 0) { // JR cc
		l.checkForLoop(pc)
	}
}

// checkForLoop checks if we've found a loop pattern
func (l *LoopDetector) checkForLoop(jumpPC uint16) {
	// Look for previous occurrence of this PC
	for i := len(l.pcHistory) - 2; i >= 0; i-- {
		if l.pcHistory[i] == jumpPC {
			// Found potential loop
			loopLength := len(l.pcHistory) - 1 - i
			if loopLength > 2 && loopLength < 100 {
				// Extract pattern
				pattern := l.opcodeHistory[i : len(l.opcodeHistory)-1]
				hash := hashPattern(pattern)
				
				// Check if this loop already exists
				found := false
				for j := range l.loops {
					if l.loops[j].Hash == hash {
						l.loops[j].Count++
						found = true
						break
					}
				}
				
				if !found {
					loop := Loop{
						StartPC: l.pcHistory[i],
						EndPC:   jumpPC,
						Length:  loopLength,
						Count:   1,
						Hash:    hash,
						Pattern: pattern,
					}
					l.loops = append(l.loops, loop)
				}
			}
			break
		}
	}
}

// HasLoop returns true if loops were detected
func (l *LoopDetector) HasLoop() bool {
	return len(l.loops) > 0
}

// HasRecursion returns true if recursion was detected
func (l *LoopDetector) HasRecursion() bool {
	// Check for repeated addresses in call stack
	seen := make(map[uint16]bool)
	for _, pc := range l.callStack {
		if seen[pc] {
			return true
		}
		seen[pc] = true
	}
	return false
}

// GetLongestLoop returns the most significant loop
func (l *LoopDetector) GetLongestLoop() *Loop {
	if len(l.loops) == 0 {
		return nil
	}
	
	// Find loop with highest count * length
	best := &l.loops[0]
	bestScore := best.Count * best.Length
	
	for i := 1; i < len(l.loops); i++ {
		score := l.loops[i].Count * l.loops[i].Length
		if score > bestScore {
			best = &l.loops[i]
			bestScore = score
		}
	}
	
	return best
}

// Reset clears the detector state
func (l *LoopDetector) Reset() {
	l.pcHistory = l.pcHistory[:0]
	l.opcodeHistory = l.opcodeHistory[:0]
	l.loops = l.loops[:0]
	l.callStack = l.callStack[:0]
}

// hashPattern creates a hash of instruction pattern
func hashPattern(pattern []byte) uint64 {
	// Simple FNV-1a hash
	hash := uint64(14695981039346656037)
	for _, b := range pattern {
		hash ^= uint64(b)
		hash *= 1099511628211
	}
	return hash
}

// PrintReport prints a determinism analysis report
func (d *DeterminismDetector) PrintReport() {
	stats := d.GetStatistics()
	
	fmt.Println("\n🔍 DETERMINISM ANALYSIS REPORT")
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Printf("Total Cycles:        %d\n", stats.TotalCycles)
	fmt.Printf("Deterministic:       %d (%.1f%%)\n", 
		stats.DeterministicCycles, stats.Ratio*100)
	fmt.Printf("Sections Found:      %d\n", stats.SectionCount)
	fmt.Printf("Average Length:      %.0f cycles\n", stats.AverageLength)
	fmt.Printf("Compression Ratio:   %.0fx\n", stats.CompressionPotential)
	
	if len(d.sections) > 0 {
		fmt.Println("\n📊 TOP DETERMINISTIC SECTIONS:")
		fmt.Println("───────────────────────────────────────────────────────")
		
		// Show top 5 longest sections
		for i, section := range d.sections {
			if i >= 5 {
				break
			}
			
			fmt.Printf("%d. Cycles %d-%d (length: %d)\n",
				i+1, section.StartCycle, section.EndCycle, section.Length)
			
			typeStr := "Linear"
			switch section.Type {
			case DetLoop:
				typeStr = fmt.Sprintf("Loop (×%d)", section.LoopCount)
			case DetRecursive:
				typeStr = "Recursive"
			case DetCompute:
				typeStr = "Compute"
			}
			fmt.Printf("   Type: %s\n", typeStr)
			
			if section.Compressible {
				// Calculate compression for this section
				uncompressed := section.Length * 67 // bytes per cycle
				compressed := 16 // Start/end markers
				if section.Type == DetLoop {
					compressed += 8 // Loop parameters
				}
				ratio := float64(uncompressed) / float64(compressed)
				fmt.Printf("   Compression: %.0fx (%.1fKB → %d bytes)\n",
					ratio, float64(uncompressed)/1024, compressed)
			}
		}
	}
	
	fmt.Println("\n💡 OPTIMIZATION OPPORTUNITIES:")
	fmt.Println("───────────────────────────────────────────────────────")
	
	if stats.Ratio > 0.8 {
		fmt.Println("✅ Highly deterministic! Perfect for compression")
		fmt.Println("   Recommendation: Use event-only recording")
	} else if stats.Ratio > 0.5 {
		fmt.Println("⚠️  Moderately deterministic")
		fmt.Println("   Recommendation: Use hybrid recording with keyframes")
	} else {
		fmt.Println("❌ Low determinism - lots of I/O or interrupts")
		fmt.Println("   Recommendation: Use frequent snapshots")
	}
	
	// Memory savings estimate
	originalSize := stats.TotalCycles * 67 // Full state per cycle
	compressedSize := stats.TotalCycles - stats.DeterministicCycles*66 // Only events in deterministic sections
	savings := originalSize - compressedSize
	
	fmt.Printf("\n💾 ESTIMATED MEMORY SAVINGS: %.1f MB\n", 
		float64(savings)/1024/1024)
	fmt.Printf("   Original:   %.1f MB\n", float64(originalSize)/1024/1024)
	fmt.Printf("   Compressed: %.1f MB\n", float64(compressedSize)/1024/1024)
	fmt.Printf("   Reduction:  %.1f%%\n", float64(savings)/float64(originalSize)*100)
}