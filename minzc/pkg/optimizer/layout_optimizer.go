package optimizer

import (
	"fmt"
	"sort"
	
	"github.com/minz/minzc/pkg/ir"
	"github.com/minz/minzc/pkg/tas"
)

// LayoutOptimizer optimizes code placement based on profile and platform
type LayoutOptimizer struct {
	profile  *tas.ProfileMetrics
	target   string
	debug    bool
}

// MemoryRegion represents a memory region with performance characteristics
type MemoryRegion struct {
	Start       uint16
	End         uint16
	Name        string
	Contended   bool    // ZX Spectrum: ULA contention
	FastAccess  bool    // CP/M: Page zero
	Priority    int     // Higher = better for hot code
}

// PlatformMemoryMap defines memory regions for each platform
var PlatformMemoryMaps = map[string][]MemoryRegion{
	"zxspectrum": {
		{0x4000, 0x7FFF, "Screen/Contended", true, false, 0},
		{0x8000, 0xBFFF, "Uncontended High", false, false, 100},
		{0xC000, 0xFFFF, "Uncontended Top", false, false, 90},
	},
	"spectrum": { // Alias
		{0x4000, 0x7FFF, "Screen/Contended", true, false, 0},
		{0x8000, 0xBFFF, "Uncontended High", false, false, 100},
		{0xC000, 0xFFFF, "Uncontended Top", false, false, 90},
	},
	// Spectrum clones with no ULA contention - full speed everywhere!
	"pentagon": { // Pentagon 128/512/1024 - no wait states
		{0x4000, 0x7FFF, "Screen (No Wait)", false, false, 100},
		{0x8000, 0xBFFF, "RAM High", false, false, 100},
		{0xC000, 0xFFFF, "RAM Top", false, false, 100},
	},
	"scorpion": { // Scorpion ZS-256 - turbo clone
		{0x0000, 0x3FFF, "ROM", true, false, 0},
		{0x4000, 0x7FFF, "Screen (Turbo)", false, false, 100},
		{0x8000, 0xBFFF, "RAM High", false, false, 100},
		{0xC000, 0xFFFF, "RAM Top", false, false, 100},
	},
	"kay": { // Kay-1024 - no contention
		{0x4000, 0x7FFF, "Screen (Fast)", false, false, 100},
		{0x8000, 0xBFFF, "RAM High", false, false, 100},
		{0xC000, 0xFFFF, "RAM Top", false, false, 100},
	},
	"profi": { // Profi - Advanced clone with turbo modes
		{0x4000, 0x7FFF, "Screen (Turbo)", false, false, 100},
		{0x8000, 0xBFFF, "RAM High", false, false, 100},
		{0xC000, 0xFFFF, "RAM Top", false, false, 100},
	},
	"atm": { // ATM Turbo 1/2 - Enhanced clone
		{0x4000, 0x7FFF, "Screen (No Wait)", false, false, 100},
		{0x8000, 0xBFFF, "RAM High", false, false, 100},
		{0xC000, 0xFFFF, "RAM Top", false, false, 100},
	},
	"timex": { // Timex TC2048/2068 - No contention in turbo mode
		{0x4000, 0x7FFF, "Screen/DOCK", false, false, 95},
		{0x8000, 0xBFFF, "RAM High", false, false, 100},
		{0xC000, 0xFFFF, "RAM/EXROM", false, false, 90},
	},
	"sam": { // SAM Coupé - Z80B at 6MHz, minimal contention
		{0x0000, 0x3FFF, "ROM", true, false, 0},
		{0x4000, 0x7FFF, "Page A", false, false, 95},
		{0x8000, 0xBFFF, "Page B", false, false, 100},
		{0xC000, 0xFFFF, "Page C/D", false, false, 90},
	},
	"cpm": {
		{0x0000, 0x00FF, "Page Zero", false, true, 100},
		{0x0100, 0x7FFF, "TPA Low", false, false, 80},
		{0x8000, 0xFFFF, "TPA High", false, false, 70},
	},
	"msx": {
		{0x0000, 0x3FFF, "ROM Slot", true, false, 0},
		{0x4000, 0x7FFF, "Page 1", false, false, 90},
		{0x8000, 0xBFFF, "Page 2", false, false, 100},
		{0xC000, 0xFFFF, "Page 3", false, false, 80},
	},
	"amstrad": {
		{0x0000, 0x3FFF, "ROM", true, false, 0},
		{0x4000, 0x7FFF, "Screen RAM", true, false, 20},
		{0x8000, 0xBFFF, "RAM Bank", false, false, 100},
		{0xC000, 0xFFFF, "RAM Top", false, false, 90},
	},
}

