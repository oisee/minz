package optimizer

import (
	"fmt"
	"regexp"
	"strings"
)

// AssemblyPeepholePattern represents a low-level assembly optimization pattern
type AssemblyPeepholePattern struct {
	Name        string
	Description string
	Pattern     *regexp.Regexp
	Replacement string
	Condition   func([]string) bool // Optional condition function
}

// AssemblyPeepholePass performs peephole optimization on assembly code
type AssemblyPeepholePass struct {
	patterns           []AssemblyPeepholePattern
	optimizationsCount int
}

// NewAssemblyPeepholePass creates a new assembly peephole pass
func NewAssemblyPeepholePass() *AssemblyPeepholePass {
	return &AssemblyPeepholePass{
		patterns: createAssemblyPeepholePatterns(),
		optimizationsCount: 0,
	}
}

// Name returns the name of this pass
func (p *AssemblyPeepholePass) Name() string {
	return "Assembly Peephole Optimization"
}

// OptimizeAssembly performs peephole optimization on assembly code
func (p *AssemblyPeepholePass) OptimizeAssembly(assembly string) string {
	lines := strings.Split(assembly, "\n")
	optimized := p.optimizeAssemblyLines(lines)
	result := strings.Join(optimized, "\n")
	
	// Add optimization report at the end if optimizations were made
	if p.optimizationsCount > 0 {
		result += fmt.Sprintf("\n\n; Assembly peephole optimization: %d patterns applied", p.optimizationsCount)
	}
	
	return result
}

// createAssemblyPeepholePatterns creates Z80-specific assembly peephole patterns
func createAssemblyPeepholePatterns() []AssemblyPeepholePattern {
	return []AssemblyPeepholePattern{
		
		// Pattern 1: Redundant register moves
		// Note: Go regex doesn't support backreferences like \3, so we need specific patterns
		{
			Name:        "redundant_ld_a_b_elimination",
			Description: "Remove redundant LD A,B / LD B,A",
			Pattern:     regexp.MustCompile(`(?m)^(\s*)LD\s+A,\s*B\s*\n\s*LD\s+B,\s*A$`),
			Replacement: "${1}LD A, B    ; Eliminated redundant LD B,A",
		},
		
		// Pattern 2: Load zero optimization
		{
			Name:        "load_zero_to_xor",
			Description: "Replace LD r, 0 with XOR r, r (smaller and faster)",
			Pattern:     regexp.MustCompile(`(?m)^(\s*)LD\s+(A|B|C|D|E|H|L),\s*0$`),
			Replacement: "${1}XOR $2, $2    ; Optimized: was LD $2, 0",
		},
		
		// Pattern 3: Increment optimization
		{
			Name:        "add_one_to_inc",
			Description: "Replace ADD r, 1 with INC r",
			Pattern:     regexp.MustCompile(`(?m)^(\s*)ADD\s+(A|B|C|D|E|H|L),\s*1$`),
			Replacement: "${1}INC $2      ; Optimized: was ADD $2, 1",
		},
		
		// Pattern 4: Decrement optimization  
		{
			Name:        "sub_one_to_dec",
			Description: "Replace SUB r, 1 with DEC r",
			Pattern:     regexp.MustCompile(`(?m)^(\s*)SUB\s+(A|B|C|D|E|H|L),\s*1$`),
			Replacement: "${1}DEC $2      ; Optimized: was SUB $2, 1",
		},
		
		// Pattern 5: Double register optimization
		{
			Name:        "double_add_to_shift",
			Description: "Replace ADD HL, HL with shift operation",
			Pattern:     regexp.MustCompile(`(?m)^(\s*)ADD\s+HL,\s*HL$`),
			Replacement: "${1}ADD HL, HL  ; Double HL (fastest Z80 left shift)",
		},
		
		// Pattern 6: Stack optimization
		{
			Name:        "push_pop_bc_elimination",
			Description: "Remove redundant PUSH BC/POP BC",
			Pattern:     regexp.MustCompile(`(?m)^(\s*)PUSH\s+BC\s*\n\s*POP\s+BC$`),
			Replacement: "${1}; Eliminated redundant PUSH/POP BC",
		},
		{
			Name:        "push_pop_de_elimination",
			Description: "Remove redundant PUSH DE/POP DE",
			Pattern:     regexp.MustCompile(`(?m)^(\s*)PUSH\s+DE\s*\n\s*POP\s+DE$`),
			Replacement: "${1}; Eliminated redundant PUSH/POP DE",
		},
		{
			Name:        "push_pop_hl_elimination",
			Description: "Remove redundant PUSH HL/POP HL",
			Pattern:     regexp.MustCompile(`(?m)^(\s*)PUSH\s+HL\s*\n\s*POP\s+HL$`),
			Replacement: "${1}; Eliminated redundant PUSH/POP HL",
		},
		
		// Pattern 7: Jump optimization
		// This would need a custom condition function to check if label matches
		// For now, we'll skip it since Go regex doesn't support backreferences
		
		// Pattern 8: Conditional jump optimization
		{
			Name:        "optimize_conditional_jumps",
			Description: "Optimize conditional jump patterns",
			Pattern:     regexp.MustCompile(`(?m)^(\s*)OR\s+A\n\s*JP\s+Z,\s*(\w+)$`),
			Replacement: "${1}OR A\n${1}JP Z, $2   ; Test for zero",
		},
		
		// Pattern 9: Register pair loading
		{
			Name:        "combine_register_pair_loads",
			Description: "Combine separate H,L loads into HL load when possible",
			Pattern:     regexp.MustCompile(`(?m)^(\s*)LD\s+H,\s*([0-9A-Fa-f]+)\n\s*LD\s+L,\s*([0-9A-Fa-f]+)$`),
			Replacement: "${1}; Could optimize: LD H,$2 / LD L,$3\n${1}LD H, $2\n${1}LD L, $3",
		},
		
		// Pattern 10: Memory access optimization
		{
			Name:        "optimize_memory_access",
			Description: "Optimize repeated memory access patterns",
			Pattern:     regexp.MustCompile(`(?m)^(\s*)LD\s+HL,\s*\((\w+)\)\s*\n\s*LD\s+HL,\s*\((\w+)\)$`),
			Replacement: "${1}LD HL, ($2)  ; Check if $2 == $3 for redundancy",
		},
		
		// Pattern 11: Remove redundant EX after LD L,E; LD H,D
		{
			Name:        "remove_redundant_ex_after_de_to_hl_copy",
			Description: "Remove EX DE,HL after LD L,E; LD H,D (redundant swap after copy)",
			Pattern:     regexp.MustCompile(`(?m)^(\s*)LD\s+L,\s*E\s*\n\s*LD\s+H,\s*D\s*\n\s*EX\s+DE,\s*HL`),
			Replacement: "${1}LD L, E\n${1}LD H, D    ; Removed redundant EX DE,HL after copy",
		},
		
		// Pattern 12: Remove redundant EX after LD H,D; LD L,E
		{
			Name:        "remove_redundant_ex_after_de_to_hl_copy_reverse",
			Description: "Remove EX DE,HL after LD H,D; LD L,E (redundant swap after copy)",
			Pattern:     regexp.MustCompile(`(?m)^(\s*)LD\s+H,\s*D\s*\n\s*LD\s+L,\s*E\s*\n\s*EX\s+DE,\s*HL`),
			Replacement: "${1}LD H, D\n${1}LD L, E    ; Removed redundant EX DE,HL after copy",
		},
		
		// Pattern 13: Optimize LD D,H; LD E,L; EX DE,HL to nothing (cancels out)
		{
			Name:        "eliminate_redundant_de_hl_swap",
			Description: "Remove LD D,H; LD E,L; EX DE,HL sequence",
			Pattern:     regexp.MustCompile(`(?m)^(\s*)LD\s+D,\s*H\s*\n\s*LD\s+E,\s*L\s*\n\s*EX\s+DE,\s*HL`),
			Replacement: "${1}; Eliminated redundant swap: LD D,H / LD E,L / EX DE,HL",
		},
		
		// Pattern 14: Optimize LD E,L; LD D,H; EX DE,HL to nothing
		{
			Name:        "eliminate_redundant_de_hl_swap_reverse",
			Description: "Remove LD E,L; LD D,H; EX DE,HL sequence",
			Pattern:     regexp.MustCompile(`(?m)^(\s*)LD\s+E,\s*L\n\s*LD\s+D,\s*H\n\s*EX\s+DE,\s*HL$`),
			Replacement: "${1}; Eliminated redundant swap: LD E,L / LD D,H / EX DE,HL",
		},
		
		// Pattern 15: Optimize double EX DE,HL
		{
			Name:        "eliminate_double_ex_de_hl",
			Description: "Remove double EX DE,HL which cancels out",
			Pattern:     regexp.MustCompile(`(?m)^(\s*)EX\s+DE,\s*HL\n\s*EX\s+DE,\s*HL$`),
			Replacement: "${1}; Eliminated double EX DE,HL",
		},
	}
}

