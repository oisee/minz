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
		
		// Pattern 2: Load zero optimization (A register only)
		// NOTE: On Z80, XOR only works with register A - "XOR A" clears A to 0
		// For other registers (B,C,D,E,H,L), we CANNOT use XOR - must keep LD r, 0
		{
			Name:        "load_zero_to_xor_a",
			Description: "Replace LD A, 0 with XOR A (smaller and faster)",
			Pattern:     regexp.MustCompile(`(?m)^(\s*)LD\s+A,\s*0$`),
			Replacement: "${1}XOR A    ; Optimized: was LD A, 0",
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
			Pattern:     regexp.MustCompile(`(?m)^(\s*)POP\s+([A-Z]+)\s*\n\s*; Register [A-Z]+ not used after POP$`),
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
		
		// Pattern 32: Optimize redundant register loads (specific cases)
		{
			Name:        "optimize_redundant_ld_a_b_b",
			Description: "Remove redundant LD A,B followed by LD A,B",
			Pattern:     regexp.MustCompile(`(?m)^(\s*)LD\s+A,\s*B\s*\n\s*LD\s+A,\s*B$`),
			Replacement: "${1}LD A, B    ; Removed redundant duplicate load",
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

		// Pattern 36: LD r,r elimination (LD A,A = NOP)
		{
			Name:        "eliminate_ld_a_a",
			Description: "Remove redundant LD A, A",
			Pattern:     regexp.MustCompile(`(?m)^(\s*)LD\s+A\s*,\s*A\s*(?:;.*)?$`),
			Replacement: "${1}; Eliminated LD A, A (NOP)",
		},
		{
			Name:        "eliminate_ld_b_b",
			Description: "Remove redundant LD B, B",
			Pattern:     regexp.MustCompile(`(?m)^(\s*)LD\s+B\s*,\s*B\s*(?:;.*)?$`),
			Replacement: "${1}; Eliminated LD B, B (NOP)",
		},
		{
			Name:        "eliminate_ld_c_c",
			Description: "Remove redundant LD C, C",
			Pattern:     regexp.MustCompile(`(?m)^(\s*)LD\s+C\s*,\s*C\s*(?:;.*)?$`),
			Replacement: "${1}; Eliminated LD C, C (NOP)",
		},
		{
			Name:        "eliminate_ld_d_d",
			Description: "Remove redundant LD D, D",
			Pattern:     regexp.MustCompile(`(?m)^(\s*)LD\s+D\s*,\s*D\s*(?:;.*)?$`),
			Replacement: "${1}; Eliminated LD D, D (NOP)",
		},
		{
			Name:        "eliminate_ld_e_e",
			Description: "Remove redundant LD E, E",
			Pattern:     regexp.MustCompile(`(?m)^(\s*)LD\s+E\s*,\s*E\s*(?:;.*)?$`),
			Replacement: "${1}; Eliminated LD E, E (NOP)",
		},
		{
			Name:        "eliminate_ld_h_h",
			Description: "Remove redundant LD H, H",
			Pattern:     regexp.MustCompile(`(?m)^(\s*)LD\s+H\s*,\s*H\s*(?:;.*)?$`),
			Replacement: "${1}; Eliminated LD H, H (NOP)",
		},
		{
			Name:        "eliminate_ld_l_l",
			Description: "Remove redundant LD L, L",
			Pattern:     regexp.MustCompile(`(?m)^(\s*)LD\s+L\s*,\s*L\s*(?:;.*)?$`),
			Replacement: "${1}; Eliminated LD L, L (NOP)",
		},

		// Pattern 37: Redundant operations elimination
		{
			Name:        "eliminate_add_a_0",
			Description: "Remove ADD A, 0 (no effect)",
			Pattern:     regexp.MustCompile(`(?m)^(\s*)ADD\s+A\s*,\s*0\s*(?:;.*)?$`),
			Replacement: "${1}; Eliminated ADD A, 0",
		},
		{
			Name:        "eliminate_sub_0",
			Description: "Remove SUB 0 (no effect)",
			Pattern:     regexp.MustCompile(`(?m)^(\s*)SUB\s+0\s*(?:;.*)?$`),
			Replacement: "${1}; Eliminated SUB 0",
		},
		{
			Name:        "eliminate_and_ff",
			Description: "Remove AND $FF (no effect for 8-bit)",
			Pattern:     regexp.MustCompile(`(?m)^(\s*)AND\s+(?:\$FF|0xFF|255)\s*(?:;.*)?$`),
			Replacement: "${1}; Eliminated AND $FF",
		},
		{
			Name:        "eliminate_or_0",
			Description: "Remove OR 0 (no effect)",
			Pattern:     regexp.MustCompile(`(?m)^(\s*)OR\s+0\s*(?:;.*)?$`),
			Replacement: "${1}; Eliminated OR 0",
		},
		{
			Name:        "eliminate_xor_0",
			Description: "Remove XOR 0 (no effect)",
			Pattern:     regexp.MustCompile(`(?m)^(\s*)XOR\s+0\s*(?:;.*)?$`),
			Replacement: "${1}; Eliminated XOR 0",
		},

		// Pattern 38: Double exchange elimination
		{
			Name:        "eliminate_double_exx",
			Description: "Remove double EXX (swap twice = no change)",
			Pattern:     regexp.MustCompile(`(?m)^(\s*)EXX\s*(?:;.*)?\n\s*EXX\s*(?:;.*)?$`),
			Replacement: "${1}; Eliminated double EXX",
		},

		// Pattern 39: Double CCF elimination
		{
			Name:        "eliminate_double_ccf",
			Description: "Remove double CCF (complement carry twice = no change)",
			Pattern:     regexp.MustCompile(`(?m)^(\s*)CCF\s*(?:;.*)?\n\s*CCF\s*(?:;.*)?$`),
			Replacement: "${1}; Eliminated double CCF",
		},

		// Pattern 40: Double CPL elimination
		{
			Name:        "eliminate_double_cpl",
			Description: "Remove double CPL (complement A twice = original)",
			Pattern:     regexp.MustCompile(`(?m)^(\s*)CPL\s*(?:;.*)?\n\s*CPL\s*(?:;.*)?$`),
			Replacement: "${1}; Eliminated double CPL",
		},

		// Pattern 41: INC/DEC cancellation - MOVED TO LINE-BASED (eliminateIncDecPairs)
		// These are handled in optimizeZ80Specific() to check for flag usage
		// INC/DEC affects Z, S, H, P/V flags - must not eliminate if flags are used!

		// Pattern 42: Consecutive idempotent operations
		{
			Name:        "eliminate_consecutive_and_a",
			Description: "Remove consecutive AND A (idempotent)",
			Pattern:     regexp.MustCompile(`(?m)^(\s*)AND\s+A\s*(?:;.*)?\n\s*AND\s+A\s*(?:;.*)?$`),
			Replacement: "${1}AND A        ; Removed duplicate",
		},
		{
			Name:        "eliminate_consecutive_or_a",
			Description: "Remove consecutive OR A (idempotent)",
			Pattern:     regexp.MustCompile(`(?m)^(\s*)OR\s+A\s*(?:;.*)?\n\s*OR\s+A\s*(?:;.*)?$`),
			Replacement: "${1}OR A         ; Removed duplicate",
		},
		{
			Name:        "eliminate_consecutive_scf",
			Description: "Remove consecutive SCF (idempotent)",
			Pattern:     regexp.MustCompile(`(?m)^(\s*)SCF\s*(?:;.*)?\n\s*SCF\s*(?:;.*)?$`),
			Replacement: "${1}SCF          ; Removed duplicate",
		},

		// Pattern 43: CP 0 after flag-setting XOR A (Z flag already set)
		{
			Name:        "eliminate_cp_0_after_xor_a",
			Description: "Remove CP 0 after XOR A (Z flag already set)",
			Pattern:     regexp.MustCompile(`(?m)^(\s*)XOR\s+A\s*(?:;.*)?\n\s*CP\s+0\s*(?:;.*)?$`),
			Replacement: "${1}XOR A        ; CP 0 eliminated (Z flag already set)",
		},

		// Pattern 44: Multiply by 2 optimization (SLA A → ADD A,A - same size but faster)
		// SLA A = 8 T-states (2 bytes), ADD A,A = 4 T-states (1 byte) - BETTER!
		{
			Name:        "sla_a_to_add_a_a",
			Description: "Replace SLA A with ADD A,A (faster and smaller)",
			Pattern:     regexp.MustCompile(`(?m)^(\s*)SLA\s+A\s*(?:;.*)?$`),
			Replacement: "${1}ADD A, A     ; Optimized: was SLA A (faster)",
		},
		{
			Name:        "eliminate_or_a_after_xor_a",
			Description: "Remove OR A after XOR A (Z flag already set, A=0)",
			Pattern:     regexp.MustCompile(`(?m)^(\s*)XOR\s+A\s*(?:;.*)?\n\s*OR\s+A\s*(?:;.*)?$`),
			Replacement: "${1}XOR A        ; OR A eliminated (Z flag already set)",
		},
		{
			Name:        "eliminate_and_a_after_xor_a",
			Description: "Remove AND A after XOR A (Z flag already set, A=0)",
			Pattern:     regexp.MustCompile(`(?m)^(\s*)XOR\s+A\s*(?:;.*)?\n\s*AND\s+A\s*(?:;.*)?$`),
			Replacement: "${1}XOR A        ; AND A eliminated (Z flag already set)",
		},

		// Pattern 45: Canonical register order - alphabetical (A,B,C,D,E,H,L)
		// Reorder LD E,X / LD C,Y to LD C,Y / LD E,X (C before E alphabetically)
		{
			Name:        "canonical_order_e_c_to_c_e",
			Description: "Reorder LD E / LD C to alphabetical LD C / LD E",
			Pattern:     regexp.MustCompile(`(?m)^(\s*)LD\s+E,\s*([^\n]+)\n(\s*)LD\s+C,\s*([^\n]+)$`),
			Replacement: "${3}LD C, $4\n${1}LD E, $2     ; Canonical order (C before E)",
		},

		// Pattern 46: Reorder LD D,X / LD C,Y to LD C / LD D
		{
			Name:        "canonical_order_d_c_to_c_d",
			Description: "Reorder LD D / LD C to alphabetical LD C / LD D",
			Pattern:     regexp.MustCompile(`(?m)^(\s*)LD\s+D,\s*([^\n]+)\n(\s*)LD\s+C,\s*([^\n]+)$`),
			Replacement: "${3}LD C, $4\n${1}LD D, $2     ; Canonical order (C before D)",
		},

		// Pattern 46b: Reorder LD E,X / LD D,Y to LD D / LD E
		{
			Name:        "canonical_order_e_d_to_d_e",
			Description: "Reorder LD E / LD D to alphabetical LD D / LD E",
			Pattern:     regexp.MustCompile(`(?m)^(\s*)LD\s+E,\s*([^\n]+)\n(\s*)LD\s+D,\s*([^\n]+)$`),
			Replacement: "${3}LD D, $4\n${1}LD E, $2     ; Canonical order (D before E)",
		},

		// Pattern 46c: Reorder LD H,X / LD E,Y to LD E / LD H
		{
			Name:        "canonical_order_h_e_to_e_h",
			Description: "Reorder LD H / LD E to alphabetical LD E / LD H",
			Pattern:     regexp.MustCompile(`(?m)^(\s*)LD\s+H,\s*([^\n]+)\n(\s*)LD\s+E,\s*([^\n]+)$`),
			Replacement: "${3}LD E, $4\n${1}LD H, $2     ; Canonical order (E before H)",
		},

		// Pattern 46d: Reorder LD L,X / LD H,Y to LD H / LD L
		{
			Name:        "canonical_order_l_h_to_h_l",
			Description: "Reorder LD L / LD H to alphabetical LD H / LD L",
			Pattern:     regexp.MustCompile(`(?m)^(\s*)LD\s+L,\s*([^\n]+)\n(\s*)LD\s+H,\s*([^\n]+)$`),
			Replacement: "${3}LD H, $4\n${1}LD L, $2     ; Canonical order (H before L)",
		},

		// Pattern 47: Optimize consecutive BDOS putchar calls - detect potential string
		// This is a marker pattern for future string optimization
		{
			Name:        "mark_consecutive_putchar",
			Description: "Mark consecutive LD E,char / LD C,2 / CALL 5 sequences",
			Pattern:     regexp.MustCompile(`(?m)((?:\s*LD\s+E,\s*(?:'.'|\d+)\s*\n\s*LD\s+C,\s*2\s*\n\s*CALL\s+5\s*\n){3,})`),
			Replacement: "; [STRING_INTENT] Consecutive putchar detected\n$1",
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
	
	lines = strings.Split(assembly, "\n")

	// Apply Z80-specific line-based optimizations
	lines = p.optimizeZ80Specific(lines)

	return lines
}

// Additional Z80-specific optimizations that could be added:

// optimizeZ80Specific performs Z80-specific optimizations
func (p *AssemblyPeepholePass) optimizeZ80Specific(lines []string) []string {
	// Optimization: Remove JP to immediately following label
	lines = p.eliminateJumpToNext(lines)

	// Optimization: Convert DEC B + JP/JR NZ to DJNZ
	lines = p.optimizeDJNZ(lines)

	// Optimization: Convert consecutive LD r,0 to XOR A + LD r,A
	lines = p.optimizeConsecutiveLdZero(lines)

	// Optimization: Remove duplicate consecutive XOR A (idempotent)
	lines = p.eliminateDuplicateXorA(lines)

	// Optimization: Register value propagation (INC/DEC instead of LD)
	lines = p.propagateRegisterValues(lines)

	// Optimization: Eliminate INC/DEC pairs (with flag safety check)
	lines = p.eliminateIncDecPairs(lines)

	// Optimization: Eliminate redundant store/load (LD A,n; LD r,A; LD A,r -> LD A,n)
	lines = p.eliminateRedundantStoreLoad(lines)

	return lines
}

// optimizeDJNZ converts DEC B followed by JP/JR NZ,label to DJNZ label
func (p *AssemblyPeepholePass) optimizeDJNZ(lines []string) []string {
	decBPattern := regexp.MustCompile(`^\s*DEC\s+B\s*(?:;.*)?$`)
	jumpNZPattern := regexp.MustCompile(`^\s*J[PR]\s+NZ\s*,\s*(\S+)\s*(?:;.*)?$`)

	result := make([]string, 0, len(lines))

	for i := 0; i < len(lines); i++ {
		line := lines[i]

		// Check if this is a DEC B instruction
		if decBPattern.MatchString(line) {
			// Look for JP/JR NZ on the next non-empty line
			foundJump := false
			for j := i + 1; j < len(lines) && j < i+3; j++ {
				nextLine := strings.TrimSpace(lines[j])

				// Skip empty lines and comments
				if nextLine == "" || strings.HasPrefix(nextLine, ";") {
					continue
				}

				// Check if this is JP/JR NZ
				if matches := jumpNZPattern.FindStringSubmatch(lines[j]); matches != nil {
					targetLabel := matches[1]
					// Replace with DJNZ
					indent := "    "
					if idx := strings.Index(line, "DEC"); idx > 0 {
						indent = line[:idx]
					}
					result = append(result, indent+"DJNZ "+targetLabel+"    ; Optimized: was DEC B + JP/JR NZ")
					p.optimizationsCount++
					foundJump = true
					i = j // Skip the JP/JR instruction
					break
				}

				// If we hit any other instruction, stop looking
				break
			}

			if !foundJump {
				result = append(result, line)
			}
		} else {
			result = append(result, line)
		}
	}

	return result
}

// optimizeConsecutiveLdZero converts consecutive LD r,0 instructions to XOR A + LD r,A
// This saves bytes when clearing multiple registers to zero:
//   Before: LD H,0 / LD D,0 / LD E,0 = 6 bytes
//   After:  XOR A / LD H,A / LD D,A / LD E,A = 4 bytes
// Requires 2+ consecutive LD r,0 (where r is not A) to be worthwhile.
// ADR: docs/294_Consecutive_LD_Zero_Optimization.md
func (p *AssemblyPeepholePass) optimizeConsecutiveLdZero(lines []string) []string {
	// Pattern: LD r, 0 where r is B, C, D, E, H, or L (not A, as XOR A handles that)
	ldZeroPattern := regexp.MustCompile(`^(\s*)LD\s+([BCDEHL])\s*,\s*0\s*(?:;.*)?$`)

	result := make([]string, 0, len(lines))
	i := 0

	for i < len(lines) {
		line := lines[i]

		// Check if this is a LD r, 0 instruction
		match := ldZeroPattern.FindStringSubmatch(line)
		if match == nil {
			result = append(result, line)
			i++
			continue
		}

		// Found first LD r, 0 - collect consecutive ones
		indent := match[1]
		registers := []string{match[2]}
		startIdx := i
		i++

		// Look for more consecutive LD r, 0 instructions
		for i < len(lines) {
			nextLine := lines[i]
			trimmed := strings.TrimSpace(nextLine)

			// Skip empty lines and comments
			if trimmed == "" || strings.HasPrefix(trimmed, ";") {
				i++
				continue
			}

			nextMatch := ldZeroPattern.FindStringSubmatch(nextLine)
			if nextMatch == nil {
				break
			}

			registers = append(registers, nextMatch[2])
			i++
		}

		// Only optimize if we have 2+ registers
		if len(registers) >= 2 {
			// Emit XOR A first
			result = append(result, indent+"XOR A        ; Clear A for multi-register zero init")
			// Then LD r, A for each register
			for _, reg := range registers {
				result = append(result, indent+"LD "+reg+", A    ; Zero via A (was LD "+reg+", 0)")
			}
			p.optimizationsCount++
		} else {
			// Only 1 register - keep original
			for j := startIdx; j < i; j++ {
				result = append(result, lines[j])
			}
		}
	}

	return result
}

// eliminateDuplicateXorA removes redundant XOR A instructions when A is known to be 0
// A is known to be 0 after XOR A, and stays 0 after LD r,A (which doesn't modify A)
// SAFETY: Does NOT remove if the XOR A follows a label (could be SMC patch target)
// or has SMC-related markers in comments.
func (p *AssemblyPeepholePass) eliminateDuplicateXorA(lines []string) []string {
	xorAPattern := regexp.MustCompile(`^(\s*)XOR\s+A\s*(?:;.*)?$`)
	// LD r,A where r is B,C,D,E,H,L - these don't modify A
	ldFromAPattern := regexp.MustCompile(`^\s*LD\s+[BCDEHL]\s*,\s*A\s*(?:;.*)?$`)
	labelPattern := regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_$]*:`)
	smcPattern := regexp.MustCompile(`\$imm|SMC|PATCH`)

	result := make([]string, 0, len(lines))
	aIsZero := false      // Track if A is known to be 0
	afterLabel := false   // Track if we just saw a label

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Track if this is a label - resets our knowledge and marks SMC risk
		isLabel := labelPattern.MatchString(trimmed)
		if isLabel {
			afterLabel = true
			aIsZero = false
			result = append(result, line)
			continue
		}

		// Skip empty lines and comments for tracking purposes
		if trimmed == "" || strings.HasPrefix(trimmed, ";") {
			result = append(result, line)
			continue
		}

		// Check if this is XOR A
		if xorAPattern.MatchString(line) {
			// Check for SMC markers in comment
			hasSMCMarker := smcPattern.MatchString(line)

			// Keep if: A not known to be 0, follows label (SMC), or has SMC marker
			if !aIsZero || afterLabel || hasSMCMarker {
				result = append(result, line)
				aIsZero = true
			} else {
				// Skip redundant XOR A - A is already 0
				p.optimizationsCount++
			}
			afterLabel = false
		} else if ldFromAPattern.MatchString(line) {
			// LD r,A doesn't change A - keep aIsZero state
			result = append(result, line)
			afterLabel = false
		} else {
			// Any other instruction - assume it might change A
			result = append(result, line)
			aIsZero = false
			afterLabel = false
		}
	}

	return result
}

// RegisterState tracks the known value of a register
type RegisterState struct {
	known bool
	value uint8
}

// propagateRegisterValues optimizes LD r,imm using known register values
// - If r already contains imm: eliminate the instruction
// - If r contains imm-1: replace with INC r
// - If r contains imm+1: replace with DEC r
// ADR: docs/295_Register_Value_Propagation.md
func (p *AssemblyPeepholePass) propagateRegisterValues(lines []string) []string {
	// Patterns
	ldImmPattern := regexp.MustCompile(`^(\s*)LD\s+([ABCDEHL])\s*,\s*(?:\$|0x|#)?([0-9A-Fa-f]+)\s*(?:;.*)?$`)
	ldRegPattern := regexp.MustCompile(`^(\s*)LD\s+([ABCDEHL])\s*,\s*([ABCDEHL])\s*(?:;.*)?$`)
	incPattern := regexp.MustCompile(`^\s*INC\s+([ABCDEHL])\s*(?:;.*)?$`)
	decPattern := regexp.MustCompile(`^\s*DEC\s+([ABCDEHL])\s*(?:;.*)?$`)
	xorAPattern := regexp.MustCompile(`^\s*XOR\s+A\s*(?:;.*)?$`)
	labelPattern := regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_$]*:`)
	smcPattern := regexp.MustCompile(`\$imm|SMC|PATCH`)
	// Instructions that invalidate all registers
	invalidateAllPattern := regexp.MustCompile(`^\s*(CALL|JP|JR|RET|RST|HALT|POP|EX|EXX|PUSH|DI|EI|IM|RETI|RETN)`)
	// Instructions that use flags (don't optimize before these)
	flagUsePattern := regexp.MustCompile(`^\s*(JR\s+(N?[ZC])|JP\s+(N?[ZC]|P[OE]|[PM])|CALL\s+(N?[ZC])|RET\s+(N?[ZC])|ADC|SBC|DAA|RLA|RRA|RLCA|RRCA)`)

	// Register state tracking
	regs := make(map[string]*RegisterState)
	for _, r := range []string{"A", "B", "C", "D", "E", "H", "L"} {
		regs[r] = &RegisterState{known: false}
	}

	invalidateAll := func() {
		for _, state := range regs {
			state.known = false
		}
	}

	result := make([]string, 0, len(lines))
	skipNextOptimize := false // Skip optimization for instruction right after label (SMC)

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip empty lines and comments
		if trimmed == "" || strings.HasPrefix(trimmed, ";") {
			result = append(result, line)
			continue
		}

		// Check for label - invalidates tracking and marks next instruction as potential SMC
		if labelPattern.MatchString(trimmed) {
			invalidateAll()
			skipNextOptimize = true
			result = append(result, line)
			continue
		}

		// Check for SMC markers in comments
		hasSMCMarker := smcPattern.MatchString(line)

		// Check for instructions that invalidate all registers
		if invalidateAllPattern.MatchString(trimmed) {
			invalidateAll()
			result = append(result, line)
			skipNextOptimize = false
			continue
		}

		// Check for XOR A - sets A = 0
		if xorAPattern.MatchString(line) {
			regs["A"].known = true
			regs["A"].value = 0
			result = append(result, line)
			skipNextOptimize = false
			continue
		}

		// Check for INC r - increments known value
		if match := incPattern.FindStringSubmatch(line); match != nil {
			reg := strings.ToUpper(match[1])
			if regs[reg].known {
				regs[reg].value++ // Wraps naturally at 8-bit
			}
			result = append(result, line)
			skipNextOptimize = false
			continue
		}

		// Check for DEC r - decrements known value
		if match := decPattern.FindStringSubmatch(line); match != nil {
			reg := strings.ToUpper(match[1])
			if regs[reg].known {
				regs[reg].value-- // Wraps naturally at 8-bit
			}
			result = append(result, line)
			skipNextOptimize = false
			continue
		}

		// Check for LD r, r' - copy propagation
		if match := ldRegPattern.FindStringSubmatch(line); match != nil {
			dest := strings.ToUpper(match[2])
			src := strings.ToUpper(match[3])
			if dest != src { // LD A, A is a NOP
				if regs[src].known {
					regs[dest].known = true
					regs[dest].value = regs[src].value
				} else {
					regs[dest].known = false
				}
			}
			result = append(result, line)
			skipNextOptimize = false
			continue
		}

		// Check for LD r, imm - main optimization target
		if match := ldImmPattern.FindStringSubmatch(line); match != nil {
			indent := match[1]
			reg := strings.ToUpper(match[2])
			immStr := match[3]

			// Parse immediate value
			var imm uint64
			fmt.Sscanf(immStr, "%x", &imm)
			immVal := uint8(imm)

			// Check if we can optimize (and should)
			canOptimize := !skipNextOptimize && !hasSMCMarker && regs[reg].known

			// Check if next instruction uses flags (conservative approach)
			if canOptimize && i+1 < len(lines) {
				nextLine := strings.TrimSpace(lines[i+1])
				if flagUsePattern.MatchString(nextLine) {
					canOptimize = false // Don't optimize - next instruction uses flags
				}
			}

			if canOptimize {
				currentVal := regs[reg].value

				if currentVal == immVal {
					// Same value - eliminate entirely
					p.optimizationsCount++
					// Update tracking (value unchanged)
					skipNextOptimize = false
					continue // Skip this line entirely
				} else if currentVal+1 == immVal || (currentVal == 0xFF && immVal == 0x00) {
					// Need value+1, use INC
					result = append(result, fmt.Sprintf("%sINC %s        ; Was LD %s, $%02X (val prop: $%02X+1)", indent, reg, reg, immVal, currentVal))
					regs[reg].value = immVal
					p.optimizationsCount++
					skipNextOptimize = false
					continue
				} else if currentVal-1 == immVal || (currentVal == 0x00 && immVal == 0xFF) {
					// Need value-1, use DEC
					result = append(result, fmt.Sprintf("%sDEC %s        ; Was LD %s, $%02X (val prop: $%02X-1)", indent, reg, reg, immVal, currentVal))
					regs[reg].value = immVal
					p.optimizationsCount++
					skipNextOptimize = false
					continue
				}
			}

			// Can't optimize - update tracking and keep instruction
			regs[reg].known = true
			regs[reg].value = immVal
			result = append(result, line)
			skipNextOptimize = false
			continue
		}

		// Any other instruction that modifies a register - invalidate that register
		// This is a simplified check - in reality we'd need full instruction decoding
		// For now, invalidate registers mentioned in the instruction
		for _, reg := range []string{"A", "B", "C", "D", "E", "H", "L"} {
			// Check if register appears as destination (simplified)
			if strings.Contains(trimmed, reg+",") || strings.HasSuffix(trimmed, " "+reg) {
				// Might be modifying this register - invalidate
				regs[reg].known = false
			}
		}

		result = append(result, line)
		skipNextOptimize = false
	}

	return result
}

// eliminateJumpToNext removes JP/JR instructions that jump to the immediately following label
func (p *AssemblyPeepholePass) eliminateJumpToNext(lines []string) []string {
	jpPattern := regexp.MustCompile(`^\s*J[PR]\s+(?:NZ|Z|NC|C|PO|PE|P|M|)\s*,?\s*(\w+)\s*(?:;.*)?$`)
	labelPattern := regexp.MustCompile(`^(\w+):`)

	result := make([]string, 0, len(lines))

	for i := 0; i < len(lines); i++ {
		line := lines[i]

		// Check if this is a JP or JR instruction
		if matches := jpPattern.FindStringSubmatch(line); matches != nil {
			targetLabel := matches[1]

			// Look for the label in the following lines (skip empty/comment lines)
			foundImmediate := false
			for j := i + 1; j < len(lines) && j < i + 5; j++ {
				nextLine := strings.TrimSpace(lines[j])

				// Skip empty lines and comments
				if nextLine == "" || strings.HasPrefix(nextLine, ";") {
					continue
				}

				// Check if this is our target label
				if labelMatches := labelPattern.FindStringSubmatch(nextLine); labelMatches != nil {
					if labelMatches[1] == targetLabel {
						// JP/JR to next instruction - eliminate it
						foundImmediate = true
						p.optimizationsCount++
						result = append(result, "    ; Eliminated JP/JR to next instruction: "+targetLabel)
						break
					}
				}

				// If we hit any other instruction, stop looking
				break
			}

			if !foundImmediate {
				result = append(result, line)
			}
		} else {
			result = append(result, line)
		}
	}

	return result
}

// eliminateIncDecPairs removes INC r; DEC r and DEC r; INC r pairs
// SAFETY: Traces forward to check if flags are used before being overwritten
// INC/DEC pairs might be used intentionally for flag testing (e.g., test if A==0)
// Also respects @keep and @no-opt annotations in comments
//
// Flag tracing logic:
// - Scan forward from the DEC/INC until we find either:
//   1. Instruction that USES flags (JR Z, JP NZ, ADC, SBC, etc.) → DON'T eliminate
//   2. Instruction that MODIFIES flags (ADD, SUB, AND, OR, XOR, CP, etc.) → safe to eliminate
//   3. Label or control flow (CALL, JP, RET) → assume unsafe, DON'T eliminate
func (p *AssemblyPeepholePass) eliminateIncDecPairs(lines []string) []string {
	// Patterns for INC and DEC
	incPattern := regexp.MustCompile(`^(\s*)INC\s+([ABCDEHL]|HL|DE|BC)\s*(?:;.*)?$`)
	decPattern := regexp.MustCompile(`^(\s*)DEC\s+([ABCDEHL]|HL|DE|BC)\s*(?:;.*)?$`)

	// Instructions that USE flags (must not optimize if flags flow to these)
	flagUsePattern := regexp.MustCompile(`(?i)^\s*(JR\s+(N?[ZC])|JP\s+(N?[ZC]|P[OE]|[PM])|CALL\s+(N?[ZC])|RET\s+(N?[ZC])|ADC|SBC|DAA)`)

	// Instructions that MODIFY/SET flags (safe to eliminate if these come before use)
	// Most ALU ops, CP, AND, OR, XOR, ADD, SUB, INC, DEC, etc.
	flagSetPattern := regexp.MustCompile(`(?i)^\s*(ADD|SUB|AND|OR|XOR|CP|INC|DEC|SLA|SRA|SRL|RLC|RRC|RL|RR|BIT|NEG|CCF|SCF)\s+`)

	// Control flow that ends our analysis (can't trace through)
	controlFlowPattern := regexp.MustCompile(`(?i)^\s*(JP|JR|CALL|RET|RST|RETI|RETN|DJNZ)\s*`)

	// Annotation patterns to preserve instructions
	keepPattern := regexp.MustCompile(`@keep|@no-opt|@preserve|INLINE\s+ASM`)

	labelPattern := regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_$]*:`)

	result := make([]string, 0, len(lines))
	i := 0

	// Helper: check if flags from INC/DEC are used before being overwritten
	// Returns true if safe to eliminate, false if flags might be used
	checkFlagSafety := func(startIdx int) bool {
		maxLookahead := 10 // Don't look too far ahead
		for k := startIdx; k < len(lines) && k < startIdx+maxLookahead; k++ {
			checkLine := strings.TrimSpace(lines[k])

			// Skip empty lines and comments
			if checkLine == "" || strings.HasPrefix(checkLine, ";") {
				continue
			}

			// Label = control flow boundary, assume unsafe
			if labelPattern.MatchString(checkLine) {
				return false
			}

			// Control flow = can't trace, assume unsafe
			if controlFlowPattern.MatchString(checkLine) {
				// But first check if it USES flags (like JR Z)
				if flagUsePattern.MatchString(checkLine) {
					return false // Definitely uses flags
				}
				// Unconditional control flow - assume unsafe
				return false
			}

			// Check if this instruction USES flags
			if flagUsePattern.MatchString(checkLine) {
				return false // Flags are used!
			}

			// Check if this instruction MODIFIES flags
			if flagSetPattern.MatchString(checkLine) {
				return true // Flags will be overwritten, safe to eliminate
			}

			// Instruction doesn't affect flags (like LD) - continue tracing
		}

		// Reached end of lookahead without finding flag use/set
		// Conservative: assume unsafe
		return false
	}

	for i < len(lines) {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		// Skip empty lines and comments
		if trimmed == "" || strings.HasPrefix(trimmed, ";") {
			result = append(result, line)
			i++
			continue
		}

		// Check for @keep annotation - never optimize these
		if keepPattern.MatchString(line) {
			result = append(result, line)
			i++
			continue
		}

		// Check for label - don't optimize across labels (SMC safety)
		if labelPattern.MatchString(trimmed) {
			result = append(result, line)
			i++
			continue
		}

		// Check for INC r
		incMatch := incPattern.FindStringSubmatch(line)
		if incMatch != nil {
			indent := incMatch[1]
			reg := strings.ToUpper(incMatch[2])

			// Look for matching DEC r on next non-empty line
			foundMatch := false
			decIdx := -1
			for j := i + 1; j < len(lines) && j < i+3; j++ {
				nextLine := strings.TrimSpace(lines[j])

				// Skip empty lines and comments
				if nextLine == "" || strings.HasPrefix(nextLine, ";") {
					continue
				}

				// Check for matching DEC
				decMatch := decPattern.FindStringSubmatch(lines[j])
				if decMatch != nil && strings.ToUpper(decMatch[2]) == reg {
					decIdx = j
					break
				}
				// Hit another instruction - stop looking
				break
			}

			if decIdx != -1 && checkFlagSafety(decIdx+1) {
				// Safe to eliminate
				result = append(result, indent+"; Eliminated INC "+reg+"; DEC "+reg+" (flags overwritten)")
				p.optimizationsCount++
				foundMatch = true
				i = decIdx + 1
			}

			if !foundMatch {
				result = append(result, line)
				i++
			}
			continue
		}

		// Check for DEC r
		decMatch := decPattern.FindStringSubmatch(line)
		if decMatch != nil {
			indent := decMatch[1]
			reg := strings.ToUpper(decMatch[2])

			// Look for matching INC r on next non-empty line
			foundMatch := false
			incIdx := -1
			for j := i + 1; j < len(lines) && j < i+3; j++ {
				nextLine := strings.TrimSpace(lines[j])

				// Skip empty lines and comments
				if nextLine == "" || strings.HasPrefix(nextLine, ";") {
					continue
				}

				// Check for matching INC
				incMatch := incPattern.FindStringSubmatch(lines[j])
				if incMatch != nil && strings.ToUpper(incMatch[2]) == reg {
					incIdx = j
					break
				}
				// Hit another instruction - stop looking
				break
			}

			if incIdx != -1 && checkFlagSafety(incIdx+1) {
				// Safe to eliminate
				result = append(result, indent+"; Eliminated DEC "+reg+"; INC "+reg+" (flags overwritten)")
				p.optimizationsCount++
				foundMatch = true
				i = incIdx + 1
			}

			if !foundMatch {
				result = append(result, line)
				i++
			}
			continue
		}

		// Not INC or DEC - keep the line
		result = append(result, line)
		i++
	}

	return result
}