// NewLayoutOptimizer creates a new layout optimizer
func NewLayoutOptimizer(profile *tas.ProfileMetrics, target string, debug bool) *LayoutOptimizer {
	return &LayoutOptimizer{
		profile: profile,
		target:  target,
		debug:   debug,
	}
}

// OptimizeLayout reorders functions for optimal memory placement
func (l *LayoutOptimizer) OptimizeLayout(module *ir.Module) {
	if l.profile == nil {
		return
	}
	
	// Get memory map for target platform
	memMap := l.getMemoryMap()
	if len(memMap) == 0 {
		if l.debug {
			fmt.Printf("Layout: No memory map for target '%s', using default\n", l.target)
		}
		return
	}
	
	// Score and sort functions by hotness
	functions := l.scoreFunctions(module.Functions)
	
	// Assign functions to memory regions
	l.assignToRegions(functions, memMap)
	
	// Add layout hints to functions
	l.addLayoutHints(functions)
	
	// Report optimization decisions
	if l.debug {
		l.reportLayout(functions)
	}
}

// FunctionScore holds a function with its profile score
type FunctionScore struct {
	Function *ir.Function
	Score    float64
	Size     int
	Region   *MemoryRegion
}

// scoreFunctions calculates hotness scores for all functions
func (l *LayoutOptimizer) scoreFunctions(functions []*ir.Function) []*FunctionScore {
	scores := make([]*FunctionScore, 0, len(functions))
	
	for _, fn := range functions {
		score := &FunctionScore{
			Function: fn,
			Size:     l.estimateFunctionSize(fn),
		}
		
		// Calculate score based on profile data
		if l.profile != nil {
			// Average frequency of all instructions in function
			var totalFreq float64
			var count int
			
			for i := range fn.Instructions {
				// Estimate PC for instruction (simplified)
				estimatedPC := uint16(0x8000 + i*2)
				if freq, exists := l.profile.BlockFrequency[estimatedPC]; exists {
					totalFreq += freq
					count++
				}
			}
			
			if count > 0 {
				score.Score = totalFreq / float64(count)
			}
			
			// Boost score for functions in hot loops
			for i := range fn.Instructions {
				estimatedPC := uint16(0x8000 + i*2)
				if depth := l.profile.LoopDepth[estimatedPC]; depth > 0 {
					score.Score *= float64(1 + depth) // Exponential boost for nested loops
				}
			}
		}
		
		scores = append(scores, score)
	}
	
	// Sort by score (hottest first)
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].Score > scores[j].Score
	})
	
	return scores
}

// estimateFunctionSize estimates code size in bytes
func (l *LayoutOptimizer) estimateFunctionSize(fn *ir.Function) int {
	// Rough estimate: 3 bytes per IR instruction
	size := len(fn.Instructions) * 3
	
	// Add space for prologue/epilogue
	size += 6 // PUSH/POP pairs
	
	// Add space for local variables
	size += len(fn.Locals) * 2
	
	return size
}

// assignToRegions assigns functions to optimal memory regions
func (l *LayoutOptimizer) assignToRegions(functions []*FunctionScore, regions []MemoryRegion) {
	// Sort regions by priority (best first)
	sort.Slice(regions, func(i, j int) bool {
		return regions[i].Priority > regions[j].Priority
	})
	
	// Assign hot functions to best regions
	regionIndex := 0
	currentOffset := regions[regionIndex].Start
	
	for _, fn := range functions {
		// Find a region that can fit this function
		for regionIndex < len(regions) {
			region := &regions[regionIndex]
			
			// Skip contended regions for hot code
			if region.Contended && fn.Score > 0.5 {
				regionIndex++
				if regionIndex < len(regions) {
					currentOffset = regions[regionIndex].Start
				}
				continue
			}
			
			// Check if function fits in current region
			fnEnd := currentOffset + uint16(fn.Size)
			if fnEnd <= region.End {
				fn.Region = region
				currentOffset = fnEnd + 16 // Align to 16-byte boundary
				break
			}
			
			// Move to next region
			regionIndex++
			if regionIndex < len(regions) {
				currentOffset = regions[regionIndex].Start
			} else {
				// Out of regions - use last one
				fn.Region = &regions[len(regions)-1]
				break
			}
		}
	}
}