// optimizeAssemblyLines applies peephole patterns to assembly lines
func (p *AssemblyPeepholePass) optimizeAssemblyLines(lines []string) []string {
	assembly := strings.Join(lines, "\n")
	
	// Apply each pattern multiple times until no more changes
	changed := true
	iterations := 0
	maxIterations := 5
	
	for changed && iterations < maxIterations {
		changed = false
		iterations++
		
		for _, pattern := range p.patterns {
			oldAssembly := assembly
			
			// Apply the pattern
			assembly = pattern.Pattern.ReplaceAllString(assembly, pattern.Replacement)
			
			if assembly != oldAssembly {
				changed = true
				p.optimizationsCount++
				if debug := false; debug {
					fmt.Printf("Applied pattern: %s\n", pattern.Name)
				}
			}
		}
	}
	
	return strings.Split(assembly, "\n")
}

// Additional Z80-specific optimizations that could be added:

// optimizeZ80Specific performs Z80-specific optimizations
func (p *AssemblyPeepholePass) optimizeZ80Specific(lines []string) []string {
	// Could add:
	// - Shadow register usage optimization
	// - Block transfer instruction usage (LDIR, LDDR)
	// - Bit manipulation optimizations
	// - Interrupt handling optimizations
	// - Relative jump vs absolute jump decisions
	// - Register exchange optimizations (EX DE,HL)
	
	return lines
}

// optimizeForSize performs size-oriented optimizations
func (p *AssemblyPeepholePass) optimizeForSize(lines []string) []string {
	// Could add:
	// - Choose shortest instruction variants
	// - Use relative jumps when possible
	// - Optimize immediate values
	
	return lines
}

// optimizeForSpeed performs speed-oriented optimizations  
func (p *AssemblyPeepholePass) optimizeForSpeed(lines []string) []string {
	// Could add:
	// - Choose fastest instruction variants
	// - Minimize memory accesses
	// - Optimize register usage for T-state minimization
	
	return lines
}