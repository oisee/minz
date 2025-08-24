package optimizer

import (
	"github.com/minz/minzc/pkg/ir"
)

// BasicPGOPass implements Quick Win #3: Simple hot/cold annotation
type BasicPGOPass struct {
	profile map[string]interface{}
}

// NewBasicPGOPass creates a new PGO optimizer with profile data
func NewBasicPGOPass(profile map[string]interface{}) *BasicPGOPass {
	return &BasicPGOPass{
		profile: profile,
	}
}

// AnnotateHotCold adds profile hints to MIR instructions
func (p *BasicPGOPass) AnnotateHotCold(fn *ir.Function) {
	if p.profile == nil {
		return
	}
	
	executions, ok := p.profile["executions"].(map[uint16]uint64)
	if !ok {
		return
	}
	
	threshold, ok := p.profile["hot_threshold"].(uint64)
	if !ok {
		return
	}
	
	// Annotate instructions with profile hints
	for i := range fn.Instructions {
		inst := &fn.Instructions[i]
		
		// For each instruction, estimate its PC address
		// This is simplified - in real implementation we'd need address mapping
		estimatedPC := uint16(0x8000 + i*2) // Rough estimate
		
		if count, exists := executions[estimatedPC]; exists {
			if count > threshold {
				inst.ProfileHint = "hot"
			} else if count == 0 {
				inst.ProfileHint = "cold" 
			} else {
				inst.ProfileHint = "warm"
			}
		}
	}
}

// OptimizeForSpectrum applies ZX Spectrum-specific PGO optimizations
func (p *BasicPGOPass) OptimizeForSpectrum(fn *ir.Function) {
	// Quick Win #4: Platform-aware memory layout hints
	for i := range fn.Instructions {
		inst := &fn.Instructions[i]
		
		if inst.ProfileHint == "hot" {
			// Add comment to suggest uncontended memory placement
			if inst.Comment != "" {
				inst.Comment += " "
			}
			inst.Comment += "[PGO: Place in uncontended memory 0x8000+]"
		} else if inst.ProfileHint == "cold" {
			// Cold code can go in contended memory
			if inst.Comment != "" {
				inst.Comment += " "
			}
			inst.Comment += "[PGO: Can use contended memory 0x4000+]"
		}
	}
}

// OptimizeForCPM applies CP/M-specific PGO optimizations  
func (p *BasicPGOPass) OptimizeForCPM(fn *ir.Function) {
	// Count hot function calls for RST optimization
	hotCallCount := 0
	for i := range fn.Instructions {
		inst := &fn.Instructions[i]
		if inst.Op == ir.OpCall && inst.ProfileHint == "hot" {
			hotCallCount++
		}
	}
	
	// If we have hot calls and RST vectors available, suggest optimization
	if hotCallCount > 0 {
		for i := range fn.Instructions {
			inst := &fn.Instructions[i]
			if inst.Op == ir.OpCall && inst.ProfileHint == "hot" {
				if inst.Comment != "" {
					inst.Comment += " "
				}
				inst.Comment += "[PGO: RST vector candidate - saves 2 bytes, 6 T-states]"
			}
		}
	}
}

// ApplyPlatformOptimizations applies the right optimizations based on target
func (p *BasicPGOPass) ApplyPlatformOptimizations(fn *ir.Function, target string) {
	// First, annotate with hot/cold data
	p.AnnotateHotCold(fn)
	
	// Then apply platform-specific optimizations
	switch target {
	case "spectrum", "zx":
		p.OptimizeForSpectrum(fn)
	case "cpm":
		p.OptimizeForCPM(fn)
	case "agon", "agon-light2":
		// Agon has flexible memory mapping, use RST optimization
		p.OptimizeForCPM(fn) // Same as CP/M for now
	default:
		// Generic Z80 optimizations
		p.OptimizeForCPM(fn)
	}
}