// addLayoutHints adds platform-specific hints to IR instructions
func (l *LayoutOptimizer) addLayoutHints(functions []*FunctionScore) {
	for _, fnScore := range functions {
		fn := fnScore.Function
		region := fnScore.Region
		
		if region == nil {
			continue
		}
		
		// Add function-level comment
		hint := fmt.Sprintf("Layout: Place in %s [0x%04X-0x%04X]", 
			region.Name, region.Start, region.End)
		
		// Add performance hints based on region
		if region.Contended {
			hint += " (WARNING: Contended memory - may be slow)"
		} else if region.FastAccess {
			hint += " (FAST: Page zero access)"
		} else if region.Priority > 90 {
			hint += " (OPTIMAL: Uncontended high-speed memory)"
		}
		
		// Apply hint to first instruction
		if len(fn.Instructions) > 0 {
			inst := &fn.Instructions[0]
			if inst.Comment != "" {
				inst.Comment = hint + "; " + inst.Comment
			} else {
				inst.Comment = hint
			}
		}
		
		// Mark hot loops for special optimization
		if fnScore.Score > 0.7 {
			for i := range fn.Instructions {
				estimatedPC := uint16(0x8000 + i*2)
				
				if isLoop, depth, isHot := l.profile.GetLoopInfo(estimatedPC); isLoop && isHot {
					inst := &fn.Instructions[i]
					loopHint := fmt.Sprintf("[HOT LOOP: depth=%d, unroll candidate]", depth)
					if inst.Comment != "" {
						inst.Comment += " " + loopHint
					} else {
						inst.Comment = loopHint
					}
				}
			}
		}
	}
}

// getMemoryMap returns the memory map for the current target
func (l *LayoutOptimizer) getMemoryMap() []MemoryRegion {
	if regions, exists := PlatformMemoryMaps[l.target]; exists {
		// Return a copy to avoid modifying the original
		result := make([]MemoryRegion, len(regions))
		copy(result, regions)
		return result
	}
	
	// Default memory map for unknown platforms
	return []MemoryRegion{
		{0x8000, 0xFFFF, "Default RAM", false, false, 50},
	}
}

// reportLayout prints layout optimization decisions
func (l *LayoutOptimizer) reportLayout(functions []*FunctionScore) {
	fmt.Println("\n=== CODE LAYOUT OPTIMIZATION ===")
	fmt.Printf("Target Platform: %s\n", l.target)
	fmt.Println("Function Placement Decisions:")
	fmt.Println("────────────────────────────────────")
	
	for _, fn := range functions {
		classification := "cold"
		if fn.Score > 0.7 {
			classification = "HOT"
		} else if fn.Score > 0.3 {
			classification = "warm"
		}
		
		regionName := "unassigned"
		if fn.Region != nil {
			regionName = fn.Region.Name
		}
		
		fmt.Printf("  %-30s [%4s] Score: %.2f → %s\n",
			fn.Function.Name, classification, fn.Score, regionName)
	}
	
	// Platform-specific advice
	switch l.target {
	case "zxspectrum", "spectrum":
		fmt.Println("\n💡 ZX Spectrum Optimization Tips:")
		fmt.Println("  • Hot code placed in uncontended memory (0x8000+)")
		fmt.Println("  • Screen/attribute access kept in contended region")
		fmt.Println("  • Each contended access costs 3-4 extra T-states!")
		
	case "pentagon", "scorpion", "kay", "profi", "atm":
		fmt.Println("\n🚀 Spectrum Clone (Turbo) Optimization Tips:")
		fmt.Println("  • NO CONTENTION - Full speed everywhere!")
		fmt.Println("  • Screen memory at 0x4000 runs at full speed")
		fmt.Println("  • No need to avoid 0x4000-0x7FFF region")
		fmt.Println("  • 30-50% faster than original Spectrum!")
		
	case "timex":
		fmt.Println("\n💡 Timex TC2048/2068 Optimization Tips:")
		fmt.Println("  • Extended memory via DOCK interface")
		fmt.Println("  • Hi-res graphics modes available")
		fmt.Println("  • Minimal contention in turbo mode")
		
	case "sam":
		fmt.Println("\n💡 SAM Coupé Optimization Tips:")
		fmt.Println("  • Z80B runs at 6MHz (1.7x faster)")
		fmt.Println("  • 512KB RAM with flexible paging")
		fmt.Println("  • Advanced graphics with minimal contention")
		
	case "cpm":
		fmt.Println("\n💡 CP/M Optimization Tips:")
		fmt.Println("  • Critical variables can use page zero (0x00-0xFF)")
		fmt.Println("  • RST vectors available for hot function calls")
		fmt.Println("  • TPA starts at 0x0100 - plenty of RAM available")
		
	case "msx":
		fmt.Println("\n💡 MSX Optimization Tips:")
		fmt.Println("  • Page 2 (0x8000-0xBFFF) is fastest for code")
		fmt.Println("  • Slot switching overhead should be minimized")
		fmt.Println("  • VRAM access through ports - no contention")
		
	case "amstrad":
		fmt.Println("\n💡 Amstrad CPC Optimization Tips:")
		fmt.Println("  • Screen RAM has wait states")
		fmt.Println("  • Use 0x8000+ for time-critical code")
		fmt.Println("  • Gate Array manages memory contention")
	}
	
	fmt.Println("════════════════════════════════════")
}