// eliminateRedundantStoreLoad eliminates LD A,n; LD r,A; LD A,r -> LD A,n
// This pattern occurs when a constant is loaded into a virtual register then
// immediately used (e.g., for @print(42))
func (p *AssemblyPeepholePass) eliminateRedundantStoreLoad(lines []string) []string {
	// Pattern: LD A, <imm>; LD <r>, A; ... (comments); LD A, <r>
	// where <r> is the same register (B, C, D, E, H, L)
	ldAImmPattern := regexp.MustCompile(`^\s*LD\s+A\s*,\s*(\d+|[0-9A-Fa-f]+H|\$[0-9A-Fa-f]+)\s*(?:;.*)?$`)
	ldRegAPattern := regexp.MustCompile(`^\s*LD\s+([BCDEHLA])\s*,\s*A\s*(?:;.*)?$`)
	ldARegPattern := regexp.MustCompile(`^\s*LD\s+A\s*,\s*([BCDEHL])\s*(?:;.*)?$`)

	result := make([]string, 0, len(lines))

	for i := 0; i < len(lines); i++ {
		line := lines[i]

		// Check for LD A, <imm>
		immMatch := ldAImmPattern.FindStringSubmatch(line)
		if immMatch != nil {
			// Found LD A, n - look for LD r, A on next non-empty line
			storeIdx := -1
			var storedReg string

			for j := i + 1; j < len(lines) && j < i+3; j++ {
				nextLine := strings.TrimSpace(lines[j])
				if nextLine == "" || strings.HasPrefix(nextLine, ";") {
					continue
				}

				storeMatch := ldRegAPattern.FindStringSubmatch(lines[j])
				if storeMatch != nil && storeMatch[1] != "A" {
					storeIdx = j
					storedReg = storeMatch[1]
					break
				}
				break // Hit different instruction
			}

			if storeIdx != -1 {
				// Found LD r, A - look for LD A, r within next few lines
				loadIdx := -1
				for k := storeIdx + 1; k < len(lines) && k < storeIdx+5; k++ {
					nextLine := strings.TrimSpace(lines[k])
					if nextLine == "" || strings.HasPrefix(nextLine, ";") {
						continue
					}

					loadMatch := ldARegPattern.FindStringSubmatch(lines[k])
					if loadMatch != nil && strings.ToUpper(loadMatch[1]) == storedReg {
						loadIdx = k
						break
					}
					// Check if this instruction uses A or stored reg - if so, can't optimize
					if strings.Contains(strings.ToUpper(nextLine), " A") ||
					   strings.Contains(strings.ToUpper(nextLine), ",A") ||
					   strings.Contains(strings.ToUpper(nextLine), " "+storedReg) ||
					   strings.Contains(strings.ToUpper(nextLine), ","+storedReg) {
						break
					}
				}

				if loadIdx != -1 {
					// Found the pattern! Keep LD A, n but remove LD r, A and LD A, r
					// Keep original line (LD A, n)
					result = append(result, line+" ; (optimized: store/load eliminated)")
					p.optimizationsCount++

					// Skip the LD r, A line but keep comments between store and load
					for j := i + 1; j <= loadIdx; j++ {
						nextLine := strings.TrimSpace(lines[j])
						if strings.HasPrefix(nextLine, ";") {
							result = append(result, lines[j]) // Keep comment lines
						}
						// Skip the actual LD instructions
					}
					i = loadIdx // Continue after LD A, r
					continue
				}
			}
		}

		result = append(result, line)
	}

	return result
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