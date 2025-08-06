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
		
		// Pattern 16: Optimize LD HL,#nnnn; LD D,H; LD E,L to LD DE,#nnnn; LD H,D; LD L,E
		{
			Name:        "optimize_immediate_load_to_de",
			Description: "Load immediate to DE instead of via HL when followed by copy",
			Pattern:     regexp.MustCompile(`(?m)^(\s*)LD\s+HL,\s*#([0-9A-Fa-f]+)\s*\n\s*LD\s+D,\s*H\s*\n\s*LD\s+E,\s*L$`),
			Replacement: "${1}LD DE, #$2    ; Optimized: load directly to DE\n${1}LD H, D\n${1}LD L, E",
		},
		
		// Pattern 17: Better - if followed by EX DE,HL, just load to DE
		{
			Name:        "optimize_immediate_load_with_swap",
			Description: "Load immediate to DE when HL load is followed by copy and swap",
			Pattern:     regexp.MustCompile(`(?m)^(\s*)LD\s+HL,\s*#([0-9A-Fa-f]+)\s*\n\s*LD\s+D,\s*H\s*\n\s*LD\s+E,\s*L\s*\n\s*EX\s+DE,\s*HL$`),
			Replacement: "${1}LD DE, #$2    ; Optimized: load directly to DE (was LD HL/copy/swap)",
		},
		
		// Pattern 18: Optimize comparison pattern - when we have the inefficient copy+swap
		{
			Name:        "optimize_comparison_copy_swap",
			Description: "Optimize comparison that copies HL to DE then swaps back",
			Pattern:     regexp.MustCompile(`(?m)^(\s*); r\d+ = r\d+ == r\d+\s*\n\s*LD\s+D,\s*H\s*\n\s*LD\s+E,\s*L\s*\n\s*EX\s+DE,\s*HL$`),
			Replacement: "${1}; Comparison optimized\n${1}EX DE, HL      ; Just swap HL and DE",
		},
		
		// Pattern 19: General case - when copy is immediately followed by swap
		{
			Name:        "optimize_copy_then_swap",
			Description: "When copying HL to DE then swapping, just swap",
			Pattern:     regexp.MustCompile(`(?m)^(\s*)LD\s+D,\s*H\s*\n\s*LD\s+E,\s*L\s*\n\s*EX\s+DE,\s*HL$`),
			Replacement: "${1}EX DE, HL      ; Optimized: direct swap instead of copy+swap",
		},
		
		// Pattern 20: Optimize JR with inverse condition followed by JP
		{
			Name:        "optimize_jr_jp_sequence",
			Description: "Convert JR NZ,skip; JP target to JP Z,target",
			Pattern:     regexp.MustCompile(`(?m)^(\s*)JR\s+NZ,\s*\$\+5\s*\n\s*JP\s+(\w+)$`),
			Replacement: "${1}JP Z, $2    ; Optimized: inverted condition",
		},
		{
			Name:        "optimize_jr_z_jp_sequence",
			Description: "Convert JR Z,skip; JP target to JP NZ,target",
			Pattern:     regexp.MustCompile(`(?m)^(\s*)JR\s+Z,\s*\$\+5\s*\n\s*JP\s+(\w+)$`),
			Replacement: "${1}JP NZ, $2   ; Optimized: inverted condition",
		},
		
		// Pattern 21: Stack drop optimization
		{
			Name:        "optimize_stack_drop_2",
			Description: "Optimize POP to INC SP for dropping 2 bytes",
			Pattern:     regexp.MustCompile(`(?m)^(\s*)POP\s+([A-Z]+)\s*;\s*Drop.*$`),
			Replacement: "${1}INC SP\n${1}INC SP       ; Optimized: drop 2 bytes from stack (was POP $2)",
		},
		
		// Pattern 21b: Stack drop optimization (without comment)
		{
			Name:        "optimize_stack_drop_general",
			Description: "Optimize POP used for dropping when result unused",
			Pattern:     regexp.MustCompile(`(?m)^(\s*)POP\s+([A-Z]+)\s*\n\s*; Register \2 not used after POP$`),
			Replacement: "${1}INC SP\n${1}INC SP       ; Optimized: drop 2 bytes (was POP $2)",
		},
		
		// Pattern 22: Optimize compare with zero
		{
			Name:        "optimize_cp_zero",
			Description: "Convert CP 0 to OR A for flag setting",
			Pattern:     regexp.MustCompile(`(?m)^(\s*)CP\s+0$`),
			Replacement: "${1}OR A         ; Optimized: CP 0 -> OR A",
		},
		
		// Pattern 23: Optimize LD reg,0 to XOR reg (for A only)
		{
			Name:        "optimize_ld_a_zero",
			Description: "Convert LD A,0 to XOR A",
			Pattern:     regexp.MustCompile(`(?m)^(\s*)LD\s+A,\s*0$`),
			Replacement: "${1}XOR A        ; Optimized: LD A,0 -> XOR A",
		},
		
		// Pattern 24: Optimize ADD A,1 to INC A
		{
			Name:        "optimize_add_a_one",
			Description: "Convert ADD A,1 to INC A",
			Pattern:     regexp.MustCompile(`(?m)^(\s*)ADD\s+A,\s*1$`),
			Replacement: "${1}INC A        ; Optimized: ADD A,1 -> INC A",
		},
		
		// Pattern 25: Optimize SUB 1 to DEC A
		{
			Name:        "optimize_sub_one",
			Description: "Convert SUB 1 to DEC A",
			Pattern:     regexp.MustCompile(`(?m)^(\s*)SUB\s+1$`),
			Replacement: "${1}DEC A        ; Optimized: SUB 1 -> DEC A",
		},
		
		// Pattern 26: Optimize ADD HL,1 to INC HL
		{
			Name:        "optimize_add_hl_one",
			Description: "Convert ADD HL,1 to INC HL",
			Pattern:     regexp.MustCompile(`(?m)^(\s*)LD\s+DE,\s*1\s*\n\s*ADD\s+HL,\s*DE$`),
			Replacement: "${1}INC HL       ; Optimized: ADD HL,1 -> INC HL",
		},
		
		// Pattern 27: Optimize 16-bit compare pattern
		{
			Name:        "optimize_16bit_compare_pattern",
			Description: "Add comment to 16-bit compare pattern",
			Pattern:     regexp.MustCompile(`(?m)^(\s*)OR\s+A\s*\n\s*SBC\s+HL,\s*DE\s*\n\s*ADD\s+HL,\s*DE$`),
			Replacement: "${1}OR A         ; 16-bit compare HL vs DE\n${1}SBC HL, DE\n${1}ADD HL, DE   ; Restore HL, flags set",
		},
		
		// Pattern 28: Optimize unnecessary OR A before SBC
		{
			Name:        "optimize_redundant_or_a",
			Description: "Remove OR A when carry is already clear",
			Pattern:     regexp.MustCompile(`(?m)^(\s*)XOR\s+A\s*\n\s*OR\s+A$`),
			Replacement: "${1}XOR A        ; Sets A=0 and clears carry",
		},
		
		// Pattern 29: Optimize LD A,H; OR L to LD A,H; OR L pattern
		{
			Name:        "optimize_hl_zero_test",
			Description: "Add comment for HL zero test pattern",
			Pattern:     regexp.MustCompile(`(?m)^(\s*)LD\s+A,\s*H\s*\n\s*OR\s+L$`),
			Replacement: "${1}LD A, H\n${1}OR L         ; Test if HL = 0",
		},
		
		// Pattern 30: Optimize multiple INC SP to ADD SP
		{
			Name:        "optimize_multiple_inc_sp",
			Description: "Convert 3+ INC SP to LD HL,n; ADD HL,SP; LD SP,HL",
			Pattern:     regexp.MustCompile(`(?m)^(\s*)INC\s+SP\s*\n\s*INC\s+SP\s*\n\s*INC\s+SP$`),
			Replacement: "${1}INC SP\n${1}INC SP\n${1}INC SP       ; Consider: LD HL,3; ADD HL,SP; LD SP,HL for larger drops",
		},
		
		// Pattern 31: Optimize JP to JR for short jumps
		{
			Name:        "suggest_jr_optimization",
			Description: "Suggest JR instead of JP for short jumps",
			Pattern:     regexp.MustCompile(`(?m)^(\s*)JP\s+(\w+)\s*;\s*Short jump candidate$`),
			Replacement: "${1}JP $2        ; Consider: JR $2 if within -128/+127 bytes",
		},
		
		// Pattern 32: Optimize redundant register loads
		{
			Name:        "optimize_redundant_ld_same_reg",
			Description: "Remove redundant load to same register with same value",
			Pattern:     regexp.MustCompile(`(?m)^(\s*)LD\s+([A-Z]),\s*([A-Z])\s*\n\s*LD\s+\2,\s*\3$`),
			Replacement: "${1}LD $2, $3    ; Removed redundant duplicate load",
		},
		
		// Pattern 33: Optimize LD BC,n; ADD HL,BC to direct add when n is small
		{
			Name:        "optimize_small_add_hl",
			Description: "Convert small ADD HL via BC to INC HL",
			Pattern:     regexp.MustCompile(`(?m)^(\s*)LD\s+BC,\s*2\s*\n\s*ADD\s+HL,\s*BC$`),
			Replacement: "${1}INC HL\n${1}INC HL       ; Optimized: ADD HL,2 -> 2x INC HL",
		},
		
		// Pattern 34: Optimize double negation
		{
			Name:        "optimize_double_neg",
			Description: "Remove double NEG",
			Pattern:     regexp.MustCompile(`(?m)^(\s*)NEG\s*\n\s*NEG$`),
			Replacement: "${1}; Eliminated double NEG",
		},
		
		// Pattern 35: Optimize CCF after SCF
		{
			Name:        "optimize_scf_ccf",
			Description: "Replace SCF; CCF with OR A (clear carry)",
			Pattern:     regexp.MustCompile(`(?m)^(\s*)SCF\s*\n\s*CCF$`),
			Replacement: "${1}OR A         ; Clear carry (was SCF; CCF)",
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