// OptimizeIntraFunction optimizes instruction layout within a function
func (l *LayoutOptimizer) OptimizeIntraFunction(fn *ir.Function) {
	if l.profile == nil {
		return
	}
	
	// Group basic blocks by execution frequency
	blocks := l.identifyBasicBlocks(fn)
	
	// Sort blocks by frequency (hot blocks first)
	sort.Slice(blocks, func(i, j int) bool {
		return blocks[i].Frequency > blocks[j].Frequency
	})
	
	// Reorder instructions to place hot blocks together
	newInstructions := make([]ir.Instruction, 0, len(fn.Instructions))
	usedInstructions := make(map[int]bool)
	
	// First pass: Add hot blocks
	for _, block := range blocks {
		if block.Frequency > 0.5 { // Hot threshold
			for i := block.Start; i <= block.End; i++ {
				if !usedInstructions[i] {
					newInstructions = append(newInstructions, fn.Instructions[i])
					usedInstructions[i] = true
				}
			}
		}
	}
	
	// Second pass: Add remaining instructions
	for i, inst := range fn.Instructions {
		if !usedInstructions[i] {
			newInstructions = append(newInstructions, inst)
		}
	}
	
	// Replace function instructions
	fn.Instructions = newInstructions
}

// BasicBlock represents a sequence of instructions without branches
type BasicBlock struct {
	Start     int
	End       int
	Frequency float64
}

// identifyBasicBlocks finds basic blocks in a function
func (l *LayoutOptimizer) identifyBasicBlocks(fn *ir.Function) []BasicBlock {
	blocks := make([]BasicBlock, 0)
	
	if len(fn.Instructions) == 0 {
		return blocks
	}
	
	currentBlock := BasicBlock{Start: 0}
	
	for i, inst := range fn.Instructions {
		// Check if this instruction ends a basic block
		isBlockEnd := false
		
		switch inst.Op {
		case ir.OpJump, ir.OpJumpIf, ir.OpJumpIfNot, ir.OpReturn:
			isBlockEnd = true
		case ir.OpCall:
			// Calls don't necessarily end blocks unless they don't return
			isBlockEnd = false
		}
		
		// Check if next instruction is a jump target (starts new block)
		isBlockStart := false
		if i < len(fn.Instructions)-1 {
			nextInst := fn.Instructions[i+1]
			if nextInst.Op == ir.OpLabel {
				isBlockStart = true
			}
		}
		
		if isBlockEnd || isBlockStart || i == len(fn.Instructions)-1 {
			currentBlock.End = i
			
			// Calculate block frequency
			var totalFreq float64
			var count int
			for j := currentBlock.Start; j <= currentBlock.End; j++ {
				estimatedPC := uint16(0x8000 + j*2)
				if freq, exists := l.profile.BlockFrequency[estimatedPC]; exists {
					totalFreq += freq
					count++
				}
			}
			if count > 0 {
				currentBlock.Frequency = totalFreq / float64(count)
			}
			
			blocks = append(blocks, currentBlock)
			
			// Start new block
			if i < len(fn.Instructions)-1 {
				currentBlock = BasicBlock{Start: i + 1}
			}
		}
	}
	
	return blocks
}