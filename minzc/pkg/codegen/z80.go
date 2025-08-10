package codegen

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/minz/minzc/pkg/ir"
)

var debug = os.Getenv("DEBUG") != ""

// Z80Generator generates Z80 assembly from IR
type Z80Generator struct {
	writer        io.Writer
	module        *ir.Module
	currentFunc   *ir.Function
	currentFunction *ir.Function // For DJNZ optimization
	currentInstructionIndex int  // For DJNZ optimization
	
	// Hierarchical register allocation system
	regAlloc         *RegisterAllocator      // Simple memory-based allocator (fallback)
	physicalAlloc    *Z80RegisterAllocator   // Sophisticated physical register allocator
	usePhysicalRegs  bool                    // Enable physical register allocation
	
	stackOffset   int
	labelCounter  int
	useShadowRegs bool // Whether to use shadow registers for current function
	localVarBase  uint16 // Base address for local variables (absolute addressing)
	useAbsoluteLocals bool // Whether to use absolute addressing for locals
	emittedParams map[string]bool // Track which SMC parameters have been emitted
	currentRegister ir.Register // Track which virtual register is currently in HL
	targetPlatform string // Target platform (zxspectrum, cpm, msx, etc.)
	constantValues map[ir.Register]int64 // Track constant values in registers
}

// NewZ80Generator creates a new Z80 code generator
func NewZ80Generator(w io.Writer) *Z80Generator {
	physicalAlloc := NewZ80RegisterAllocator()
	// Enable shadow registers for advanced allocation
	physicalAlloc.EnableShadowRegisters()
	
	return &Z80Generator{
		writer:          w,
		regAlloc:        NewRegisterAllocator(),  // Fallback memory allocator
		physicalAlloc:   physicalAlloc,           // Physical register allocator
		usePhysicalRegs: true,                    // Enable hierarchical allocation
		localVarBase:    0xF000,                  // Default local variable area at 0xF000
		targetPlatform:  "zxspectrum",            // Default to ZX Spectrum
		constantValues:  make(map[ir.Register]int64),
	}
}

// SetTargetPlatform sets the target platform for the generator
func (g *Z80Generator) SetTargetPlatform(platform string) {
	g.targetPlatform = platform
}

// uniqueLabel generates a unique label with the given prefix
func (g *Z80Generator) uniqueLabel(prefix string) string {
	label := fmt.Sprintf("%s_%d", prefix, g.labelCounter)
	g.labelCounter++
	return label
}

// isPowerOfTwo checks if a number is a power of 2
func isPowerOfTwo(n int64) bool {
	return n > 0 && (n & (n - 1)) == 0
}

// getShiftCount returns the number of shifts for a power of 2
func getShiftCount(n int64) int {
	count := 0
	for n > 1 {
		n >>= 1
		count++
	}
	return count
}

// canOptimizeMultiplication checks if multiplication can be optimized
func canOptimizeMultiplication(multiplier int64) bool {
	// Power of 2: single shift
	if isPowerOfTwo(multiplier) {
		return true
	}
	
	// Common decompositions
	switch multiplier {
	case 3,   // x * 3 = (x << 1) + x
	     5,   // x * 5 = (x << 2) + x
	     6,   // x * 6 = (x << 2) + (x << 1)
	     7,   // x * 7 = (x << 3) - x
	     9,   // x * 9 = (x << 3) + x
	     10,  // x * 10 = (x << 3) + (x << 1)
	     12,  // x * 12 = (x << 3) + (x << 2)
	     15:  // x * 15 = (x << 4) - x
		return true
	}
	return false
}

// emitOptimizedMultiplication generates optimized multiplication code
func (g *Z80Generator) emitOptimizedMultiplication(multiplier int64, is16bit bool) {
	if isPowerOfTwo(multiplier) {
		// Simple power of 2 - just shift
		shifts := getShiftCount(multiplier)
		g.emit("    ; Optimized multiplication by %d (shift left %d)", multiplier, shifts)
		if is16bit {
			for i := 0; i < shifts; i++ {
				g.emit("    ADD HL, HL        ; HL << 1")
			}
		} else {
			for i := 0; i < shifts; i++ {
				g.emit("    ADD A, A          ; A << 1")
			}
		}
		return
	}
	
	// Complex decompositions
	switch multiplier {
	case 3: // x * 3 = (x << 1) + x
		g.emit("    ; Optimized x * 3 = (x << 1) + x")
		if is16bit {
			g.emit("    LD D, H")
			g.emit("    LD E, L           ; Save original in DE")
			g.emit("    ADD HL, HL        ; x << 1")
			g.emit("    ADD HL, DE        ; + x")
		} else {
			g.emit("    LD B, A           ; Save original")
			g.emit("    ADD A, A          ; x << 1")
			g.emit("    ADD A, B          ; + x")
		}
		
	case 5: // x * 5 = (x << 2) + x
		g.emit("    ; Optimized x * 5 = (x << 2) + x")
		if is16bit {
			g.emit("    LD D, H")
			g.emit("    LD E, L           ; Save original in DE")
			g.emit("    ADD HL, HL        ; x << 1")
			g.emit("    ADD HL, HL        ; x << 2")
			g.emit("    ADD HL, DE        ; + x")
		} else {
			g.emit("    LD B, A           ; Save original")
			g.emit("    ADD A, A          ; x << 1")
			g.emit("    ADD A, A          ; x << 2")
			g.emit("    ADD A, B          ; + x")
		}
		
	case 6: // x * 6 = (x << 2) + (x << 1)
		g.emit("    ; Optimized x * 6 = (x << 2) + (x << 1)")
		if is16bit {
			g.emit("    ADD HL, HL        ; x << 1")
			g.emit("    LD D, H")
			g.emit("    LD E, L           ; Save (x << 1) in DE")
			g.emit("    ADD HL, HL        ; x << 2")
			g.emit("    ADD HL, DE        ; + (x << 1)")
		} else {
			g.emit("    ADD A, A          ; x << 1")
			g.emit("    LD B, A           ; Save (x << 1)")
			g.emit("    ADD A, A          ; x << 2")
			g.emit("    ADD A, B          ; + (x << 1)")
		}
		
	case 7: // x * 7 = (x << 3) - x
		g.emit("    ; Optimized x * 7 = (x << 3) - x")
		if is16bit {
			g.emit("    LD D, H")
			g.emit("    LD E, L           ; Save original in DE")
			g.emit("    ADD HL, HL        ; x << 1")
			g.emit("    ADD HL, HL        ; x << 2")
			g.emit("    ADD HL, HL        ; x << 3")
			g.emit("    OR A              ; Clear carry")
			g.emit("    SBC HL, DE        ; - x")
		} else {
			g.emit("    LD B, A           ; Save original")
			g.emit("    ADD A, A          ; x << 1")
			g.emit("    ADD A, A          ; x << 2")
			g.emit("    ADD A, A          ; x << 3")
			g.emit("    SUB B             ; - x")
		}
		
	case 9: // x * 9 = (x << 3) + x
		g.emit("    ; Optimized x * 9 = (x << 3) + x")
		if is16bit {
			g.emit("    LD D, H")
			g.emit("    LD E, L           ; Save original in DE")
			g.emit("    ADD HL, HL        ; x << 1")
			g.emit("    ADD HL, HL        ; x << 2")
			g.emit("    ADD HL, HL        ; x << 3")
			g.emit("    ADD HL, DE        ; + x")
		} else {
			g.emit("    LD B, A           ; Save original")
			g.emit("    ADD A, A          ; x << 1")
			g.emit("    ADD A, A          ; x << 2")
			g.emit("    ADD A, A          ; x << 3")
			g.emit("    ADD A, B          ; + x")
		}
		
	case 10: // x * 10 = (x << 3) + (x << 1)
		g.emit("    ; Optimized x * 10 = (x << 3) + (x << 1)")
		if is16bit {
			g.emit("    ADD HL, HL        ; x << 1")
			g.emit("    LD D, H")
			g.emit("    LD E, L           ; Save (x << 1) in DE")
			g.emit("    ADD HL, HL        ; x << 2")
			g.emit("    ADD HL, HL        ; x << 3")
			g.emit("    ADD HL, DE        ; + (x << 1)")
		} else {
			g.emit("    ADD A, A          ; x << 1")
			g.emit("    LD B, A           ; Save (x << 1)")
			g.emit("    ADD A, A          ; x << 2")
			g.emit("    ADD A, A          ; x << 3")
			g.emit("    ADD A, B          ; + (x << 1)")
		}
		
	case 12: // x * 12 = (x << 3) + (x << 2)
		g.emit("    ; Optimized x * 12 = (x << 3) + (x << 2)")
		if is16bit {
			g.emit("    ADD HL, HL        ; x << 1")
			g.emit("    ADD HL, HL        ; x << 2")
			g.emit("    LD D, H")
			g.emit("    LD E, L           ; Save (x << 2) in DE")
			g.emit("    ADD HL, HL        ; x << 3")
			g.emit("    ADD HL, DE        ; + (x << 2)")
		} else {
			g.emit("    ADD A, A          ; x << 1")
			g.emit("    ADD A, A          ; x << 2")
			g.emit("    LD B, A           ; Save (x << 2)")
			g.emit("    ADD A, A          ; x << 3")
			g.emit("    ADD A, B          ; + (x << 2)")
		}
		
	case 15: // x * 15 = (x << 4) - x
		g.emit("    ; Optimized x * 15 = (x << 4) - x")
		if is16bit {
			g.emit("    LD D, H")
			g.emit("    LD E, L           ; Save original in DE")
			g.emit("    ADD HL, HL        ; x << 1")
			g.emit("    ADD HL, HL        ; x << 2")
			g.emit("    ADD HL, HL        ; x << 3")
			g.emit("    ADD HL, HL        ; x << 4")
			g.emit("    OR A              ; Clear carry")
			g.emit("    SBC HL, DE        ; - x")
		} else {
			g.emit("    LD B, A           ; Save original")
			g.emit("    ADD A, A          ; x << 1")
			g.emit("    ADD A, A          ; x << 2")
			g.emit("    ADD A, A          ; x << 3")
			g.emit("    ADD A, A          ; x << 4")
			g.emit("    SUB B             ; - x")
		}
	}
}

// Generate generates Z80 assembly for an IR module
func (g *Z80Generator) Generate(module *ir.Module) error {
	g.module = module

	// Write header
	g.writeHeader()

	// Generate data section
	if len(module.Globals) > 0 || len(module.Strings) > 0 {
		g.emit("\n; Data section")
		g.emit("    ORG $F000")  // Data section at $F000
		g.emit("")
		for _, global := range module.Globals {
			g.generateGlobal(global)
		}
		
		// Generate string literals
		if debug {
			fmt.Printf("DEBUG: Generating %d strings in data section\n", len(module.Strings))
		}
		for _, str := range module.Strings {
			if debug {
				fmt.Printf("  String: %s = \"%s\"\n", str.Label, str.Value)
			}
			g.generateString(str)
		}
	}

	// Generate code section
	g.emit("\n; Code section")
	g.emit("    ORG $8000")
	g.emit("")

	// Generate functions
	for _, fn := range module.Functions {
		// fmt.Printf("DEBUG CodeGen: Function %s: IsSMCDefault=%v, IsSMCEnabled=%v, ptr=%p\n", fn.Name, fn.IsSMCDefault, fn.IsSMCEnabled, fn)
		if err := g.generateFunction(fn); err != nil {
			return err
		}
	}

	// Generate PATCH-TABLE if there are any TRUE SMC functions
	g.generatePatchTable()
	
	// Only generate print helpers if they're actually used
	if g.needsPrintHelpers() {
		g.generatePrintHelpers()
	}
	
	// Write footer
	g.writeFooter()

	return nil
}

// writeHeader writes the assembly file header
func (g *Z80Generator) writeHeader() {
	g.emit("; MinZ generated code")
	g.emit("; Generated: %s", time.Now().Format("2006-01-02 15:04:05"))
	g.emit("")
}

// writeFooter writes the assembly file footer
func (g *Z80Generator) writeFooter() {
	g.emit("")
	g.emit("    END main")
}

// generatePatchTable generates the PATCH-TABLE for TRUE SMC functions
func (g *Z80Generator) generatePatchTable() {
	// Collect all TRUE SMC functions and their anchors
	var patchEntries []struct {
		funcName string
		paramName string
		anchorSymbol string
		size int
	}
	
	for _, fn := range g.module.Functions {
		if fn.UsesTrueSMC {
			for _, param := range fn.Params {
				entry := struct {
					funcName string
					paramName string
					anchorSymbol string
					size int
				}{
					funcName: fn.Name,
					paramName: param.Name,
					anchorSymbol: fmt.Sprintf("%s$imm0", param.Name),
					size: param.Type.Size(),
				}
				patchEntries = append(patchEntries, entry)
			}
		}
	}
	
	if len(patchEntries) == 0 {
		return // No TRUE SMC functions
	}
	
	// Emit PATCH-TABLE
	g.emit("")
	g.emit("; TRUE SMC PATCH-TABLE")
	g.emit("; Format: DW anchor_addr, DB size, DB param_tag")
	g.emit("PATCH_TABLE:")
	
	for _, entry := range patchEntries {
		g.emit("    DW %s           ; %s.%s", entry.anchorSymbol, entry.funcName, entry.paramName)
		g.emit("    DB %d              ; Size in bytes", entry.size)
		g.emit("    DB 0              ; Reserved for param tag")
	}
	
	// End marker
	g.emit("    DW 0              ; End of table")
	g.emit("PATCH_TABLE_END:")
}

// generateGlobal generates code for a global variable
func (g *Z80Generator) generateGlobal(global ir.Global) {
	g.emit("%s:", global.Name)
	
	switch t := global.Type.(type) {
	case *ir.BasicType:
		// Handle basic type initialization
		if global.Init != nil {
			// Init contains the evaluated constant value
			switch t.Kind {
			case ir.TypeU8, ir.TypeI8:
				g.emit("    DB %v", global.Init)
			case ir.TypeU16, ir.TypeI16:
				g.emit("    DW %v", global.Init)
			case ir.TypeBool:
				val := 0
				if v, ok := global.Init.(bool); ok && v {
					val = 1
				}
				g.emit("    DB %d", val)
			default:
				g.emit("    DW %v", global.Init)
			}
		} else {
			// No initializer, use zero
			switch t.Kind {
			case ir.TypeU8, ir.TypeI8, ir.TypeBool:
				g.emit("    DB 0")
			case ir.TypeU16, ir.TypeI16:
				g.emit("    DW 0")
			default:
				g.emit("    DW 0")
			}
		}
	case *ir.ArrayType:
		// Handle array initialization
		if global.Init != nil {
			// TODO: Support array initializers
			g.emit("    ; Array with initializer")
			size := global.Type.Size()
			g.emit("    DS %d", size)
		} else {
			size := global.Type.Size()
			g.emit("    DS %d", size)
		}
	case *ir.StructType:
		// Handle struct initialization
		size := global.Type.Size()
		g.emit("    DS %d", size)
	default:
		g.emit("    ; TODO: %s type", global.Type.String())
	}
}

// generateString generates a length-prefixed string literal
func (g *Z80Generator) generateString(str *ir.String) {
	g.emit("%s:", str.Label)
	
	length := len(str.Value)
	
	// For LString (long strings), use u16 length prefix with 255 marker
	if str.IsLong {
		// LString format: [255][len_low][len_high][data...]
		g.emit("    DB 255    ; LString marker")
		g.emit("    DW %d    ; Length (16-bit)", length)
	} else {
		// String format: [len][data...]
		// Regular strings should be <= 255 chars
		if length > 255 {
			// This shouldn't happen if semantic analysis is correct
			// but let's handle it gracefully
			g.emit("    DB 255    ; LString marker (auto-promoted)")
			g.emit("    DW %d    ; Length (16-bit)", length)
		} else {
			g.emit("    DB %d    ; Length", length)
		}
	}
	
	// String content
	if length > 0 {
		// Collect all bytes/strings for a single DB directive
		var dbOperands []string
		escaped := ""
		
		for _, ch := range str.Value {
			if ch >= 32 && ch <= 126 && ch != '"' && ch != '\\' {
				escaped += string(ch)
			} else {
				// If we have accumulated string content, add it as an operand
				if escaped != "" {
					dbOperands = append(dbOperands, fmt.Sprintf("\"%s\"", escaped))
					escaped = ""
				}
				// Add special character as numeric value
				dbOperands = append(dbOperands, fmt.Sprintf("%d", ch))
			}
		}
		
		// Add any remaining string content
		if escaped != "" {
			dbOperands = append(dbOperands, fmt.Sprintf("\"%s\"", escaped))
		}
		
		// Emit as a single DB directive with comma-separated operands
		if len(dbOperands) > 0 {
			g.emit("    DB %s", strings.Join(dbOperands, ", "))
		}
	}
	
	// No null terminator needed - length-prefixed strings
}

// generateFunction generates code for a function
func (g *Z80Generator) generateFunction(fn *ir.Function) error {
	g.currentFunc = fn
	g.currentFunction = fn
	g.currentInstructionIndex = 0
	g.stackOffset = 0
	g.regAlloc.Reset()

	// Perform hierarchical register allocation if enabled
	if g.usePhysicalRegs {
		g.physicalAlloc.AllocateFunction(fn)
		g.emit("; Using hierarchical register allocation (physical → shadow → memory)")
	}

	// Function label
	g.emit("")
	g.emit("; Function: %s", fn.Name)
	// g.emit("; IsSMCDefault=%v, IsSMCEnabled=%v", fn.IsSMCDefault, fn.IsSMCEnabled)
	
	// Check if this is an SMC function
	if fn.IsSMCDefault || fn.IsSMCEnabled {
		return g.generateSMCFunction(fn)
	}
	
	// Traditional function generation
	g.emit("%s:", fn.Name)

	// Determine if we should use stack-based locals
	useStackLocals := g.shouldUseStackLocals(fn)
	if useStackLocals {
		g.emit("; Using stack-based locals (IX+offset)")
		g.useAbsoluteLocals = false
	} else {
		g.emit("; Using absolute addressing for locals")
		g.useAbsoluteLocals = true
	}

	// Allocate addresses/offsets for local variables
	if g.useAbsoluteLocals {
		// Absolute addressing mode
		localOffset := uint16(0)
		localAddresses := make(map[string]uint16)
		for _, local := range fn.Locals {
			addr := g.localVarBase + localOffset
			localAddresses[local.Name] = addr
			g.regAlloc.SetAddress(local.Reg, addr)
			localOffset += uint16(local.Type.Size())
		}
	} else {
		// Stack-based addressing mode (IX+offset)
		// Locals are at negative offsets from IX
		localOffset := 0
		for _, local := range fn.Locals {
			localOffset += local.Type.Size()
			// Store negative offset (locals grow downward)
			g.regAlloc.SetAddress(local.Reg, uint16(localOffset))
		}
		g.stackOffset = localOffset
	}
	
	// Function prologue
	g.generatePrologue(fn)

	// Reset constant tracking for new function
	g.constantValues = make(map[ir.Register]int64)
	
	// Generate instructions
	for i, inst := range fn.Instructions {
		g.currentInstructionIndex = i
		if err := g.generateInstruction(inst); err != nil {
			return err
		}
	}

	// Function epilogue (if not already returned)
	if len(fn.Instructions) == 0 || fn.Instructions[len(fn.Instructions)-1].Op != ir.OpReturn {
		g.generateEpilogue()
	}

	return nil
}

// generateTrueSMCFunction generates a TRUE SMC function with anchor-based parameters
func (g *Z80Generator) generateTrueSMCFunction(fn *ir.Function) error {
	g.emit("%s:", fn.Name)
	g.emit("; TRUE SMC function with immediate anchors")
	
	// Always use absolute addressing for SMC functions
	g.useAbsoluteLocals = true
	
	// Track which parameters have been anchored
	anchoredParams := make(map[string]bool)
	
	// Don't generate anchors here - wait for first use
	// Store function for later reference
	g.currentFunc = fn
	
	// Generate function body
	for _, inst := range fn.Instructions {
		// Check if this is first use of a parameter (could be OpTrueSMCLoad already)
		if (inst.Op == ir.OpLoadParam || inst.Op == ir.OpTrueSMCLoad) && inst.Symbol != "" {
			paramName := inst.Symbol
			// Extract parameter name from symbol (might be "x$imm0" format)
			if idx := strings.Index(paramName, "$"); idx > 0 {
				paramName = paramName[:idx]
			}
			
			if !anchoredParams[paramName] {
				// Generate anchor at first use
				param := g.findParameter(fn, paramName)
				if param != nil {
					anchoredParams[paramName] = true
					g.generateParameterAnchor(param, inst.Dest)
					continue
				}
			}
		}
		
		if err := g.generateSMCInstruction(inst); err != nil {
			return err
		}
	}
	
	// Add RET if not already present
	if len(fn.Instructions) == 0 || fn.Instructions[len(fn.Instructions)-1].Op != ir.OpReturn {
		g.emit("    RET")
	}
	
	return nil
}

// generateParameterAnchor generates an anchor for a parameter at first use
func (g *Z80Generator) generateParameterAnchor(param *ir.Parameter, destReg ir.Register) {
	anchorOp := fmt.Sprintf("%s$immOP", param.Name)
	anchorImm := fmt.Sprintf("%s$imm0", param.Name)
	
	g.emit("%s:", anchorOp)
	
	if param.Type.Size() == 1 {
		// 8-bit parameter - check if destination is a physical register
		if g.usePhysicalRegs {
			// Try to get physical register allocation
			if physReg, ok := g.physicalAlloc.GetAllocation(destReg); ok {
				// Check if it's a simple 8-bit register
				switch physReg {
				case RegA, RegB, RegC, RegD, RegE:
					regName := g.physicalRegToAssembly(physReg)
					g.emit("    LD %s, 0        ; %s anchor (will be patched)", regName, param.Name)
					g.emit("%s EQU %s+1", anchorImm, anchorOp)
					return
				}
			}
		}
		
		// Fall back to using A as intermediate
		g.emit("    LD A, 0        ; %s anchor (will be patched)", param.Name)
		g.emit("%s EQU %s+1", anchorImm, anchorOp)
		g.storeFromA(destReg)
	} else if param.Type.Size() == 2 {
		// 16-bit parameter - use LD HL, nn
		g.emit("    LD HL, 0       ; %s anchor (will be patched)", param.Name)
		g.emit("%s EQU %s+1", anchorImm, anchorOp)
		// Value is now in HL, store to destination
		g.storeFromHL(destReg)
	} else if param.Type.Size() == 3 {
		// 24-bit parameter - need two anchors: A for high byte, HL for low 16 bits
		anchorHigh := fmt.Sprintf("%s$immHI", param.Name)
		anchorLow := fmt.Sprintf("%s$immLO", param.Name)
		
		// Load high byte into A
		g.emit("%s:", anchorHigh)
		g.emit("    LD A, 0        ; %s high byte anchor (will be patched)", param.Name)
		g.emit("%s EQU %s+1", anchorHigh+"0", anchorHigh)
		
		// Load low 16 bits into HL
		g.emit("%s:", anchorLow)
		g.emit("    LD HL, 0       ; %s low 16 bits anchor (will be patched)", param.Name)
		g.emit("%s EQU %s+1", anchorLow+"0", anchorLow)
		
		// For now, just store HL to destination (low 16 bits only)
		// TODO: Need proper 24-bit register allocation
		g.storeFromHL(destReg)
	}
}

// findParameter finds a parameter by name in a function
func (g *Z80Generator) findParameter(fn *ir.Function, name string) *ir.Parameter {
	for i := range fn.Params {
		if fn.Params[i].Name == name {
			return &fn.Params[i]
		}
	}
	return nil
}

// generateSMCFunction generates an SMC-based function
func (g *Z80Generator) generateSMCFunction(fn *ir.Function) error {
	// Check if this uses TRUE SMC with anchors
	if fn.UsesTrueSMC {
		return g.generateTrueSMCFunction(fn)
	}
	
	g.emit("%s:", fn.Name)
	
	// Always use absolute addressing for SMC functions
	g.useAbsoluteLocals = true
	g.emittedParams = make(map[string]bool)
	
	// Comment about optimization strategy
	g.emit("; IsSMCDefault=%v, IsSMCEnabled=%v", fn.IsSMCDefault, fn.IsSMCEnabled)
	g.emit("; Using absolute addressing for locals (SMC style)")
	if fn.IsRecursive {
		g.emit("; Recursive context handled via stack push/pop of SMC parameters")
	}
	
	// If this has tail recursion, add the start label
	if fn.HasTailRecursion {
		g.emit("%s_start:", fn.Name)
	}
	
	// Generate minimal prologue if needed
	if fn.UsedRegisters != 0 && !fn.IsRecursive {
		// Only save registers if not recursive (recursive saves in context)
		if fn.ModifiedRegisters.Contains(ir.Z80_BC) {
			g.emit("    PUSH BC")
		}
		if fn.ModifiedRegisters.Contains(ir.Z80_DE) {
			g.emit("    PUSH DE")
		}
	}
	
	// Reset constant tracking for new function
	g.constantValues = make(map[ir.Register]int64)
	
	// Generate instructions with SMC awareness
	for i, inst := range fn.Instructions {
		// Check if this is the last instruction and it's a return - replace with patch points if needed
		isLastInst := i == len(fn.Instructions)-1
		if isLastInst && inst.Op == ir.OpReturn && fn.NeedsPatchPoints {
			// Replace regular return with patchable return sequence
			g.generatePatchableReturn(fn, inst)
		} else {
			if err := g.generateSMCInstruction(inst); err != nil {
				return err
			}
		}
	}
	
	// Epilogue if needed
	if len(fn.Instructions) == 0 || fn.Instructions[len(fn.Instructions)-1].Op != ir.OpReturn {
		if fn.UsedRegisters != 0 && !fn.IsRecursive {
			if fn.ModifiedRegisters.Contains(ir.Z80_DE) {
				g.emit("    POP DE")
			}
			if fn.ModifiedRegisters.Contains(ir.Z80_BC) {
				g.emit("    POP BC")
			}
		}
		if fn.NeedsPatchPoints {
			g.generatePatchableReturn(fn, ir.Instruction{Op: ir.OpReturn})
		} else {
			g.emit("    RET")
		}
	}
	
	return nil
}

// generatePatchableReturn generates a patchable return sequence instead of regular RET
func (g *Z80Generator) generatePatchableReturn(fn *ir.Function, inst ir.Instruction) {
	// First, handle the return value if needed - assume it's in register A for u8
	if inst.Op == ir.OpReturn && inst.Src1 != 0 {
		g.loadToA(inst.Src1)
	}
	
	// Add patch points following the expected pattern
	g.emit("")
	g.emit("    ; *** SMART PATCHABLE RETURN SEQUENCE ***")
	g.emit("    ; Default: Store to memory (most common complex case)")
	g.emit("    ; For immediate use: Patch first NOP to RET for early return")
	
	// Generate the patch point labels
	g.emit("%s_return_patch.op:", fn.Name)
	g.emit("    NOP                     ; PATCH POINT: NOP or RET (C9) for early return")
	
	g.emit("%s_store_addr.op:", fn.Name)
	g.emit("%s_store_addr equ %s_store_addr.op + 1", fn.Name, fn.Name)
	g.emit("    LD (0000), A            ; DEFAULT: Store result (address gets patched)")
	g.emit("    RET                     ; Return after store")
}

// generateSMCInstruction generates an instruction for SMC function
func (g *Z80Generator) generateSMCInstruction(inst ir.Instruction) error {
	switch inst.Op {
	case ir.OpCall:
		// Check if this is a recursive call
		if inst.Symbol == g.currentFunc.Name && g.currentFunc.RequiresContext {
			return g.generateSMCRecursiveCall(inst)
		}
		// Fall through to regular instruction generation
		return g.generateInstruction(inst)
		
	case ir.OpTrueSMCLoad:
		// TRUE SMC: Load from anchor address (повторное использование)
		// The symbol should already include $imm0
		anchorAddr := inst.Symbol
		if !strings.HasSuffix(anchorAddr, "$imm0") {
			// Legacy format - add $imm0
			anchorAddr = strings.TrimSuffix(anchorAddr, "$immOP") + "$imm0"
		}
		
		if inst.Type != nil && inst.Type.Size() == 1 {
			g.emit("    LD A, (%s)    ; Reuse from anchor", anchorAddr)
			g.storeFromA(inst.Dest)
		} else {
			g.emit("    LD HL, (%s)   ; Reuse from anchor", anchorAddr)
			g.storeFromHL(inst.Dest)
		}
		return nil
		
	case ir.OpTrueSMCPatch:
		// TRUE SMC: Patch anchor before call
		// This is handled in generateCall when we see a call to SMC function
		g.emit("    ; TRUE SMC patch handled at call site")
		return nil
		
	case ir.OpSetError:
		// Carry-flag error ABI: Set CY=1 and error code in A
		if inst.Imm != 0 {
			g.emit("    LD A, %d       ; Error code", inst.Imm)
		} else {
			g.loadToA(inst.Src1) // Load error code from register
		}
		g.emit("    SCF              ; Set carry flag (error)")
		return nil
		
	case ir.OpCheckError:
		// Carry-flag error ABI: Check CY flag
		// Dest = 1 if error (CY=1), 0 if success (CY=0)
		g.emit("    LD HL, 0       ; Assume success")
		noerrLabel := g.getFunctionLabel("no_err")
		g.emit("    JR NC, %s", noerrLabel)
		g.emit("    INC HL         ; Error detected")
		g.emit("%s:", noerrLabel)
		g.labelCounter++
		g.storeFromHL(inst.Dest)
		return nil
		
	case ir.OpLoadParam:
		// For SMC, emit the parameter instruction at point of FIRST use
		// The instruction itself contains the parameter value!
		paramName := inst.Symbol
		paramLabel := fmt.Sprintf("%s_param_%s", g.currentFunc.Name, paramName)
		
		// Check if we've already emitted this parameter
		if !g.emittedParams[paramName] {
			// First use - emit the parameter instruction
			g.emittedParams[paramName] = true
			
			// Find the parameter info
			var param *ir.Parameter
			for _, p := range g.currentFunc.Params {
				if p.Name == paramName {
					param = &p
					break
				}
			}
			
			if param == nil {
				return fmt.Errorf("parameter %s not found", paramName)
			}
			
			// Check if this is a TSMC reference parameter
			if param.IsTSMCRef {
				// TSMC reference: Create anchor for indirect memory operations
				g.emit("; TSMC reference parameter %s", paramName)
				g.emit("%s$immOP:", paramName)
				
				// For pointers, we emit instructions that will have their immediates patched
				if _, ok := param.Type.(*ir.PointerType); ok {
					// ALL pointers load the ADDRESS into HL, not the value!
					g.emit("    LD HL, 0000      ; TSMC ref address for %s", paramName)
					g.emit("%s$imm0 EQU %s$immOP+1", paramName, paramName)
					// Store the address (not dereferenced value)
					g.storeFromHL(inst.Dest)
				} else {
					// Non-pointer TSMC ref (future extension)
					g.emit("    LD HL, 0000      ; TSMC ref %s", paramName)
					g.emit("%s$imm0 EQU %s$immOP+1", paramName, paramName)
					g.storeFromHL(inst.Dest)
				}
			} else {
				// Regular SMC parameter - use different registers for different parameters
				// Add patch point labels for instruction patching
				g.emit("%s.op:", paramLabel)
				g.emit("%s equ %s.op + 1", paramLabel, paramLabel)
				
				// Find parameter index to use different registers
				paramIndex := -1
				for i, p := range g.currentFunc.Params {
					if p.Name == paramName {
						paramIndex = i
						break
					}
				}
				
				
				// For the first use, we need to emit the load instruction
				if param.Type.Size() == 1 {
					// For u8/f.8, check if it's f.8 which should use A directly
					if bt, ok := param.Type.(*ir.BasicType); ok && bt.Kind == ir.TypeF_8 {
						// f.8 should use A register directly
						g.emit("    LD A, #00      ; SMC parameter %s (f.8)", paramName)
						g.storeFromA(inst.Dest)
					} else {
						// Other u8 types, use native 8-bit registers for instruction patching
						switch paramIndex {
						case 0:
							g.emit("    LD A, #00      ; Parameter %s (gets patched)", paramName)
							g.storeFromA(inst.Dest)
						case 1:
							g.emit("    LD B, #00      ; Parameter %s (gets patched)", paramName)
							g.emit("    LD A, B")
							g.storeFromA(inst.Dest)
						default:
							g.emit("    LD C, #00      ; Parameter %s (gets patched)", paramName)
							g.emit("    LD A, C")
							g.storeFromA(inst.Dest)
						}
					}
				} else if param.Type.Size() == 3 {
					// For 24-bit types (u24/i24/f16.8/f8.16), use A+HL split
					g.emit("    LD A, #00      ; SMC parameter %s (high byte)", paramName)
					g.emit("    LD HL, #0000   ; SMC parameter %s (low 16 bits)", paramName)
					// Store the 24-bit value
					// For now, store as 16-bit in HL (truncating high byte)
					// TODO: Properly handle 24-bit storage
					g.storeFromHL(inst.Dest)
				} else {
					// 16-bit types
					switch paramIndex {
					case 0:
						g.emit("    LD HL, #0000   ; SMC parameter %s", paramName)
					case 1:
						g.emit("    LD DE, #0000   ; SMC parameter %s", paramName)
						g.emit("    EX DE, HL      ; Move to HL for storage")
					default:
						g.emit("    LD BC, #0000   ; SMC parameter %s", paramName)
						g.emit("    LD H, B")
						g.emit("    LD L, C        ; Move to HL for storage")
					}
					// Store to the destination
					g.storeFromHL(inst.Dest)
				}
			}
		} else {
			// Subsequent use - need to check if TSMC ref or regular param
			var param *ir.Parameter
			for _, p := range g.currentFunc.Params {
				if p.Name == paramName {
					param = &p
					break
				}
			}
			
			if param != nil && param.IsTSMCRef {
				// TSMC reference - reload the address from immediate
				if _, ok := param.Type.(*ir.PointerType); ok {
					// Reload the address from the immediate
					g.emit("    LD HL, (%s$imm0) ; Reload TSMC ref address", paramName)
					g.storeFromHL(inst.Dest)
				} else {
					g.emit("    LD HL, (%s$imm0) ; Reload TSMC ref value", paramName)
					g.storeFromHL(inst.Dest)
				}
			} else {
				// Regular SMC parameter - load from the parameter location
				if inst.Type != nil && inst.Type.Size() == 1 {
					g.emit("    LD A, (%s)", paramLabel)
					g.storeFromA(inst.Dest)
				} else {
					g.emit("    LD HL, (%s)", paramLabel)
					g.storeFromHL(inst.Dest)
				}
			}
		}
		return nil
		
	default:
		// Use regular instruction generation
		return g.generateInstruction(inst)
	}
}

// generateSMCRecursiveCall generates a recursive call with context save/restore
func (g *Z80Generator) generateSMCRecursiveCall(inst ir.Instruction) error {
	fn := g.currentFunc
	
	g.emit("    ; === SMC Recursive Context Save ===")
	
	// Save all SMC parameters
	for _, param := range fn.Params {
		paramLabel := fmt.Sprintf("%s_param_%s", fn.Name, param.Name)
		
		if param.Type.Size() == 1 {
			g.emit("    LD A, (%s)", paramLabel)
			g.emit("    PUSH AF")
		} else {
			g.emit("    LD HL, (%s)", paramLabel)
			g.emit("    PUSH HL")
		}
	}
	
	g.emit("    ; === Update SMC Parameters ===")
	// Note: The semantic analyzer should have generated instructions to
	// set up the new parameter values before the call
	
	g.emit("    CALL %s", inst.Symbol)
	
	g.emit("    ; === SMC Recursive Context Restore ===")
	// Restore in reverse order
	for i := len(fn.Params) - 1; i >= 0; i-- {
		param := fn.Params[i]
		paramLabel := fmt.Sprintf("%s_param_%s", fn.Name, param.Name)
		
		if param.Type.Size() == 1 {
			g.emit("    POP AF")
			g.emit("    LD (%s), A", paramLabel)
		} else {
			g.emit("    POP HL")
			g.emit("    LD (%s), HL", paramLabel)
		}
	}
	
	// Store the result if needed
	if inst.Dest != 0 {
		g.storeFromHL(inst.Dest)
	}
	
	return nil
}

// generatePrologue generates function prologue
func (g *Z80Generator) generatePrologue(fn *ir.Function) {
	// Generate lean prologue based on actual register usage
	
	// For interrupt handlers, save all modified registers
	if fn.IsInterrupt {
		g.generateInterruptPrologue(fn)
		return
	}
	
	// Save only the registers we actually modify
	if fn.ModifiedRegisters.Contains(ir.Z80_AF) {
		g.emit("    PUSH AF")
	}
	if fn.ModifiedRegisters.Contains(ir.Z80_BC) {
		g.emit("    PUSH BC")
	}
	if fn.ModifiedRegisters.Contains(ir.Z80_DE) {
		g.emit("    PUSH DE")
	}
	if fn.ModifiedRegisters.Contains(ir.Z80_HL) {
		g.emit("    PUSH HL")
	}
	
	// Setup stack frame if using stack-based locals
	if !g.useAbsoluteLocals && (len(fn.Locals) > 0 || len(fn.Params) > 0) {
		g.emit("    PUSH IX")
		g.emit("    LD IX, SP")
		
		// Allocate space for locals
		if g.stackOffset > 0 {
			if g.stackOffset <= 127 {
				// Small frame - use ADD SP
				g.emit("    LD HL, -%d", g.stackOffset)
				g.emit("    ADD HL, SP")
				g.emit("    LD SP, HL")
			} else {
				// Large frame
				g.emit("    LD HL, -%d", g.stackOffset)
				g.emit("    ADD HL, SP")
				g.emit("    LD SP, HL")
			}
		}
	} else if len(fn.Locals) > 0 || len(fn.Params) > 0 {
		// Even in absolute mode, we might need IX for parameters
		g.emit("    PUSH IX")
		g.emit("    LD IX, SP")
	}
	
	// Check if we should use shadow registers for this function
	if fn.UsedRegisters.Contains(ir.Z80_BC_SHADOW | ir.Z80_DE_SHADOW | ir.Z80_HL_SHADOW) {
		g.useShadowRegs = true
		g.emit("    EXX           ; Switch to shadow registers")
	}

	// Load parameters based on calling convention
	if fn.IsRecursive || fn.IsSMCEnabled || len(fn.Params) > 3 {
		// Stack-based parameters (traditional)
		for i, param := range fn.Params {
			// Parameters are at positive offsets from IX
			// First param at IX+4 (after return address and saved IX)
			offset := 4 + i*2
			g.emit("    ; Parameter %s from stack", param.Name)
			
			// Load from stack
			g.emit("    LD L, (IX+%d)", offset)
			g.emit("    LD H, (IX+%d)", offset+1)
			
			// Store in local variable space
			if g.useAbsoluteLocals {
				g.storeFromHL(param.Reg)
			} else {
				localOffset := g.getLocalOffset(param.Reg)
				g.emit("    LD (IX%+d), L", localOffset)
				g.emit("    LD (IX%+d), H", localOffset+1)
			}
		}
	} else {
		// Register-based parameters (optimized)
		g.loadParametersFromRegisters(fn)
	}
}

// generateEpilogue generates function epilogue
func (g *Z80Generator) generateEpilogue() {
	// Generate lean epilogue based on what we saved
	fn := g.currentFunc
	
	// For interrupt handlers
	if fn.IsInterrupt {
		g.generateInterruptEpilogue(fn)
		return
	}
	
	// For SMC functions
	if fn.IsSMCDefault || fn.IsSMCEnabled {
		// No IX usage at all - even recursive functions don't need it!
		// SMC parameter context is handled via stack push/pop
		if fn.UsedRegisters != 0 && !fn.IsRecursive {
			if fn.ModifiedRegisters.Contains(ir.Z80_DE) {
				g.emit("    POP DE")
			}
			if fn.ModifiedRegisters.Contains(ir.Z80_BC) {
				g.emit("    POP BC")
			}
		}
		g.emit("    RET")
		return
	}
	
	// Traditional function epilogue
	// Restore shadow register state if used
	if g.useShadowRegs {
		g.emit("    EXX           ; Restore main registers")
	}
	
	// Restore stack frame if we used it
	if len(fn.Locals) > 0 || len(fn.Params) > 0 {
		g.emit("    LD SP, IX")
		g.emit("    POP IX")
	}
	
	// Restore registers in reverse order
	if fn.ModifiedRegisters.Contains(ir.Z80_HL) {
		g.emit("    POP HL")
	}
	if fn.ModifiedRegisters.Contains(ir.Z80_DE) {
		g.emit("    POP DE")
	}
	if fn.ModifiedRegisters.Contains(ir.Z80_BC) {
		g.emit("    POP BC")
	}
	if fn.ModifiedRegisters.Contains(ir.Z80_AF) {
		g.emit("    POP AF")
	}
	
	g.emit("    RET")
}

// generatePatchPoint generates a patchable instruction sequence
func (g *Z80Generator) generatePatchPoint(inst *ir.Instruction) error {
	if inst.PatchPoint == nil {
		return fmt.Errorf("PatchPoint instruction missing PatchPoint data")
	}
	
	pp := inst.PatchPoint
	g.emit("\n; *** PATCHABLE INSTRUCTION SEQUENCE: %s ***", pp.Label)
	g.emit("; Templates: %s", strings.Join(g.getTemplateNames(pp), ", "))
	g.emit("; Default: %s (%d bytes reserved)", pp.Default, pp.Size)
	
	// Generate the patch point label and default template
	g.emit("%s:", pp.Label)
	defaultTemplate := pp.Templates[pp.Default]
	if defaultTemplate == nil {
		return fmt.Errorf("default template '%s' not found in patch point", pp.Default)
	}
	
	// Emit default template with padding
	g.emitTemplateBytes(pp.Label, defaultTemplate, pp.Size)
	
	return nil
}

// generatePatchTemplate selects template for patchable instruction sequence (SMART PATCHING)
func (g *Z80Generator) generatePatchTemplate(inst *ir.Instruction) error {
	g.emit("    ; Smart patch '%s' for %s", inst.TemplateName, inst.PatchPointLabel)
	
	// SMART PATCHING: Single-byte patch based on usage pattern
	switch inst.TemplateName {
	case "immediate":
		// For immediate use: Patch NOP → RET for early return (saves 24+ T-states!)
		g.emit("    LD A, #C9               ; RET opcode")
		g.emit("    LD (%s.op), A", inst.PatchPointLabel)
	case "store_u8", "store_u16":
		// For storage: Restore NOP (default behavior: continue to store)
		g.emit("    LD A, #00               ; NOP opcode")
		g.emit("    LD (%s.op), A", inst.PatchPointLabel)
	case "reg_b":
		// For register transfer: Patch NOP → LD B, A
		g.emit("    LD A, #47               ; LD B, A opcode")
		g.emit("    LD (%s.op), A", inst.PatchPointLabel)
	case "reg_c":
		// For register transfer: Patch NOP → LD C, A
		g.emit("    LD A, #4F               ; LD C, A opcode")
		g.emit("    LD (%s.op), A", inst.PatchPointLabel)
	default:
		// Default: restore NOP for store pattern
		g.emit("    LD A, #00               ; NOP opcode (default)")
		g.emit("    LD (%s.op), A", inst.PatchPointLabel)
	}
	
	return nil
}

// generatePatchTarget sets target address for store operations in patch (SMART PATCHING)
func (g *Z80Generator) generatePatchTarget(inst *ir.Instruction) error {
	g.emit("    ; Patch storage address: %s", inst.TargetAddress)
	
	// Smart patching: patch the storage address directly
	funcName := inst.PatchPointLabel // This should be the function name + "_return_patch"
	funcBaseName := strings.TrimSuffix(funcName, "_return_patch")
	
	g.emit("    LD HL, %s", inst.TargetAddress)
	g.emit("    LD (%s_store_addr), HL", funcBaseName)
	
	return nil
}

// generatePatchParam patches function parameter immediate
func (g *Z80Generator) generatePatchParam(inst *ir.Instruction) error {
	g.emit("    ; Patch parameter %s = %d", inst.ParamName, inst.Imm)
	
	// Load immediate value
	if inst.Type != nil && inst.Type.Size() == 1 {
		// 8-bit parameter
		g.emit("    LD A, %d               ; Parameter value", inst.Imm)
		g.emit("    LD (%s_param_%s+1), A   ; Patch parameter immediate", 
			   inst.Symbol, inst.ParamName)
	} else {
		// 16-bit parameter
		g.emit("    LD HL, %d              ; Parameter value", inst.Imm)
		g.emit("    LD (%s_param_%s+1), HL  ; Patch parameter immediate", 
			   inst.Symbol, inst.ParamName)
	}
	
	return nil
}

// Helper functions for patch point generation

func (g *Z80Generator) getTemplateNames(pp *ir.PatchPoint) []string {
	names := make([]string, 0, len(pp.Templates))
	for name := range pp.Templates {
		names = append(names, name)
	}
	return names
}

func (g *Z80Generator) emitTemplateBytes(label string, template *ir.PatchTemplate, reservedSize int) {
	g.emit("    ; Default template: %s (%s)", template.Name, template.Description)
	
	// Emit template bytes as data
	for i, b := range template.Instructions {
		if i == 0 {
			g.emit("    DB $%02X               ; %s", b, template.Description)
		} else {
			g.emit("    DB $%02X", b)
		}
	}
	
	// Pad with NOPs if needed
	for i := template.Size; i < reservedSize; i++ {
		g.emit("    DB $00                 ; NOP padding")
	}
}

// prepareCallArguments prepares arguments for a function call
func (g *Z80Generator) prepareCallArguments(args []ir.Register, targetFunc *ir.Function) {
	// For SMC functions, parameters are patched directly - no need to prepare here
	if targetFunc != nil && (targetFunc.IsSMCDefault || targetFunc.IsSMCEnabled) {
		// SMC parameters are handled by generateTrueSMCCall or embedded in the function
		return
	}
	
	// Determine calling convention for non-SMC functions
	useRegisterPassing := false
	if targetFunc != nil && !targetFunc.IsRecursive && len(args) <= 3 {
		useRegisterPassing = true
	}
	
	if useRegisterPassing && targetFunc != nil {
		// Register-based parameter passing
		g.emit("    ; Register-based parameter passing")
		
		// Map arguments to registers based on type and position
		for i, arg := range args {
			if i >= len(targetFunc.Params) {
				break
			}
			param := targetFunc.Params[i]
			
			if param.Type.Size() == 1 {
				// 8-bit parameter
				switch i {
				case 0:
					g.loadToA(arg)
					g.emit("    ; Parameter %s in A", param.Name)
				case 1:
					g.loadToA(arg)
					g.emit("    LD E, A       ; Parameter %s in E", param.Name)
				case 2:
					g.loadToA(arg)
					g.emit("    LD D, A       ; Parameter %s in D", param.Name)
				}
			} else {
				// 16-bit parameter
				switch i {
				case 0:
					g.loadToHL(arg)
					g.emit("    ; Parameter %s in HL", param.Name)
				case 1:
					g.loadToDE(arg)
					g.emit("    ; Parameter %s in DE", param.Name)
				case 2:
					g.loadToHL(arg)
					g.emit("    PUSH HL       ; Parameter %s on stack", param.Name)
				}
			}
		}
	} else {
		// Stack-based parameter passing (traditional)
		g.emit("    ; Stack-based parameter passing")
		
		// Push arguments in reverse order (rightmost first)
		for i := len(args) - 1; i >= 0; i-- {
			g.loadToHL(args[i])
			g.emit("    PUSH HL       ; Argument %d", i)
		}
	}
}

// loadParametersFromRegisters loads function parameters from registers
func (g *Z80Generator) loadParametersFromRegisters(fn *ir.Function) {
	// Check if this function uses register-based parameters
	if fn.IsRecursive || fn.IsSMCEnabled || len(fn.Params) > 3 {
		// Use traditional stack-based parameters
		return
	}
	
	g.emit("    ; Load parameters from registers")
	
	for i, param := range fn.Params {
		if param.Type.Size() == 1 {
			// 8-bit parameter
			switch i {
			case 0:
				// Parameter already in A
				g.storeFromA(param.Reg)
			case 1:
				g.emit("    LD A, E       ; Get parameter %s", param.Name)
				g.storeFromA(param.Reg)
			case 2:
				g.emit("    LD A, D       ; Get parameter %s", param.Name)
				g.storeFromA(param.Reg)
			}
		} else {
			// 16-bit parameter
			switch i {
			case 0:
				// Parameter already in HL
				g.storeFromHL(param.Reg)
			case 1:
				g.emit("    EX DE, HL     ; Get parameter %s from DE", param.Name)
				g.storeFromHL(param.Reg)
			case 2:
				// Parameter on stack
				g.emit("    POP HL        ; Get parameter %s from stack", param.Name)
				g.storeFromHL(param.Reg)
			}
		}
	}
}

// generateInterruptPrologue generates prologue for interrupt handlers
func (g *Z80Generator) generateInterruptPrologue(fn *ir.Function) {
	// Interrupt handlers must save ALL registers they modify
	// Use EX and EXX for efficiency when possible
	
	if fn.ModifiedRegisters.Contains(ir.Z80_AF) {
		g.emit("    EX AF, AF'    ; Save AF to shadow")
	}
	
	if fn.ModifiedRegisters.Contains(ir.Z80_BC | ir.Z80_DE | ir.Z80_HL) {
		g.emit("    EXX           ; Save BC, DE, HL to shadows")
	}
	
	// If we need more than shadow registers can hold, use stack
	if fn.ModifiedRegisters.Contains(ir.Z80_IX) {
		g.emit("    PUSH IX")
	}
	if fn.ModifiedRegisters.Contains(ir.Z80_IY) {
		g.emit("    PUSH IY")
	}
}

// generateInterruptEpilogue generates epilogue for interrupt handlers
func (g *Z80Generator) generateInterruptEpilogue(fn *ir.Function) {
	// Restore in reverse order
	if fn.ModifiedRegisters.Contains(ir.Z80_IY) {
		g.emit("    POP IY")
	}
	if fn.ModifiedRegisters.Contains(ir.Z80_IX) {
		g.emit("    POP IX")
	}
	
	if fn.ModifiedRegisters.Contains(ir.Z80_BC | ir.Z80_DE | ir.Z80_HL) {
		g.emit("    EXX           ; Restore BC, DE, HL")
	}
	
	if fn.ModifiedRegisters.Contains(ir.Z80_AF) {
		g.emit("    EX AF, AF'    ; Restore AF")
	}
	
	g.emit("    EI            ; Re-enable interrupts")
	g.emit("    RETI          ; Return from interrupt")
}

// generateInstruction generates code for a single IR instruction
func (g *Z80Generator) generateInstruction(inst ir.Instruction) error {
	// Add comment for instruction
	if inst.Comment == "" {
		g.emit("    ; %s", inst.String())
	} else {
		g.emit("    ; %s", inst.Comment)
	}

	switch inst.Op {
	case ir.OpNop:
		g.emit("    NOP")
		
	case ir.OpLabel:
		g.emit("%s:", g.sanitizeLabel(inst.Label))
		
	case ir.OpJump:
		g.emit("    JP %s", g.sanitizeLabel(inst.Label))
		
	case ir.OpJumpIf:
		// Load condition to A
		g.loadToA(inst.Src1)
		g.emit("    OR A")
		g.emit("    JP NZ, %s", g.sanitizeLabel(inst.Label))
		
	case ir.OpJumpIfNot:
		// Load condition to A
		g.loadToA(inst.Src1)
		g.emit("    OR A")
		g.emit("    JP Z, %s", g.sanitizeLabel(inst.Label))
		
	case ir.OpJumpIfZero:
		// Load value to A and test if zero
		g.loadToA(inst.Src1)
		g.emit("    OR A")
		g.emit("    JP Z, %s", inst.Symbol)
		
	case ir.OpJumpIfNotZero:
		// Load value to A and test if not zero
		g.loadToA(inst.Src1)
		g.emit("    OR A")
		g.emit("    JP NZ, %s", inst.Symbol)
		
	case ir.OpReturn:
		if inst.Src1 != 0 {
			// Check if this function has direct return optimization
			if target, ok := g.currentFunc.GetMetadata("direct_return_target"); ok {
				// Directly store to the target location instead of returning in HL
				g.loadToHL(inst.Src1)
				g.emit("    LD (%s), HL    ; Direct return optimization", target)
			} else {
				// Normal return: Load return value to HL (Z80 convention)
				g.loadToHL(inst.Src1)
			}
		}
		g.generateEpilogue()
		
	case ir.OpLoadConst:
		// Track the constant value for optimization
		g.constantValues[inst.Dest] = inst.Imm
		if debug {
			fmt.Printf("DEBUG: OpLoadConst - tracked r%d = %d\n", inst.Dest, inst.Imm)
			fmt.Printf("DEBUG: Current constants map: %v\n", g.constantValues)
		}
		
		// Load constant to register
		if inst.Imm < 256 {
			g.emit("    LD A, %d", inst.Imm)
			g.storeFromA(inst.Dest)
		} else {
			g.emit("    LD HL, %d", inst.Imm)
			g.storeFromHL(inst.Dest)
		}
		
	case ir.OpSMCLoadConst:
		// Self-modifying code: load constant that can be modified
		if inst.SMCLabel != "" {
			g.emit("%s:", inst.SMCLabel)
		}
		if inst.Imm < 256 {
			g.emit("    LD A, %d      ; SMC constant", inst.Imm)
			g.storeFromA(inst.Dest)
		} else {
			g.emit("    LD HL, %d     ; SMC constant", inst.Imm)
			g.storeFromHL(inst.Dest)
		}
		
	case ir.OpSMCStoreConst:
		// Self-modifying code: modify a previous SMC constant
		// Src1 contains the new value
		// SMCTarget contains the label of the instruction to modify
		g.loadToHL(inst.Src1)
		g.emit("    LD (%s+1), HL ; Modify SMC constant", inst.SMCTarget)
		// For 8-bit values, only modify the low byte
		if inst.Type != nil && inst.Type.Size() == 1 {
			g.emit("    LD A, L")
			g.emit("    LD (%s+1), A  ; Modify SMC 8-bit constant", inst.SMCTarget)
		}
		
	case ir.OpLoadVar:
		// First, determine the type of the variable
		var varType ir.Type
		var localReg ir.Register
		
		// Check if this is a global variable by symbol name
		if inst.Symbol != "" {
			// Look up global variable
			globalAddr := g.getGlobalAddr(inst.Symbol)
			if globalAddr != 0 {
				// For now, assume 16-bit for globals
				varType = &ir.BasicType{Kind: ir.TypeU16}
			} else {
				// Try to find local variable by name
				for _, local := range g.currentFunc.Locals {
					if local.Name == inst.Symbol {
						localReg = local.Reg
						varType = local.Type
						break
					}
				}
			}
		} else {
			// Local variable by register
			localReg = inst.Src1
			// Find type from locals
			for _, local := range g.currentFunc.Locals {
				if local.Reg == inst.Src1 {
					varType = local.Type
					break
				}
			}
		}
		
		// Load value based on type
		isU8 := false
		if basicType, ok := varType.(*ir.BasicType); ok {
			isU8 = basicType.Kind == ir.TypeU8 || basicType.Kind == ir.TypeI8
		}
		
		if isU8 {
			// For 8-bit values, load to A
			if inst.Symbol != "" {
				globalAddr := g.getGlobalAddr(inst.Symbol)
				if globalAddr != 0 {
					g.emit("    LD A, ($%04X)", globalAddr)
				} else {
					// Local variable
					if g.useAbsoluteLocals {
						addr := g.getAbsoluteAddr(localReg)
						g.emit("    LD A, ($%04X)", addr)
					} else {
						offset := g.getLocalOffset(localReg)
						g.emit("    LD A, (IX%+d)", offset)
					}
				}
			} else {
				// Local variable
				if g.useAbsoluteLocals {
					addr := g.getAbsoluteAddr(inst.Src1)
					g.emit("    LD A, ($%04X)", addr)
				} else {
					offset := g.getLocalOffset(inst.Src1)
					g.emit("    LD A, (IX%+d)", offset)
				}
			}
			g.storeFromA(inst.Dest)
		} else {
			// For 16-bit values, load to HL
			if inst.Symbol != "" {
				globalAddr := g.getGlobalAddr(inst.Symbol)
				if globalAddr != 0 {
					g.emit("    LD HL, ($%04X)", globalAddr)
				} else {
					// Local variable
					if g.useAbsoluteLocals {
						addr := g.getAbsoluteAddr(localReg)
						g.emit("    LD HL, ($%04X)", addr)
					} else {
						offset := g.getLocalOffset(localReg)
						g.emit("    LD L, (IX%+d)", offset)
						g.emit("    LD H, (IX%+d)", offset+1)
					}
				}
			} else {
				// Local variable
				if g.useAbsoluteLocals {
					addr := g.getAbsoluteAddr(inst.Src1)
					g.emit("    LD HL, ($%04X)", addr)
				} else {
					offset := g.getLocalOffset(inst.Src1)
					g.emit("    LD L, (IX%+d)", offset)
					g.emit("    LD H, (IX%+d)", offset+1)
				}
			}
			g.storeFromHL(inst.Dest)
		}
		
	case ir.OpStoreVar:
		// Store to variable
		// First, determine the type of the variable
		var varType ir.Type
		var localReg ir.Register
		
		// Check if this is a global variable by symbol name
		if inst.Symbol != "" {
			// Look up global variable
			globalAddr := g.getGlobalAddr(inst.Symbol)
			if globalAddr != 0 {
				// For now, assume 16-bit for globals
				varType = &ir.BasicType{Kind: ir.TypeU16}
			} else {
				// Try to find local variable by name
				for _, local := range g.currentFunc.Locals {
					if local.Name == inst.Symbol {
						localReg = local.Reg
						varType = local.Type
						break
					}
				}
			}
		} else {
			// Local variable by register
			localReg = inst.Dest
			// Find type from locals
			for _, local := range g.currentFunc.Locals {
				if local.Reg == inst.Dest {
					varType = local.Type
					break
				}
			}
		}
		
		// Load value based on type
		isU8 := false
		if basicType, ok := varType.(*ir.BasicType); ok {
			isU8 = basicType.Kind == ir.TypeU8 || basicType.Kind == ir.TypeI8
		}
		
		if isU8 {
			// For 8-bit values, load to A
			if inst.Src1 != ir.RegZero {
				g.loadToA(inst.Src1)
			}
			
			// Store 8-bit value
			if inst.Symbol != "" {
				globalAddr := g.getGlobalAddr(inst.Symbol)
				if globalAddr != 0 {
					g.emit("    LD ($%04X), A", globalAddr)
				} else {
					// Local variable
					if g.useAbsoluteLocals {
						addr := g.getAbsoluteAddr(localReg)
						g.emit("    LD ($%04X), A", addr)
					} else {
						offset := g.getLocalOffset(localReg)
						g.emit("    LD (IX%+d), A", offset)
					}
				}
			} else {
				// Local variable
				if g.useAbsoluteLocals {
					addr := g.getAbsoluteAddr(inst.Dest)
					g.emit("    LD ($%04X), A", addr)
				} else {
					offset := g.getLocalOffset(inst.Dest)
					g.emit("    LD (IX%+d), A", offset)
				}
			}
		} else {
			// For 16-bit values, load to HL
			if inst.Src1 != ir.RegZero {
				g.loadToHL(inst.Src1)
			}
			
			// Store 16-bit value
			if inst.Symbol != "" {
				globalAddr := g.getGlobalAddr(inst.Symbol)
				if globalAddr != 0 {
					g.emit("    LD ($%04X), HL", globalAddr)
				} else {
					// Local variable
					if g.useAbsoluteLocals {
						addr := g.getAbsoluteAddr(localReg)
						g.emit("    LD ($%04X), HL", addr)
					} else {
						offset := g.getLocalOffset(localReg)
						g.emit("    LD (IX%+d), L", offset)
						g.emit("    LD (IX%+d), H", offset+1)
					}
				}
			} else {
				// Local variable
				if g.useAbsoluteLocals {
					addr := g.getAbsoluteAddr(inst.Dest)
					g.emit("    LD ($%04X), HL", addr)
				} else {
					offset := g.getLocalOffset(inst.Dest)
					g.emit("    LD (IX%+d), L", offset)
					g.emit("    LD (IX%+d), H", offset+1)
				}
			}
		}
		
	case ir.OpStoreTSMCRef:
		// Store to TSMC reference immediate operand
		// This modifies the immediate field of the instruction that loads the parameter
		g.loadToHL(inst.Src1)
		
		// The label for the immediate operand is paramName$imm0
		immLabel := fmt.Sprintf("%s$imm0", inst.Symbol)
		g.emit("    LD (%s), HL    ; Update TSMC reference immediate", immLabel)
		
	case ir.OpMove:
		// Move from source to destination register
		g.loadToHL(inst.Src1)
		g.storeFromHL(inst.Dest)
		
	case ir.OpAdd:
		// Load operands efficiently
		g.loadToHL(inst.Src1)
		g.emit("    LD D, H")
		g.emit("    LD E, L")
		g.loadToHL(inst.Src2)
		g.emit("    ADD HL, DE")
		g.storeFromHL(inst.Dest)
		
	case ir.OpSub:
		// HL = Src1 - Src2
		// Optimal: load Src1 to HL, Src2 to DE, then subtract
		g.loadToHL(inst.Src1)
		g.loadToDE(inst.Src2)
		g.emit("    OR A          ; Clear carry")
		g.emit("    SBC HL, DE    ; HL = Src1 - Src2")
		g.storeFromHL(inst.Dest)
		
	case ir.OpNeg:
		// Negate the value (two's complement)
		g.loadToHL(inst.Src1)
		// Check if 8-bit or 16-bit based on type
		if inst.Type != nil {
			if basicType, ok := inst.Type.(*ir.BasicType); ok {
				switch basicType.Kind {
				case ir.TypeI8, ir.TypeU8:
					// 8-bit negation
					g.emit("    LD A, L       ; Get low byte")
					g.emit("    NEG           ; Negate A")
					g.emit("    LD L, A       ; Store back")
					g.emit("    LD H, 0       ; Clear high byte")
				case ir.TypeI16, ir.TypeU16:
					// 16-bit negation
					g.emit("    XOR A         ; Clear A")
					g.emit("    SUB L         ; 0 - L")
					g.emit("    LD L, A")
					g.emit("    LD A, 0")
					g.emit("    SBC A, H      ; 0 - H with borrow")
					g.emit("    LD H, A")
				default:
					// Default to 16-bit
					g.emit("    XOR A         ; Clear A")
					g.emit("    SUB L         ; 0 - L")
					g.emit("    LD L, A")
					g.emit("    LD A, 0")
					g.emit("    SBC A, H      ; 0 - H with borrow")
					g.emit("    LD H, A")
				}
			} else {
				// Default to 16-bit negation
				g.emit("    XOR A         ; Clear A")
				g.emit("    SUB L         ; 0 - L")
				g.emit("    LD L, A")
				g.emit("    LD A, 0")
				g.emit("    SBC A, H      ; 0 - H with borrow")
				g.emit("    LD H, A")
			}
		} else {
			// Default to 16-bit negation
			g.emit("    XOR A         ; Clear A")
			g.emit("    SUB L         ; 0 - L")
			g.emit("    LD L, A")
			g.emit("    LD A, 0")
			g.emit("    SBC A, H      ; 0 - H with borrow")
			g.emit("    LD H, A")
		}
		g.storeFromHL(inst.Dest)
		
	case ir.OpMul:
		// Check for constant optimization opportunity
		var constMultiplier int64
		var hasConstant bool
		var constOnSrc2 bool
		
		// Check if either operand is a constant
		if val, ok := g.constantValues[inst.Src2]; ok {
			constMultiplier = val
			hasConstant = true
			constOnSrc2 = true
			if debug {
				fmt.Printf("DEBUG: Found constant %d in Src2 (r%d)\n", val, inst.Src2)
			}
		} else if val, ok := g.constantValues[inst.Src1]; ok {
			constMultiplier = val
			hasConstant = true
			constOnSrc2 = false
			if debug {
				fmt.Printf("DEBUG: Found constant %d in Src1 (r%d)\n", val, inst.Src1)
			}
		}
		
		if debug && !hasConstant {
			fmt.Printf("DEBUG: No constant found for multiplication. Src1=r%d, Src2=r%d\n", inst.Src1, inst.Src2)
			fmt.Printf("DEBUG: Known constants: %v\n", g.constantValues)
		}
		
		// Clear destination from constant tracking (it's computed, not constant)
		delete(g.constantValues, inst.Dest)
		
		// Determine if this is 16-bit
		is16bit := false
		if inst.Type != nil {
			if basicType, ok := inst.Type.(*ir.BasicType); ok {
				is16bit = (basicType.Kind == ir.TypeU16 || basicType.Kind == ir.TypeI16)
			}
		}
		
		// Try optimization if we have a constant
		if hasConstant && canOptimizeMultiplication(constMultiplier) {
			// Load the variable operand
			if constOnSrc2 {
				if is16bit {
					g.loadToHL(inst.Src1)
				} else {
					g.loadToA(inst.Src1)
				}
			} else {
				if is16bit {
					g.loadToHL(inst.Src2)
				} else {
					g.loadToA(inst.Src2)
				}
			}
			
			// Emit optimized multiplication
			g.emitOptimizedMultiplication(constMultiplier, is16bit)
			
			// Store result
			if is16bit {
				g.storeFromHL(inst.Dest)
			} else {
				// Result is in A, need to move to HL for storage
				g.emit("    LD L, A")
				g.emit("    LD H, 0")
				g.storeFromHL(inst.Dest)
			}
			break
		}
		
		// Fall back to original loop-based multiplication
		if is16bit {
			// 16-bit multiplication using repeated addition
			g.emit("    ; 16-bit multiplication")
			g.loadToHL(inst.Src1)
			g.emit("    LD (mul_src1_%d), HL  ; Save multiplicand", g.labelCounter)
			g.loadToHL(inst.Src2)
			g.emit("    LD (mul_src2_%d), HL  ; Save multiplier", g.labelCounter)
			g.emit("    LD HL, 0             ; Result = 0")
			g.emit("    LD DE, (mul_src1_%d)  ; DE = multiplicand", g.labelCounter)
			g.emit("    LD BC, (mul_src2_%d)  ; BC = multiplier", g.labelCounter)
			g.emit("    LD A, B")
			g.emit("    OR C                 ; Check if multiplier is 0")
			g.emit("    JR Z, .mul16_done_%d", g.labelCounter)
			g.emit("%s:", g.getFunctionLabel("mul16_loop"))
			g.emit("    ADD HL, DE           ; Result += multiplicand")
			g.emit("    DEC BC")
			g.emit("    LD A, B")
			g.emit("    OR C")
			g.emit("    JR NZ, .mul16_loop_%d", g.labelCounter)
			g.emit("%s:", g.getFunctionLabel("mul16_done"))
			g.emit("mul_src1_%d: DW 0", g.labelCounter)
			g.emit("mul_src2_%d: DW 0", g.labelCounter)
			g.labelCounter++
			g.storeFromHL(inst.Dest)
		} else {
			// Default 8-bit multiplication
			g.emit("    ; 8-bit multiplication")
			g.loadToA(inst.Src1)
			g.emit("    LD B, A       ; B = multiplicand")
			g.loadToA(inst.Src2)
			g.emit("    LD C, A       ; C = multiplier")
			g.emit("    LD HL, 0      ; HL = result")
			g.emit("    LD A, C")
			g.emit("    OR A          ; Check if multiplier is 0")
			muldoneLabel := g.getFunctionLabel("mul_done")
			g.emit("    JR Z, %s", muldoneLabel)
			g.emit("%s:", g.getFunctionLabel("mul_loop"))
			g.emit("    LD D, 0")
			g.emit("    LD E, B")
			g.emit("    ADD HL, DE    ; Add multiplicand to result")
			g.emit("    DEC C")
			g.emit("    JR NZ, %s", g.getFunctionLabel("mul_loop"))
			g.emit("%s:", g.getFunctionLabel("mul_done"))
			g.labelCounter++
			g.storeFromHL(inst.Dest)
		}
		
	case ir.OpDiv:
		// 8-bit division using repeated subtraction
		// Src1 / Src2 -> Dest
		g.emit("    ; 8-bit division")
		g.loadToA(inst.Src1)
		g.emit("    LD D, A       ; D = dividend")
		g.loadToA(inst.Src2)
		g.emit("    LD E, A       ; E = divisor")
		g.emit("    OR A          ; Check for divide by zero")
		divbyzeroLabel := g.getFunctionLabel("div_by_zero")
		g.emit("    JR Z, %s", divbyzeroLabel)
		g.emit("    LD B, 0       ; B = quotient")
		g.emit("    LD A, D       ; A = remainder")
		g.emit("%s:", g.getFunctionLabel("div_loop"))
		g.emit("    CP E          ; Compare remainder with divisor")
		divdoneLabel := g.getFunctionLabel("div_done")
		g.emit("    JR C, %s", divdoneLabel)
		g.emit("    SUB E         ; Subtract divisor")
		g.emit("    INC B         ; Increment quotient")
		g.emit("    JR %s", g.getFunctionLabel("div_loop"))
		g.emit("%s:", g.getFunctionLabel("div_by_zero"))
		g.emit("    LD B, 0       ; Return 0 for divide by zero")
		g.emit("%s:", divdoneLabel)
		g.emit("    LD L, B       ; Result in L")
		g.emit("    LD H, 0")
		g.labelCounter++
		g.storeFromHL(inst.Dest)
		
	case ir.OpMod:
		// Modulo operation - remainder after division
		// Src1 % Src2 -> Dest
		g.emit("    ; 8-bit modulo")
		g.loadToA(inst.Src1)
		g.emit("    LD D, A       ; D = dividend")
		g.loadToA(inst.Src2)
		g.emit("    LD E, A       ; E = divisor")
		g.emit("    OR A          ; Check for divide by zero")
		modbyzeroLabel := g.getFunctionLabel("mod_by_zero")
		g.emit("    JR Z, %s", modbyzeroLabel)
		g.emit("    LD A, D       ; A = dividend")
		g.emit("%s:", g.getFunctionLabel("mod_loop"))
		g.emit("    CP E          ; Compare with divisor")
		moddoneLabel := g.getFunctionLabel("mod_done")
		g.emit("    JR C, %s", moddoneLabel)
		g.emit("    SUB E         ; Subtract divisor")
		g.emit("    JR %s", g.getFunctionLabel("mod_loop"))
		g.emit("%s:", g.getFunctionLabel("mod_by_zero"))
		g.emit("    LD A, 0       ; Return 0 for modulo by zero")
		g.emit("%s:", moddoneLabel)
		g.emit("    LD L, A       ; Result (remainder) in L")
		g.emit("    LD H, 0")
		g.labelCounter++
		g.storeFromHL(inst.Dest)
		
	case ir.OpInc:
		// Increment register
		if inst.Type != nil && inst.Type.Size() == 1 {
			// For byte values
			g.loadToA(inst.Src1)
			g.emit("    INC A")
			g.storeFromA(inst.Dest)
		} else {
			// For word values
			g.loadToHL(inst.Src1)
			g.emit("    INC HL")
			g.storeFromHL(inst.Dest)
		}
		
	case ir.OpDec:
		// Check for DJNZ optimization pattern
		if g.canOptimizeToDJNZ(inst) {
			return g.generateDJNZ(inst)
		}
		
		// Decrement register
		if inst.Type != nil && inst.Type.Size() == 1 {
			// For byte values
			g.loadToA(inst.Src1)
			g.emit("    DEC A")
			g.storeFromA(inst.Dest)
		} else {
			// For word values
			g.loadToHL(inst.Src1)
			g.emit("    DEC HL")
			g.storeFromHL(inst.Dest)
		}
		
	case ir.OpAnd:
		// Bitwise AND
		g.loadToHL(inst.Src1)
		g.emit("    LD D, H")
		g.emit("    LD E, L")
		g.loadToHL(inst.Src2)
		g.emit("    LD A, L")
		g.emit("    AND E")
		g.emit("    LD L, A")
		g.emit("    LD A, H")
		g.emit("    AND D")
		g.emit("    LD H, A")
		g.storeFromHL(inst.Dest)
		
	case ir.OpOr:
		// Bitwise OR
		g.loadToHL(inst.Src1)
		g.emit("    LD D, H")
		g.emit("    LD E, L")
		g.loadToHL(inst.Src2)
		g.emit("    LD A, L")
		g.emit("    OR E")
		g.emit("    LD L, A")
		g.emit("    LD A, H")
		g.emit("    OR D")
		g.emit("    LD H, A")
		g.storeFromHL(inst.Dest)
		
	case ir.OpLogicalAnd:
		// Logical AND with short-circuit evaluation
		// First operand: if false (0), result is false
		g.loadToA(inst.Src1)
		g.emit("    OR A           ; Test if zero")
		falseLabel := g.uniqueLabel("land_false")
		endLabel := g.uniqueLabel("land_end")
		g.emit("    JR Z, %s       ; Skip if first operand is false", falseLabel)
		
		// Second operand: if false (0), result is false
		g.loadToA(inst.Src2)
		g.emit("    OR A           ; Test if zero")
		g.emit("    JR Z, %s       ; Skip if second operand is false", falseLabel)
		
		// Both true - result is true (1)
		g.emit("    LD A, 1        ; Result is true")
		g.emit("    JR %s", endLabel)
		
		// False path
		g.emit("%s:", falseLabel)
		g.emit("    XOR A          ; Result is false (0)")
		
		g.emit("%s:", endLabel)
		g.storeFromA(inst.Dest)
		
	case ir.OpLogicalOr:
		// Logical OR with short-circuit evaluation
		// First operand: if true (non-zero), result is true
		g.loadToA(inst.Src1)
		g.emit("    OR A           ; Test if zero")
		trueLabel := g.uniqueLabel("lor_true")
		endLabel := g.uniqueLabel("lor_end")
		g.emit("    JR NZ, %s      ; Skip if first operand is true", trueLabel)
		
		// Second operand: if true (non-zero), result is true
		g.loadToA(inst.Src2)
		g.emit("    OR A           ; Test if zero")
		g.emit("    JR NZ, %s      ; Skip if second operand is true", trueLabel)
		
		// Both false - result is false (0)
		g.emit("    XOR A          ; Result is false")
		g.emit("    JR %s", endLabel)
		
		// True path
		g.emit("%s:", trueLabel)
		g.emit("    LD A, 1        ; Result is true (1)")
		
		g.emit("%s:", endLabel)
		g.storeFromA(inst.Dest)
		
	case ir.OpXor:
		// Bitwise XOR
		// Special case for XOR with self (zeroing)
		if inst.Src1 == inst.Src2 && inst.Src1 == inst.Dest {
			// XOR A,A is a common way to zero A register
			g.emit("    XOR A")
			g.storeFromA(inst.Dest)
		} else {
			g.loadToHL(inst.Src1)
			g.emit("    LD D, H")
			g.emit("    LD E, L")
			g.loadToHL(inst.Src2)
			g.emit("    LD A, L")
			g.emit("    XOR E")
			g.emit("    LD L, A")
			g.emit("    LD A, H")
			g.emit("    XOR D")
			g.emit("    LD H, A")
			g.storeFromHL(inst.Dest)
		}
		
	case ir.OpShl:
		// Shift left
		// Check if 16-bit or 8-bit based on type
		if inst.Type != nil {
			if basicType, ok := inst.Type.(*ir.BasicType); ok && 
			   (basicType.Kind == ir.TypeU16 || basicType.Kind == ir.TypeI16) {
				// 16-bit shift left
				g.emit("    ; 16-bit shift left")
				g.loadToHL(inst.Src1)
				g.loadToA(inst.Src2)
				g.emit("    LD B, A       ; B = shift count")
				g.emit("    OR A")
				g.emit("    JR Z, .shl16_done_%d", g.labelCounter)
				g.emit("%s:", g.getFunctionLabel("shl16_loop"))
				g.emit("    ADD HL, HL    ; Shift left by 1")
				g.emit("    DJNZ .shl16_loop_%d", g.labelCounter)
				g.emit("%s:", g.getFunctionLabel("shl16_done"))
				g.labelCounter++
				g.storeFromHL(inst.Dest)
				break
			}
		}
		
		// Default 8-bit shift left
		g.emit("    ; Shift left")
		g.loadToA(inst.Src1)
		g.emit("    LD B, A       ; B = value to shift")
		g.loadToA(inst.Src2)
		g.emit("    LD C, A       ; C = shift count")
		g.emit("    LD A, B       ; A = value")
		g.emit("    OR A          ; Clear carry")
		shldoneLabel := g.getFunctionLabel("shl_done")
		g.emit("    JR Z, %s", shldoneLabel)
		g.emit("    LD B, C       ; B = counter")
		g.emit("%s:", g.getFunctionLabel("shl_loop"))
		g.emit("    DEC B")
		g.emit("    JP M, %s", shldoneLabel)
		g.emit("    SLA A         ; Shift left, 0 into bit 0")
		g.emit("    JR %s", g.getFunctionLabel("shl_loop"))
		g.emit("%s:", g.getFunctionLabel("shl_done"))
		g.emit("    LD L, A")
		g.emit("    LD H, 0")
		g.labelCounter++
		g.storeFromHL(inst.Dest)
		
	case ir.OpShr:
		// Shift right (logical)
		// Check if 16-bit or 8-bit based on type
		if inst.Type != nil {
			if basicType, ok := inst.Type.(*ir.BasicType); ok && 
			   (basicType.Kind == ir.TypeU16 || basicType.Kind == ir.TypeI16) {
				// 16-bit shift right
				g.emit("    ; 16-bit shift right")
				g.loadToHL(inst.Src1)
				g.loadToA(inst.Src2)
				g.emit("    LD B, A       ; B = shift count")
				g.emit("    OR A")
				g.emit("    JR Z, .shr16_done_%d", g.labelCounter)
				g.emit("%s:", g.getFunctionLabel("shr16_loop"))
				g.emit("    SRL H         ; Shift high byte right")
				g.emit("    RR L          ; Rotate right through carry")
				g.emit("    DJNZ .shr16_loop_%d", g.labelCounter)
				g.emit("%s:", g.getFunctionLabel("shr16_done"))
				g.labelCounter++
				g.storeFromHL(inst.Dest)
				break
			}
		}
		
		// Default 8-bit shift right
		g.emit("    ; Shift right")
		g.loadToA(inst.Src1)
		g.emit("    LD B, A       ; B = value to shift")
		g.loadToA(inst.Src2)
		g.emit("    LD C, A       ; C = shift count")
		g.emit("    LD A, B       ; A = value")
		g.emit("    OR A          ; Clear carry")
		shrdoneLabel := g.getFunctionLabel("shr_done")
		g.emit("    JR Z, %s", shrdoneLabel)
		g.emit("    LD B, C       ; B = counter")
		g.emit("%s:", g.getFunctionLabel("shr_loop"))
		g.emit("    DEC B")
		g.emit("    JP M, %s", shrdoneLabel)
		g.emit("    SRL A         ; Shift right, 0 into bit 7")
		g.emit("    JR %s", g.getFunctionLabel("shr_loop"))
		g.emit("%s:", g.getFunctionLabel("shr_done"))
		g.emit("    LD L, A")
		g.emit("    LD H, 0")
		g.labelCounter++
		g.storeFromHL(inst.Dest)
		
	case ir.OpNot:
		// Bitwise NOT (one's complement)
		// Check if 16-bit or 8-bit based on type
		if inst.Type != nil {
			if basicType, ok := inst.Type.(*ir.BasicType); ok && 
			   (basicType.Kind == ir.TypeU16 || basicType.Kind == ir.TypeI16) {
				// 16-bit NOT
				g.loadToHL(inst.Src1)
				g.emit("    LD A, L")
				g.emit("    CPL           ; Complement low byte")
				g.emit("    LD L, A")
				g.emit("    LD A, H")
				g.emit("    CPL           ; Complement high byte")
				g.emit("    LD H, A")
				g.storeFromHL(inst.Dest)
			} else {
				// 8-bit NOT
				g.loadToA(inst.Src1)
				g.emit("    CPL           ; Complement A")
				g.storeFromA(inst.Dest)
			}
		} else {
			// Default to 8-bit
			g.loadToA(inst.Src1)
			g.emit("    CPL           ; Complement A")
			g.storeFromA(inst.Dest)
		}
		
	case ir.OpEq, ir.OpNe, ir.OpLt, ir.OpGt, ir.OpLe, ir.OpGe:
		g.generateComparison(inst)
		
	case ir.OpCall:
		// Check if calling a TRUE SMC function
		g.emit("    ; Call to %s (args: %d)", inst.Symbol, len(inst.Args))
		targetFunc := g.findFunction(inst.Symbol)
		
		// Prepare arguments before the call
		if len(inst.Args) > 0 {
			g.prepareCallArguments(inst.Args, targetFunc)
		}
		
		if targetFunc != nil {
			g.emit("    ; Found function, UsesTrueSMC=%v", targetFunc.UsesTrueSMC)
			if targetFunc.UsesTrueSMC {
				// Generate TRUE SMC patching before call
				g.generateTrueSMCCall(inst, targetFunc)
			} else {
				// Use the full function name (including module prefix)
				// Z80 assembler will handle the dots in the label
				g.emit("    CALL %s", targetFunc.Name)
			}
		} else {
			// Function not found in current module - might be external
			// Use the symbol as-is
			g.emit("    CALL %s", inst.Symbol)
		}
		// Result is in HL
		g.storeFromHL(inst.Dest)
		
	case ir.OpPatchPoint:
		// Define a patchable instruction sequence
		return g.generatePatchPoint(&inst)
		
	case ir.OpPatchTemplate:
		// Select template for patchable instruction sequence
		return g.generatePatchTemplate(&inst)
		
	case ir.OpPatchTarget:
		// Set target address for store operations in patch
		return g.generatePatchTarget(&inst)
		
	case ir.OpPatchParam:
		// Patch function parameter immediate
		return g.generatePatchParam(&inst)
	
	case ir.OpAlloc:
		// Allocate memory on stack
		// For now, just reserve space by adjusting SP
		g.emit("    LD HL, -%d", inst.Imm)
		g.emit("    ADD HL, SP")
		g.emit("    LD SP, HL")
		// Return pointer in result register
		g.emit("    EX DE, HL")
		g.emit("    LD HL, SP")
		g.storeFromHL(inst.Dest)
		
	case ir.OpLoad:
		// Load through pointer
		// Src1 = pointer
		g.loadToHL(inst.Src1)
		// Check type size
		if inst.Type != nil && inst.Type.Size() == 1 {
			// 8-bit load
			g.emit("    LD A, (HL)")
			g.storeFromA(inst.Dest)
		} else {
			// 16-bit load
			g.emit("    LD E, (HL)")
			g.emit("    INC HL")
			g.emit("    LD D, (HL)")
			g.emit("    EX DE, HL")
			g.storeFromHL(inst.Dest)
		}
		
	case ir.OpStore:
		// Store through pointer
		// Src1 = pointer, Src2 = value
		g.loadToHL(inst.Src1)
		g.emit("    PUSH HL")
		// Check type size
		if inst.Type != nil && inst.Type.Size() == 1 {
			// 8-bit store
			g.loadToA(inst.Src2)
			g.emit("    POP HL")
			g.emit("    LD (HL), A")
		} else {
			// 16-bit store
			g.loadToHL(inst.Src2)
			g.emit("    POP DE")
			g.emit("    LD (DE), L")
			g.emit("    INC DE")
			g.emit("    LD (DE), H")
		}
		
	case ir.OpLoadField:
		// Load field from struct
		// Src1 = struct pointer, Imm = field offset
		g.loadToHL(inst.Src1)
		if inst.Imm > 0 {
			g.emit("    LD DE, %d", inst.Imm)
			g.emit("    ADD HL, DE")
		}
		// Load value at offset
		g.emit("    LD E, (HL)")
		g.emit("    INC HL")
		g.emit("    LD D, (HL)")
		g.emit("    EX DE, HL")
		g.storeFromHL(inst.Dest)
		
	case ir.OpStoreField:
		// Store to field in struct
		// Src1 = struct pointer, Src2 = value, Imm = field offset
		g.loadToHL(inst.Src1)
		if inst.Imm > 0 {
			g.emit("    LD DE, %d", inst.Imm)
			g.emit("    ADD HL, DE")
		}
		g.emit("    PUSH HL")
		g.loadToHL(inst.Src2)
		g.emit("    POP DE")
		// Store value at offset
		g.emit("    LD (DE), L")
		g.emit("    INC DE")
		g.emit("    LD (DE), H")
		
	case ir.OpLoadBitField:
		// Load bit field value
		// Src1 = source register containing bit struct
		// Imm = bit offset, Imm2 = bit width
		bitOffset := int(inst.Imm)
		bitWidth := int(inst.Imm2)
		
		// Load source value
		g.loadToA(inst.Src1)
		
		// Shift right to get field to LSB
		for i := 0; i < bitOffset; i++ {
			g.emit("    SRL A")
		}
		
		// Mask to get only the field bits
		mask := (1 << bitWidth) - 1
		g.emit("    AND %d", mask)
		
		// Store result
		g.storeFromA(inst.Dest)
		
	case ir.OpStoreBitField:
		// Store bit field value
		// Src1 = register containing bit struct (target)
		// Src2 = register containing value to store
		// Imm = bit offset, Imm2 = bit width
		bitOffset := int(inst.Imm)
		bitWidth := int(inst.Imm2)
		
		// First, load the current value
		g.loadToA(inst.Src1)
		g.emit("    LD B, A        ; Save original value")
		
		// Create mask for clearing the field bits
		fieldMask := ((1 << bitWidth) - 1) << bitOffset
		clearMask := ^fieldMask & 0xFF
		
		// Clear the field bits
		g.emit("    AND %d         ; Clear field bits", clearMask)
		g.emit("    LD C, A        ; Save cleared value")
		
		// Load the new value and prepare it
		g.loadToA(inst.Src2)
		
		// Mask to ensure value fits in field width
		valueMask := (1 << bitWidth) - 1
		g.emit("    AND %d         ; Mask to field width", valueMask)
		
		// Shift left to position
		for i := 0; i < bitOffset; i++ {
			g.emit("    SLA A          ; Shift to bit position")
		}
		
		// Combine with cleared value
		g.emit("    OR C           ; Combine with cleared original")
		
		// Store back
		g.storeFromA(inst.Src1)
		
	case ir.OpAsm:
		// Emit named label if provided
		if inst.AsmName != "" {
			g.emit("%s:", inst.AsmName)
		}
		
		// Process inline assembly code
		g.emitAsmBlock(inst.AsmCode)
		
	case ir.OpSetError:
		// Set carry flag and load error code to A
		g.emit("    ; Set error code and carry flag")
		g.loadToA(inst.Src1)
		g.emit("    SCF           ; Set carry flag (error)")
		// If we have a destination, store the error code
		if inst.Dest != 0 {
			g.storeFromA(inst.Dest)
		}
		
	case ir.OpCheckError:
		// Check carry flag - result is 1 if error (CY set), 0 if success
		g.emit("    ; Check carry flag for error")
		g.emit("    LD A, 0       ; Assume success")
		noerrorLabel := g.getFunctionLabel("no_error")
		g.emit("    JR NC, %s", noerrorLabel)
		g.emit("    INC A         ; Set to 1 if error")
		g.emit("%s:", noerrorLabel)
		g.labelCounter++
		g.storeFromA(inst.Dest)
		
	case ir.OpPrint:
		// Built-in print function - print a u8 character
		// Character is in Src1
		g.loadToA(inst.Src1)
		
		// Generate platform-specific print code
		switch g.targetPlatform {
		case "cpm":
			// CP/M uses BDOS function 2 (console output)
			g.emit("    LD E, A        ; Character to E")
			g.emit("    LD C, 2        ; BDOS function 2: console output")
			g.emit("    CALL 5         ; Call BDOS")
		case "msx":
			// MSX uses BIOS call at 0x00A2 (CHPUT)
			g.emit("    CALL $00A2     ; MSX BIOS CHPUT")
		case "cpc", "amstrad":
			// Amstrad CPC uses firmware call at 0xBB5A (TXT OUTPUT)
			g.emit("    CALL $BB5A     ; CPC TXT OUTPUT")
		default: // "zxspectrum" and others
			// ZX Spectrum uses RST 16
			g.emit("    RST 16         ; Print character in A")
		}
		
	case ir.OpPrintU8:
		// Print u8 as decimal number
		g.loadToA(inst.Src1)
		g.emit("    CALL print_u8_decimal")
		
	case ir.OpPrintU16:
		// Print u16 as decimal number
		g.loadToHL(inst.Src1)
		g.emit("    CALL print_u16_decimal")
		
	case ir.OpPrintI8:
		// Print i8 as signed decimal
		g.loadToA(inst.Src1)
		g.emit("    CALL print_i8_decimal")
		
	case ir.OpPrintI16:
		// Print i16 as signed decimal
		g.loadToHL(inst.Src1)
		g.emit("    CALL print_i16_decimal")
		
	case ir.OpPrintBool:
		// Print bool as "true" or "false"
		g.loadToA(inst.Src1)
		g.emit("    CALL print_bool")
		
	case ir.OpPrintString:
		// Print null-terminated string
		g.loadToHL(inst.Src1)
		g.emit("    CALL print_string")
		
	case ir.OpPrintStringDirect:
		// Direct print for short strings - ultra-fast!
		// Each character is loaded and printed directly
		if inst.Comment != "" {
			g.emit(fmt.Sprintf("    ; %s", inst.Comment))
		}
		
		// Generate platform-specific code for each character
		for _, ch := range inst.Symbol {
			g.emit(fmt.Sprintf("    LD A, %d", ch))
			
			switch g.targetPlatform {
			case "cpm":
				// CP/M uses BDOS function 2
				g.emit("    LD E, A        ; Character to E")
				g.emit("    LD C, 2        ; BDOS function 2")
				g.emit("    CALL 5         ; Call BDOS")
			case "msx":
				// MSX uses BIOS CHPUT
				g.emit("    CALL $00A2     ; MSX BIOS CHPUT")
			case "cpc", "amstrad":
				// Amstrad CPC uses TXT OUTPUT
				g.emit("    CALL $BB5A     ; CPC TXT OUTPUT")
			default: // "zxspectrum" and others
				// ZX Spectrum uses RST 16
				g.emit("    RST 16         ; Print character")
			}
		}
		
	case ir.OpLoadString:
		// Load address of string literal
		g.emit(fmt.Sprintf("    LD HL, %s", inst.Symbol))
		g.storeFromHL(inst.Dest)
		
	case ir.OpLen:
		// Built-in len function - get length of array/string
		// Array/string pointer is in Src1, result goes to Dest
		// For now, assume arrays store their length at offset -2
		g.loadToHL(inst.Src1)
		g.emit("    DEC HL")
		g.emit("    DEC HL         ; Point to length field")
		g.emit("    LD E, (HL)")
		g.emit("    INC HL")
		g.emit("    LD D, (HL)     ; Load 16-bit length")
		g.emit("    EX DE, HL      ; Result in HL")
		g.storeFromHL(inst.Dest)
		
	case ir.OpMemcpy:
		// Built-in memcpy - copy memory block
		// Src1 = dest, Src2 = src, Args[0] = size
		g.emit("    ; memcpy(dest, src, size)")
		// Load destination to DE
		g.loadToHL(inst.Src1)
		g.emit("    EX DE, HL      ; Dest in DE")
		// Load source to HL
		g.loadToHL(inst.Src2)
		// Load size to BC
		g.loadToHL(inst.Args[0])
		g.emit("    LD B, H")
		g.emit("    LD C, L        ; Size in BC")
		// Use LDIR for block copy
		g.emit("    LDIR           ; Copy BC bytes from HL to DE")
		
	case ir.OpMemset:
		// Built-in memset - set memory block
		// Src1 = dest, Src2 = value, Args[0] = size
		g.emit("    ; memset(dest, value, size)")
		// Load destination to HL
		g.loadToHL(inst.Src1)
		// Load value to A
		g.loadToA(inst.Src2)
		// Load size to BC
		g.loadToHL(inst.Args[0])
		g.emit("    LD B, H")
		g.emit("    LD C, L        ; Size in BC")
		// Fill memory
		g.emit("%s:", g.getFunctionLabel("memset_loop"))
		g.emit("    LD (HL), A     ; Store value")
		g.emit("    INC HL         ; Next address")
		g.emit("    DEC BC         ; Decrement count")
		g.emit("    LD D, B")
		g.emit("    OR C")
		g.emit("    JR NZ, %s", g.getFunctionLabel("memset_loop"))
		g.labelCounter++
		
	case ir.OpLoadLabel:
		// Load address of a label
		g.emit("    LD HL, %s", inst.Symbol)
		g.storeFromHL(inst.Dest)
		
	case ir.OpLoadIndex:
		// Load element from array
		// Src1 = array pointer, Src2 = index
		g.loadToHL(inst.Src1)
		// Save array pointer
		g.emit("    PUSH HL")
		// Load index to DE
		if inst.Type != nil && inst.Type.Size() == 1 {
			// For byte index, load to A first then to DE
			g.loadToA(inst.Src2)
			g.emit("    LD E, A")
			g.emit("    LD D, 0")
		} else {
			g.loadToDE(inst.Src2)
		}
		// Restore array pointer
		g.emit("    POP HL")
		// Multiply index by element size (assuming 1 byte elements for now)
		// TODO: Handle different element sizes
		g.emit("    ADD HL, DE")
		// Load value at array[index]
		g.emit("    LD A, (HL)")
		g.storeFromA(inst.Dest)
		
	// Loop operations
	case ir.OpLoadAddr:
		// Load address of a variable/array
		if inst.Symbol != "" {
			g.emit("    LD HL, %s", inst.Symbol)
		} else {
			// Load address from register (for arrays)
			g.loadToHL(inst.Src1)
		}
		g.storeFromHL(inst.Dest)
		
	case ir.OpCopyToBuffer:
		// Copy memory block to static buffer
		// Src1 = source pointer, Imm = buffer address, Imm2 = size
		g.loadToHL(inst.Src1)
		g.emit("    LD DE, $%04X    ; Buffer address", inst.Imm)
		g.emit("    LD BC, %d       ; Size", inst.Imm2)
		g.emit("    LDIR            ; Copy to buffer")
		
	case ir.OpCopyFromBuffer:
		// Copy static buffer back to memory
		// Dest = destination pointer, Imm = buffer address, Imm2 = size  
		g.loadToHL(inst.Dest)
		g.emit("    EX DE, HL       ; DE = destination")
		g.emit("    LD HL, $%04X    ; Buffer address", inst.Imm)
		g.emit("    LD BC, %d       ; Size", inst.Imm2)
		g.emit("    LDIR            ; Copy from buffer")
		
	case ir.OpDJNZ:
		// Decrement and jump if not zero
		// Uses B register for Z80's native DJNZ instruction
		g.loadToB(inst.Src1)
		g.emit("    DJNZ %s", inst.Label)
		// Store updated value back
		g.emit("    LD A, B")
		g.storeFromA(inst.Src1)
		
	case ir.OpLoadImm:
		// Load immediate value
		// Track the constant value for optimization
		g.constantValues[inst.Dest] = inst.Imm
		
		if inst.Imm <= 255 {
			g.emit("    LD A, %d", inst.Imm)
			g.storeFromA(inst.Dest)
		} else {
			g.emit("    LD HL, %d", inst.Imm)
			g.storeFromHL(inst.Dest)
		}
		
	case ir.OpAddImm:
		// Add immediate to register
		g.loadToHL(inst.Src1)
		g.emit("    LD DE, %d", inst.Imm)
		g.emit("    ADD HL, DE")
		g.storeFromHL(inst.Dest)
		
	case ir.OpCmp:
		// Compare two values (sets flags but no result)
		g.loadToHL(inst.Src1)
		g.emit("    LD D, H")
		g.emit("    LD E, L")
		g.loadToHL(inst.Src2)
		g.emit("    OR A      ; Clear carry")
		g.emit("    SBC HL, DE")
		
	case ir.OpTest:
		// Test register (sets flags without modifying)
		// Used to check if a value is zero/non-zero
		if inst.Type != nil && inst.Type.Size() == 1 {
			g.loadToA(inst.Src1)
			g.emit("    OR A           ; Test A (set flags)")
		} else {
			g.loadToHL(inst.Src1)
			g.emit("    LD A, H")
			g.emit("    OR L           ; Test HL (set flags)")
		}
		
	case ir.OpLoadDirect:
		// Load from direct memory address
		if inst.Type != nil && inst.Type.Size() == 1 {
			// For byte values, use A register
			g.emit("    LD A, ($%04X)", inst.Imm)
			g.storeFromA(inst.Dest)
		} else {
			// For word values, use HL register
			g.emit("    LD HL, ($%04X)", inst.Imm)
			g.storeFromHL(inst.Dest)
		}
		
	case ir.OpStoreDirect:
		// Store to direct memory address
		if inst.Type != nil && inst.Type.Size() == 1 {
			// For byte values, use A register
			g.loadToA(inst.Src1)
			g.emit("    LD ($%04X), A", inst.Imm)
		} else {
			// For word values, use HL register
			g.loadToHL(inst.Src1)
			g.emit("    LD ($%04X), HL", inst.Imm)
		}
		
	case ir.OpLoadPtr:
		// Load value through pointer (indirect load)
		// Src1 = pointer to load from
		g.loadToHL(inst.Src1)
		if inst.Type != nil && inst.Type.Size() == 1 {
			// For byte values
			g.emit("    LD A, (HL)")
			g.storeFromA(inst.Dest)
		} else {
			// For word values
			g.emit("    LD E, (HL)")
			g.emit("    INC HL")
			g.emit("    LD D, (HL)")
			g.emit("    EX DE, HL")
			g.storeFromHL(inst.Dest)
		}
		
	case ir.OpStorePtr:
		// Store value through pointer (indirect store)
		// Src1 = pointer to store to, Src2 = value to store
		g.loadToHL(inst.Src1)
		if inst.Type != nil && inst.Type.Size() == 1 {
			// For byte values
			g.emit("    PUSH HL")
			g.loadToA(inst.Src2)
			g.emit("    POP HL")
			g.emit("    LD (HL), A")
		} else {
			// For word values
			g.emit("    PUSH HL")
			g.loadToHL(inst.Src2)
			g.emit("    EX DE, HL")
			g.emit("    POP HL")
			g.emit("    LD (HL), E")
			g.emit("    INC HL")
			g.emit("    LD (HL), D")
		}
		
	case ir.OpAddr:
		// Address-of operator: get address of variable
		// Src1 = variable to get address of, Dest = register to store address
		reg := inst.Src1
		
		// Calculate the actual address of the variable
		addr := g.getAbsoluteAddr(reg)
		g.emit("    ; Address-of operation for register r%d", int(reg))
		g.emit("    LD HL, $%04X  ; Variable address", addr)
		g.storeFromHL(inst.Dest)
		
		
	case ir.OpStoreIndex:
		// Store element to array
		// Src1 = array pointer, Src2 = index, Imm = value to store (packed in immediate)
		// Note: This is a limitation - we need a third source operand
		// For now, assume the value is in a fixed location or use a workaround
		g.loadToHL(inst.Src1)
		// Save array pointer
		g.emit("    PUSH HL")
		// Load index
		if inst.Type != nil && inst.Type.Size() == 1 {
			// For byte arrays
			g.loadToA(inst.Src2)
			g.emit("    LD E, A")
			g.emit("    LD D, 0")
		} else {
			// For word arrays
			g.loadToDE(inst.Src2)
			// Multiply by 2 for word-sized elements
			g.emit("    SLA E")
			g.emit("    RL D")
		}
		// Restore array pointer and add index
		g.emit("    POP HL")
		g.emit("    ADD HL, DE")
		// Store value at array[index]
		// TODO: This needs the value source - for now using immediate
		if inst.Type != nil && inst.Type.Size() == 1 {
			g.emit("    LD (HL), %d    ; TODO: Need value source", inst.Imm)
		} else {
			g.emit("    LD (HL), %d    ; TODO: Need value source (low)", inst.Imm & 0xFF)
			g.emit("    INC HL")
			g.emit("    LD (HL), %d    ; TODO: Need value source (high)", (inst.Imm >> 8) & 0xFF)
		}
		
	case ir.OpLoadParam:
		// For non-SMC functions, parameters are already in registers/memory
		// Just need to move to the destination register
		g.emit("    ; Load parameter %s", inst.Symbol)
		// In the current implementation, parameters are loaded at function entry
		// This instruction is just a marker - the actual load happens in prologue
		
	case ir.OpArrayInit:
		// Initialize array
		// The array will be allocated on the stack or in absolute memory
		// This is just a marker, actual initialization happens with OpArrayElement
		if inst.Type != nil {
			if arrayType, ok := inst.Type.(*ir.ArrayType); ok {
				g.emit("    ; Initialize array with %d elements", arrayType.Length)
				// Allocate space for the array
				// size := arrayType.Length * arrayType.Element.Size() // TODO: Use when allocating
				if g.useAbsoluteLocals {
					// For absolute locals, allocate space in memory
					// Use the register allocator to track the memory location
					// g.regAlloc.AllocateLocal(inst.Dest, int(size)) // TODO: Implement
					addr := g.getAbsoluteAddr(inst.Dest)
					g.emit("    ; Array allocated at $%04X", addr)
					// Store the address in the register
					g.emit("    LD HL, $%04X", addr)
					g.storeFromHL(inst.Dest)
				} else {
					// Stack-based allocation
					g.emit("    ; Array allocated on stack")
					// Allocate space on the stack
					// g.regAlloc.AllocateLocal(inst.Dest, int(size)) // TODO: Implement
					offset := g.getLocalOffset(inst.Dest)
					g.emit("    ; Array at IX%+d", offset)
					// Calculate and store the address
					g.emit("    PUSH IX")
					g.emit("    POP HL")
					g.emit("    LD DE, %d", offset)
					g.emit("    ADD HL, DE")
					g.storeFromHL(inst.Dest)
				}
			}
		}
		
		
	case ir.OpCallIndirect:
		// Indirect function call through register (for lambdas)
		g.emit("    ; Indirect call through r%d", inst.Src1)
		
		// For lambda calls, pass parameters in registers
		// This works for MinZ lambdas which are typically simple
		if len(inst.Args) > 0 {
			g.emit("    ; Register-based parameter passing for lambda")
			// For now, support up to 1 parameter in A register
			if len(inst.Args) >= 1 {
				// First parameter goes in A (for u8) or HL (for u16)
				g.loadToHL(inst.Args[0])
				g.emit("    ; Parameter 0 in HL")
			}
			if len(inst.Args) >= 2 {
				// Second parameter would go in DE
				g.loadToDE(inst.Args[1])
				g.emit("    ; Parameter 1 in DE")
			}
			// Save parameters that will be used by lambda
			g.emit("    PUSH HL       ; Save parameter for lambda")
		}
		
		// Load function address into HL
		g.loadToHL(inst.Src1)
		
		// Restore parameters
		if len(inst.Args) > 0 {
			g.emit("    EX (SP), HL   ; Swap function address with parameter")
			g.emit("    EX DE, HL     ; Parameter in DE, function in HL")
			g.emit("    EX (SP), HL   ; Function address on stack, parameter in HL")
			g.emit("    POP DE        ; Function address in DE")
			g.emit("    EX DE, HL     ; Swap: function in HL, parameter in DE")
			g.emit("    PUSH DE       ; Push parameter")
		}
		
		// Make indirect call through HL
		callindirectLabel := g.getFunctionLabel("call_indirect")
		g.emit("    CALL %s", callindirectLabel)
		callindirectendLabel := g.getFunctionLabel("call_indirect_end")
		g.emit("    JR %s", callindirectendLabel)
		g.emit("%s:", callindirectLabel)
		g.emit("    JP (HL)       ; Jump to function address")
		g.emit("%s:", callindirectendLabel)
		g.labelCounter++
		
		// Clean up stack if needed
		if len(inst.Args) > 0 {
			g.emit("    POP DE        ; Clean up parameter")
		}
		
		// Result is in HL
		g.storeFromHL(inst.Dest)
		
	case ir.OpArrayElement:
		// Set array element during initialization
		// Dest = array register, Src1 = element value, Imm = index
		if g.useAbsoluteLocals {
			// Load array base address
			g.loadToHL(inst.Dest)
			g.emit("    PUSH HL        ; Save array base")
			
			// Calculate offset for element
			if inst.Type != nil && inst.Type.Size() == 2 {
				// 16-bit elements
				g.emit("    LD DE, %d", inst.Imm * 2)
			} else {
				// 8-bit elements (default)
				g.emit("    LD DE, %d", inst.Imm)
			}
			g.emit("    ADD HL, DE     ; Calculate element address")
			
			// Store the value
			if inst.Type != nil && inst.Type.Size() == 2 {
				// Store 16-bit value
				g.emit("    PUSH HL        ; Save element address")
				g.loadToHL(inst.Src1)
				g.emit("    EX DE, HL      ; Value in DE")
				g.emit("    POP HL         ; Restore element address")
				g.emit("    LD (HL), E")
				g.emit("    INC HL")
				g.emit("    LD (HL), D")
			} else {
				// Store 8-bit value
				g.emit("    PUSH HL        ; Save element address")
				g.loadToA(inst.Src1)
				g.emit("    POP HL         ; Restore element address")
				g.emit("    LD (HL), A")
			}
			
			g.emit("    POP HL         ; Restore array base")
		} else {
			// Stack-based array
			// Calculate the offset of the array element
			baseOffset := g.getLocalOffset(inst.Dest)
			elementOffset := baseOffset + int(inst.Imm) * int(inst.Type.Size())
			
			// Store the value
			if inst.Type != nil && inst.Type.Size() == 2 {
				// Store 16-bit value
				g.loadToHL(inst.Src1)
				g.emit("    LD (IX%+d), L", elementOffset)
				g.emit("    LD (IX%+d), H", elementOffset+1)
			} else {
				// Store 8-bit value
				g.loadToA(inst.Src1)
				g.emit("    LD (IX%+d), A", elementOffset)
			}
		}
		
		// These cases are already handled above
		if inst.Type != nil && inst.Type.Size() == 2 {
			// Store 16-bit value
			g.emit("    PUSH HL        ; Save field address")
			g.loadToHL(inst.Src2)
			g.emit("    EX DE, HL      ; Value in DE")
			g.emit("    POP HL         ; Restore field address")
			g.emit("    LD (HL), E")
			g.emit("    INC HL")
			g.emit("    LD (HL), D")
		} else {
			// Store 8-bit value
			g.emit("    PUSH HL        ; Save field address")
			g.loadToA(inst.Src2)
			g.emit("    POP HL         ; Restore field address")
			g.emit("    LD (HL), A")
		}
		
	default:
		return fmt.Errorf("unsupported opcode: %v (%d)", inst.Op, int(inst.Op))
	}

	return nil
}

// generateComparison generates code for comparison operations (OPTIMIZED VERSION)
func (g *Z80Generator) generateComparison(inst ir.Instruction) {
	// For comparisons, we need both operands in different registers
	// Optimal pattern: determine which operand to load first based on the operation
	
	switch inst.Op {
	case ir.OpEq:
		// Equality comparison - order doesn't matter
		// Optimal: load Src1 to HL, Src2 to DE, then compare
		g.loadToHL(inst.Src1)
		g.loadToDE(inst.Src2)
		g.emit("    OR A           ; Clear carry")
		g.emit("    SBC HL, DE     ; Compare Src1 - Src2")
		eqTrueLabel := g.getFunctionLabel("eq_true")
		eqDoneLabel := g.getFunctionLabel("eq_done")
		g.emit("    JP Z, %s", eqTrueLabel)
		g.emit("    LD HL, 0       ; False")
		g.emit("    JP %s", eqDoneLabel)
		g.emit("%s:", eqTrueLabel)
		g.emit("    LD HL, 1       ; True")
		g.emit("%s:", eqDoneLabel)
		g.labelCounter++
		g.storeFromHL(inst.Dest)
		
	case ir.OpNe:
		// Not equal
		// Optimal: load Src1 to HL, Src2 to DE, then compare
		g.loadToHL(inst.Src1)
		g.loadToDE(inst.Src2)
		g.emit("    OR A           ; Clear carry")
		g.emit("    SBC HL, DE     ; Compare Src1 - Src2")
		neTrueLabel := g.getFunctionLabel("ne_true")
		neDoneLabel := g.getFunctionLabel("ne_done")
		g.emit("    JP NZ, %s", neTrueLabel)
		g.emit("    LD HL, 0       ; False")
		g.emit("    JP %s", neDoneLabel)
		g.emit("%s:", neTrueLabel)
		g.emit("    LD HL, 1       ; True")
		g.emit("%s:", neDoneLabel)
		g.labelCounter++
		g.storeFromHL(inst.Dest)
		
	case ir.OpLt:
		// Less than: Src1 < Src2
		g.loadToHL(inst.Src1)
		g.loadToDE(inst.Src2)
		g.emit("    OR A           ; Clear carry")
		g.emit("    SBC HL, DE     ; Compare Src1 - Src2")
		ltTrueLabel := g.getFunctionLabel("lt_true")
		ltDoneLabel := g.getFunctionLabel("lt_done")
		g.emit("    JP M, %s", ltTrueLabel)
		g.emit("    LD HL, 0       ; False")
		g.emit("    JP %s", ltDoneLabel)
		g.emit("%s:", ltTrueLabel)
		g.emit("    LD HL, 1       ; True")
		g.emit("%s:", ltDoneLabel)
		g.labelCounter++
		g.storeFromHL(inst.Dest)
		
	case ir.OpGt:
		// Greater than: Src1 > Src2
		g.loadToHL(inst.Src1)
		g.loadToDE(inst.Src2)
		g.emit("    OR A           ; Clear carry")
		g.emit("    SBC HL, DE     ; Compare Src1 - Src2")
		gtCheckZeroLabel := g.getFunctionLabel("gt_check_zero")
		gtFalseLabel := g.getFunctionLabel("gt_false")
		gtDoneLabel := g.getFunctionLabel("gt_done")
		g.emit("    JP P, %s", gtCheckZeroLabel)
		g.emit("    LD HL, 0       ; False (negative)")
		g.emit("    JP %s", gtDoneLabel)
		g.emit("%s:", gtCheckZeroLabel)
		g.emit("    LD A, H")
		g.emit("    OR L           ; Check if result is zero")
		g.emit("    JP Z, %s", gtFalseLabel)
		g.emit("    LD HL, 1       ; True (positive and non-zero)")
		g.emit("    JP %s", gtDoneLabel)
		g.emit("%s:", gtFalseLabel)
		g.emit("    LD HL, 0       ; False (zero)")
		g.emit("%s:", gtDoneLabel)
		g.labelCounter++
		g.storeFromHL(inst.Dest)
		
	case ir.OpLe:
		// Less than or equal: Src1 <= Src2
		g.loadToHL(inst.Src1)
		g.loadToDE(inst.Src2)
		g.emit("    OR A           ; Clear carry")
		g.emit("    SBC HL, DE     ; Compare Src1 - Src2")
		leTrueLabel := g.getFunctionLabel("le_true")
		leDoneLabel := g.getFunctionLabel("le_done")
		g.emit("    JP M, %s", leTrueLabel)
		g.emit("    JP Z, %s", leTrueLabel)
		g.emit("    LD HL, 0       ; False")
		g.emit("    JP %s", leDoneLabel)
		g.emit("%s:", leTrueLabel)
		g.emit("    LD HL, 1       ; True")
		g.emit("%s:", leDoneLabel)
		g.labelCounter++
		g.storeFromHL(inst.Dest)
		
	case ir.OpGe:
		// Greater than or equal: Src1 >= Src2
		g.loadToHL(inst.Src1)
		g.loadToDE(inst.Src2)
		g.emit("    OR A           ; Clear carry")
		g.emit("    SBC HL, DE     ; Compare Src1 - Src2")
		geTrueLabel := g.getFunctionLabel("ge_true")
		geDoneLabel := g.getFunctionLabel("ge_done")
		g.emit("    JP P, %s", geTrueLabel)
		g.emit("    JP Z, %s", geTrueLabel)
		g.emit("    LD HL, 0       ; False")
		g.emit("    JP %s", geDoneLabel)
		g.emit("%s:", geTrueLabel)
		g.emit("    LD HL, 1       ; True")
		g.emit("%s:", geDoneLabel)
		g.labelCounter++
		g.storeFromHL(inst.Dest)
	}
}

// Register management helpers

// loadToA loads a virtual register to A
func (g *Z80Generator) loadToA(reg ir.Register) {
	if reg == ir.RegZero {
		g.emit("    XOR A")
		return
	}
	
	// Use hierarchical register allocation
	location, value := g.getRegisterLocation(reg)
	
	switch location {
	case LocationPhysical:
		physReg := value.(PhysicalReg)
		if physReg == RegA {
			// Already in A, no operation needed
			g.emit("    ; Register %d already in A", reg)
			return
		}
		// Move from physical register to A
		regName := g.physicalRegToAssembly(physReg)
		if physReg == RegBC || physReg == RegDE || physReg == RegHL {
			// 16-bit register, take low byte
			g.emit("    LD A, %s", regName[1:]) // BC->C, DE->E, HL->L
		} else {
			g.emit("    LD A, %s", regName)
		}
		
	case LocationShadow:
		physReg := value.(PhysicalReg)
		// Access shadow register (need to switch register set)
		if physReg == RegA_Shadow {
			g.emit("    EX AF, AF'        ; Switch to shadow A")
			g.emit("    ; Register %d now in A (shadow)", reg)
		} else {
			g.emit("    EXX               ; Switch to shadow registers")
			regName := g.physicalRegToAssembly(physReg)
			if physReg == RegBC_Shadow || physReg == RegDE_Shadow || physReg == RegHL_Shadow {
				g.emit("    LD A, %s         ; From shadow %s", regName[1:], regName)
			} else {
				g.emit("    LD A, %s         ; From shadow %s", regName, regName)
			}
			g.emit("    EXX               ; Switch back to main registers")
		}
		
	case LocationMemory:
		// Fallback to memory-based allocation
		addr := value.(uint16)
		if !g.useAbsoluteLocals && g.isLocalRegister(reg) {
			// Stack-based local variable - use IX+offset
			offset := g.getLocalOffset(reg)
			g.emit("    LD A, (IX%+d)     ; Virtual register %d from stack", offset, reg)
		} else {
			// Absolute addressing
			g.emit("    LD A, ($%04X)     ; Virtual register %d from memory", addr, reg)
		}
	}
}

// storeFromA stores A to a virtual register
func (g *Z80Generator) storeFromA(reg ir.Register) {
	// Use hierarchical register allocation
	location, value := g.getRegisterLocation(reg)
	
	switch location {
	case LocationPhysical:
		physReg := value.(PhysicalReg)
		if physReg == RegA {
			// Already in A, no operation needed
			g.emit("    ; Register %d already in A", reg)
			return
		}
		// Move from A to physical register
		regName := g.physicalRegToAssembly(physReg)
		if physReg == RegBC || physReg == RegDE || physReg == RegHL {
			// 16-bit register, store to low byte (need to preserve high byte)
			g.emit("    LD %s, A         ; Store to %s (low byte)", regName[1:], regName)
		} else {
			g.emit("    LD %s, A         ; Store to physical register %s", regName, regName)
		}
		
	case LocationShadow:
		physReg := value.(PhysicalReg)
		// Store to shadow register (need to switch register set)
		if physReg == RegA_Shadow {
			g.emit("    EX AF, AF'        ; Switch to shadow A")
			g.emit("    ; Register %d now stored in A (shadow)", reg)
		} else {
			g.emit("    EXX               ; Switch to shadow registers")
			regName := g.physicalRegToAssembly(physReg)
			if physReg == RegBC_Shadow || physReg == RegDE_Shadow || physReg == RegHL_Shadow {
				g.emit("    LD %s, A         ; Store to shadow %s", regName[1:], regName)
			} else {
				g.emit("    LD %s, A         ; Store to shadow %s", regName, regName)
			}
			g.emit("    EXX               ; Switch back to main registers")
		}
		
	case LocationMemory:
		// Fallback to memory-based allocation
		addr := value.(uint16)
		if !g.useAbsoluteLocals && g.isLocalRegister(reg) {
			// Stack-based local variable - use IX+offset
			offset := g.getLocalOffset(reg)
			g.emit("    LD (IX%+d), A     ; Virtual register %d to stack", offset, reg)
		} else {
			// Absolute addressing
			g.emit("    LD ($%04X), A     ; Virtual register %d to memory", addr, reg)
		}
	}
}

// loadToHL loads a virtual register to HL
func (g *Z80Generator) loadToHL(reg ir.Register) {
	if reg == ir.RegZero {
		g.emit("    LD HL, 0")
		return
	}
	
	// Use hierarchical register allocation for 16-bit loads
	location, value := g.getRegisterLocation(reg)
	
	switch location {
	case LocationPhysical:
		physReg := value.(PhysicalReg)
		if physReg == RegHL {
			// Already in HL
			g.emit("    ; Register %d already in HL", reg)
			return
		}
		// Move from physical register to HL
		regName := g.physicalRegToAssembly(physReg)
		if physReg == RegBC || physReg == RegDE {
			g.emit("    LD H, %s", regName[:1]) // BC->B, DE->D
			g.emit("    LD L, %s", regName[1:]) // BC->C, DE->E
		}
		
	case LocationShadow:
		physReg := value.(PhysicalReg)
		regName := g.physicalRegToAssembly(physReg)
		if physReg == RegHL_Shadow {
			// To load shadow HL to main HL, we need to use stack
			g.emit("    EXX               ; Switch to shadow registers")
			g.emit("    PUSH HL           ; Save shadow HL")
			g.emit("    EXX               ; Switch back to main registers")
			g.emit("    POP HL            ; Load shadow HL into main HL")
		} else if physReg == RegBC_Shadow || physReg == RegDE_Shadow {
			g.emit("    EXX               ; Switch to shadow registers")
			g.emit("    LD H, %s", regName[:1])
			g.emit("    LD L, %s", regName[1:])
			g.emit("    EXX               ; Switch back")
		}
		
	case LocationMemory:
		addr := value.(uint16)
		if !g.useAbsoluteLocals && g.isLocalRegister(reg) {
			// Stack-based local variable - use IX+offset
			offset := g.getLocalOffset(reg)
			g.emit("    LD L, (IX%+d)     ; Virtual register %d from stack (low)", offset, reg)
			g.emit("    LD H, (IX%+d)     ; Virtual register %d from stack (high)", offset+1, reg)
		} else {
			// Absolute addressing
			g.emit("    LD HL, ($%04X)    ; Virtual register %d from memory", addr, reg)
		}
	}
}

// loadToDE loads a virtual register to DE  
func (g *Z80Generator) loadToDE(reg ir.Register) {
	if reg == ir.RegZero {
		g.emit("    LD DE, 0")
		return
	}
	
	// Use hierarchical register allocation
	location, value := g.getRegisterLocation(reg)
	
	switch location {
	case LocationPhysical:
		physReg := value.(PhysicalReg)
		if physReg == RegDE {
			// Already in DE
			g.emit("    ; Register %d already in DE", reg)
			return
		}
		// Move from physical register to DE
		switch physReg {
		case RegBC, RegHL:
			regName := g.physicalRegToAssembly(physReg)
			g.emit("    LD D, %s", regName[:1])
			g.emit("    LD E, %s", regName[1:])
		case RegA:
			g.emit("    LD E, A        ; Load 8-bit value to DE")
			g.emit("    LD D, 0        ; Zero extend")
		case RegB:
			g.emit("    LD E, B        ; Load 8-bit value to DE")
			g.emit("    LD D, 0        ; Zero extend")
		case RegC:
			g.emit("    LD E, C        ; Load 8-bit value to DE")
			g.emit("    LD D, 0        ; Zero extend")
		case RegD:
			g.emit("    LD E, D        ; Load 8-bit value to DE")
			g.emit("    LD D, 0        ; Zero extend")
		case RegE:
			g.emit("    ; E already in place")
			g.emit("    LD D, 0        ; Zero extend")
		case RegH:
			g.emit("    LD E, H        ; Load 8-bit value to DE")
			g.emit("    LD D, 0        ; Zero extend")
		case RegL:
			g.emit("    LD E, L        ; Load 8-bit value to DE")
			g.emit("    LD D, 0        ; Zero extend")
		default:
			g.emit("    ; WARNING: Unknown physical register %v for loadToDE", physReg)
		}
		
	case LocationShadow:
		physReg := value.(PhysicalReg)
		g.emit("    EXX               ; Switch to shadow registers")
		regName := g.physicalRegToAssembly(physReg)
		if physReg == RegDE_Shadow {
			g.emit("    ; Register %d in shadow DE", reg)
			// Need to transfer shadow DE to main DE
			g.emit("    PUSH DE")
			g.emit("    EXX")
			g.emit("    POP DE")
		} else if physReg == RegBC_Shadow || physReg == RegHL_Shadow {
			g.emit("    LD D, %s", regName[:1])
			g.emit("    LD E, %s", regName[1:])
			g.emit("    EXX               ; Switch back")
		}
		
	case LocationMemory:
		addr := value.(uint16)
		if !g.useAbsoluteLocals && g.isLocalRegister(reg) {
			// Stack-based local variable - use IX+offset
			offset := g.getLocalOffset(reg)
			g.emit("    LD E, (IX%+d)     ; Virtual register %d from stack (low)", offset, reg)
			g.emit("    LD D, (IX%+d)     ; Virtual register %d from stack (high)", offset+1, reg)
		} else {
			// Z80 doesn't have direct LD DE, (addr), so we use HL as intermediate
			g.emit("    LD HL, ($%04X)    ; Virtual register %d from memory", addr, reg)
			g.emit("    EX DE, HL")
		}
	}
}

// storeFromHL stores HL to a virtual register
func (g *Z80Generator) storeFromHL(reg ir.Register) {
	// Use hierarchical register allocation
	location, value := g.getRegisterLocation(reg)
	
	switch location {
	case LocationPhysical:
		physReg := value.(PhysicalReg)
		if physReg == RegHL {
			// Already in HL
			g.emit("    ; Register %d already in HL", reg)
			return
		}
		// Move from HL to physical register
		regName := g.physicalRegToAssembly(physReg)
		if physReg == RegBC || physReg == RegDE {
			g.emit("    LD %s, H", regName[:1])
			g.emit("    LD %s, L", regName[1:])
		}
		
	case LocationShadow:
		physReg := value.(PhysicalReg)
		regName := g.physicalRegToAssembly(physReg)
		if physReg == RegHL_Shadow {
			// To store HL to shadow HL, we need to use stack
			g.emit("    PUSH HL           ; Save current HL")
			g.emit("    EXX               ; Switch to shadow registers")
			g.emit("    POP HL            ; Load into shadow HL")
			g.emit("    EXX               ; Switch back to main registers")
		} else if physReg == RegBC_Shadow || physReg == RegDE_Shadow {
			g.emit("    EXX               ; Switch to shadow registers")
			g.emit("    LD %s, H", regName[:1])
			g.emit("    LD %s, L", regName[1:])
			g.emit("    EXX               ; Switch back")
		}
		
	case LocationMemory:
		addr := value.(uint16)
		if !g.useAbsoluteLocals && g.isLocalRegister(reg) {
			// Stack-based local variable - use IX+offset
			offset := g.getLocalOffset(reg)
			g.emit("    LD (IX%+d), L     ; Virtual register %d to stack (low)", offset, reg)
			g.emit("    LD (IX%+d), H     ; Virtual register %d to stack (high)", offset+1, reg)
		} else {
			// Absolute addressing
			g.emit("    LD ($%04X), HL    ; Virtual register %d to memory", addr, reg)
		}
	}
}

// getAbsoluteAddr gets the absolute address for a local variable
func (g *Z80Generator) getAbsoluteAddr(reg ir.Register) uint16 {
	// Check if we have a pre-allocated address for this register
	if addr, ok := g.regAlloc.GetAddress(reg); ok && addr != 0 {
		return addr
	}
	// Default: Each register gets 2 bytes
	return g.localVarBase + uint16(reg)*2
}

// getGlobalAddr gets the absolute address for a global variable
func (g *Z80Generator) getGlobalAddr(name string) uint16 {
	globalBase := uint16(0xF000)
	for i, global := range g.module.Globals {
		if global.Name == name {
			// Each global gets 32 bytes of space
			return globalBase + uint16(i*32)
		}
	}
	return 0 // Not found
}

// newLabel generates a new label
func (g *Z80Generator) newLabel() string {
	g.labelCounter++
	return fmt.Sprintf(".L%d", g.labelCounter)
}

// emit writes a line of assembly
func (g *Z80Generator) emit(format string, args ...interface{}) {
	if len(args) > 0 {
		fmt.Fprintf(g.writer, format+"\n", args...)
	} else {
		fmt.Fprintln(g.writer, format)
	}
}

// getFunctionLabel generates a function-scoped label to avoid duplicates
func (g *Z80Generator) getFunctionLabel(prefix string) string {
	if g.currentFunc == nil {
		return fmt.Sprintf("%s_%d", prefix, g.labelCounter)
	}
	// Create a sanitized function name for labels
	funcName := strings.ReplaceAll(g.currentFunc.Name, ".", "_")
	funcName = strings.ReplaceAll(funcName, "$", "_")
	return fmt.Sprintf("%s_%s_%d", funcName, prefix, g.labelCounter)
}

// sanitizeLabel makes IR-generated labels function-scoped
func (g *Z80Generator) sanitizeLabel(label string) string {
	if g.currentFunc == nil {
		return label
	}
	// Create a sanitized function name for labels
	funcName := strings.ReplaceAll(g.currentFunc.Name, ".", "_")
	funcName = strings.ReplaceAll(funcName, "$", "_")
	return fmt.Sprintf("%s_%s", funcName, label)
}

// findFunction finds a function in the current module
func (g *Z80Generator) findFunction(name string) *ir.Function {
	if g.module == nil {
		return nil
	}
	for _, fn := range g.module.Functions {
		if fn.Name == name {
			return fn
		}
		// Also check if the short name matches
		if idx := strings.LastIndex(fn.Name, "."); idx >= 0 {
			shortName := fn.Name[idx+1:]
			if shortName == name {
				return fn
			}
		}
	}
	return nil
}

// generateTrueSMCCall generates patching code for TRUE SMC function call
func (g *Z80Generator) generateTrueSMCCall(inst ir.Instruction, targetFunc *ir.Function) {
	g.emit("    ; TRUE SMC call to %s", targetFunc.Name)
	
	// Validate we have the right number of arguments
	if len(inst.Args) != len(targetFunc.Params) {
		g.emit("    ; ERROR: argument count mismatch")
		g.emit("    CALL %s", inst.Symbol)
		return
	}
	
	// Patch each parameter anchor with the argument value
	for i, param := range targetFunc.Params {
		argReg := inst.Args[i]
		anchorAddr := fmt.Sprintf("%s$imm0", param.Name)
		
		if param.Type.Size() == 1 {
			// 8-bit patch
			g.loadToA(argReg)
			g.emit("    LD (%s), A        ; Patch %s", anchorAddr, param.Name)
		} else if param.Type.Size() == 3 {
			// 24-bit patch - need to patch both high byte and low 16 bits
			anchorHigh := fmt.Sprintf("%s$immHI0", param.Name)
			anchorLow := fmt.Sprintf("%s$immLO0", param.Name)
			
			// Load the 24-bit value
			// For now, assume it's in memory as 3 consecutive bytes
			g.loadToHL(argReg)  // This loads the low 16 bits
			
			// Patch low 16 bits
			g.emit("    LD (%s), HL       ; Patch %s low 16 bits", anchorLow, param.Name)
			
			// Load high byte - for now, assume it's at argReg+2 in memory
			// TODO: Proper 24-bit register allocation
			g.emit("    LD A, 0           ; TODO: Load high byte of %s", param.Name)
			g.emit("    LD (%s), A        ; Patch %s high byte", anchorHigh, param.Name)
		} else {
			// 16-bit patch - NO DI/EI needed (atomic instruction)
			g.loadToHL(argReg)
			g.emit("    LD (%s), HL       ; Patch %s (atomic)", anchorAddr, param.Name)
		}
	}
	
	// Make the call
	g.emit("    CALL %s", targetFunc.Name)
}

// emitAsmBlock processes and emits inline assembly code
func (g *Z80Generator) emitAsmBlock(code string) {
	// Process the assembly code line by line
	lines := strings.Split(code, "\n")
	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" {
			continue
		}
		
		// Skip opening and closing braces from asm blocks
		if trimmedLine == "{" || trimmedLine == "}" {
			continue
		}
		
		// Process !symbol references
		processedLine := g.resolveAsmSymbols(trimmedLine)
		
		// Emit the processed line with proper indentation
		if strings.Contains(processedLine, ":") && !strings.Contains(processedLine, "(") {
			// Labels go at the beginning of the line
			g.emit(processedLine)
		} else {
			// Instructions are indented
			g.emit("    %s", processedLine)
		}
	}
}

// resolveAsmSymbols replaces !symbol references with actual values
func (g *Z80Generator) resolveAsmSymbols(line string) string {
	// Simple replacement for !symbol patterns
	result := line
	
	// Find all !symbol references
	for i := 0; i < len(line); i++ {
		if line[i] == '!' && i+1 < len(line) && (isAlpha(line[i+1]) || line[i+1] == '_') {
			// Find the end of the symbol
			start := i
			i++
			for i < len(line) && (isAlnum(line[i]) || line[i] == '_' || line[i] == '.') {
				i++
			}
			
			// Extract the symbol
			symbol := line[start+1:i]
			
			// Resolve the symbol
			replacement := g.resolveSymbol(symbol)
			
			// Replace in the result
			result = result[:start] + replacement + result[i:]
			
			// Adjust index for the replacement
			i = start + len(replacement) - 1
		}
	}
	
	return result
}

// resolveSymbol resolves a symbol to its address or value
func (g *Z80Generator) resolveSymbol(symbol string) string {
	// Check for dotted notation (e.g., block.label)
	if strings.Contains(symbol, ".") {
		parts := strings.Split(symbol, ".")
		if len(parts) == 2 {
			// For now, just return the full symbol as a label
			return symbol
		}
	}
	
	// Check if it's a global variable
	for _, global := range g.module.Globals {
		if global.Name == symbol {
			return global.Name // Use the label directly
		}
	}
	
	// Check if it's a function
	for _, fn := range g.module.Functions {
		if fn.Name == symbol {
			return fn.Name // Use the function label directly
		}
	}
	
	// Check if it's a global variable
	globalBase := uint16(0xF000)
	for i, global := range g.module.Globals {
		if global.Name == symbol {
			// Each global gets 32 bytes of space
			addr := globalBase + uint16(i*32)
			return fmt.Sprintf("$%04X", addr)
		}
	}
	
	// Check if it's a local variable
	if g.currentFunc != nil {
		for i, local := range g.currentFunc.Locals {
			if local.Name == symbol {
				// Return the stack offset or memory location
				if g.useAbsoluteLocals {
					return fmt.Sprintf("$%04X", g.localVarBase + uint16(i*2))
				} else {
					// Calculate offset directly for local variables
					offset := g.stackOffset + i*2
					return fmt.Sprintf("(IX-%d)", offset)
				}
			}
		}
		
		// Check parameters
		for i, param := range g.currentFunc.Params {
			if param.Name == symbol {
				// Parameters are above the return address
				offset := 4 + (len(g.currentFunc.Params)-i-1)*2
				return fmt.Sprintf("(IX+%d)", offset)
			}
		}
	}
	
	// If not found, return the symbol unchanged (let sjasmplus handle it)
	return "!" + symbol
}

// Helper functions for character checking
func isAlpha(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
}

func isDigit(ch byte) bool {
	return ch >= '0' && ch <= '9'
}

func isAlnum(ch byte) bool {
	return isAlpha(ch) || isDigit(ch)
}

// RegisterAllocator manages Z80 register allocation
type RegisterAllocator struct {
	// Maps virtual registers to Z80 registers
	allocation map[ir.Register]string
	// Tracks which Z80 registers are in use
	inUse map[string]bool
	// Maps virtual registers to memory addresses
	addresses map[ir.Register]uint16
}

// NewRegisterAllocator creates a new register allocator
func NewRegisterAllocator() *RegisterAllocator {
	return &RegisterAllocator{
		allocation: make(map[ir.Register]string),
		inUse:      make(map[string]bool),
		addresses:  make(map[ir.Register]uint16),
	}
}

// Reset clears the allocator state
func (r *RegisterAllocator) Reset() {
	r.allocation = make(map[ir.Register]string)
	r.inUse = make(map[string]bool)
	r.addresses = make(map[ir.Register]uint16)
}

// Allocate assigns a Z80 register to a virtual register
func (r *RegisterAllocator) Allocate(reg ir.Register) string {
	// For now, always spill to memory
	// TODO: Implement proper register allocation
	return ""
}

// Free releases a Z80 register
func (r *RegisterAllocator) Free(z80reg string) {
	r.inUse[z80reg] = false
}

// SetAddress assigns a memory address to a virtual register
func (r *RegisterAllocator) SetAddress(reg ir.Register, addr uint16) {
	r.addresses[reg] = addr
}

// GetAddress returns the memory address for a virtual register
func (r *RegisterAllocator) GetAddress(reg ir.Register) (uint16, bool) {
	addr, ok := r.addresses[reg]
	return addr, ok
}

// Hierarchical register allocation helpers

// getRegisterLocation determines how a virtual register should be accessed
type RegisterLocation int

const (
	LocationPhysical RegisterLocation = iota // Allocated to physical Z80 register
	LocationShadow                           // Allocated to shadow register  
	LocationMemory                           // Fallback to memory address
)

// getRegisterLocation determines where a virtual register is allocated
func (g *Z80Generator) getRegisterLocation(reg ir.Register) (RegisterLocation, interface{}) {
	if !g.usePhysicalRegs {
		// Physical allocation disabled, use memory
		return LocationMemory, g.getAbsoluteAddr(reg)
	}
	
	// Check physical register allocation first
	if physReg, allocated := g.physicalAlloc.GetAllocation(reg); allocated && physReg != RegNone {
		if physReg >= RegA_Shadow && physReg <= RegHL_Shadow {
			return LocationShadow, physReg
		}
		return LocationPhysical, physReg
	}
	
	// Fallback to memory
	return LocationMemory, g.getAbsoluteAddr(reg)
}

// physicalRegToAssembly converts PhysicalReg to assembly string
func (g *Z80Generator) physicalRegToAssembly(reg PhysicalReg) string {
	switch reg {
	case RegA: return "A"
	case RegB: return "B"
	case RegC: return "C"
	case RegD: return "D"
	case RegE: return "E"
	case RegH: return "H"
	case RegL: return "L"
	case RegBC: return "BC"
	case RegDE: return "DE"
	case RegHL: return "HL"
	case RegIX: return "IX"
	case RegIY: return "IY"
	// Shadow registers require EXX/EX AF,AF' for access
	case RegA_Shadow: return "A'"
	case RegB_Shadow: return "B'"
	case RegC_Shadow: return "C'"
	case RegD_Shadow: return "D'"
	case RegE_Shadow: return "E'"
	case RegH_Shadow: return "H'"
	case RegL_Shadow: return "L'"
	case RegBC_Shadow: return "BC'"
	case RegDE_Shadow: return "DE'"
	case RegHL_Shadow: return "HL'"
	default: return "???"
	}
}

// loadToB loads a virtual register to B
func (g *Z80Generator) loadToB(reg ir.Register) {
	g.loadToA(reg)
	g.emit("    LD B, A")
}

// canOptimizeToDJNZ checks if we can optimize DEC + JUMP_IF_NOT_ZERO to DJNZ
func (g *Z80Generator) canOptimizeToDJNZ(decInst ir.Instruction) bool {
	// Check if this is the start of a DJNZ pattern
	idx := g.currentInstructionIndex
	if idx+1 >= len(g.currentFunction.Instructions) {
		return false
	}
	
	nextInst := g.currentFunction.Instructions[idx+1]
	
	// Pattern: DEC reg, JUMP_IF_NOT_ZERO same_reg, label
	return nextInst.Op == ir.OpJumpIfNotZero && 
		   decInst.Dest == nextInst.Src1 &&
		   decInst.Src1 == nextInst.Src1
}

// generateDJNZ generates optimized DJNZ instruction
func (g *Z80Generator) generateDJNZ(decInst ir.Instruction) error {
	// Get the next instruction (JUMP_IF_NOT_ZERO)
	nextInst := g.currentFunction.Instructions[g.currentInstructionIndex+1]
	
	// Load counter to B register
	g.loadToB(decInst.Src1)
	
	// Generate DJNZ
	g.emit("    DJNZ %s", nextInst.Symbol)
	
	// Skip the next instruction since we've handled it
	g.currentInstructionIndex++
	
	return nil
}

// shouldUseStackLocals determines if a function should use stack-based locals
func (g *Z80Generator) shouldUseStackLocals(fn *ir.Function) bool {
	// Use stack locals for:
	// 1. Recursive functions (required)
	if g.isRecursive(fn) {
		return true
	}
	
	// 2. Functions with many locals (> 6)
	if len(fn.Locals) > 6 {
		return true
	}
	
	// 3. Functions that call other functions (preserve locals across calls)
	for _, inst := range fn.Instructions {
		if inst.Op == ir.OpCall {
			return true
		}
	}
	
	// Otherwise use absolute addressing for speed
	return false
}

// isRecursive checks if a function is recursive
func (g *Z80Generator) isRecursive(fn *ir.Function) bool {
	// Check if function calls itself
	for _, inst := range fn.Instructions {
		if inst.Op == ir.OpCall && inst.Symbol == fn.Name {
			return true
		}
	}
	return false
}

// getLocalOffset calculates the IX+offset for a local variable
func (g *Z80Generator) getLocalOffset(reg ir.Register) int {
	// Get the stored offset (positive value)
	addr, ok := g.regAlloc.GetAddress(reg)
	if !ok {
		// Default offset if not found
		return -int(reg) * 2
	}
	// Convert to negative offset from IX
	return -int(addr)
}

// isLocalRegister checks if a register represents a local variable
func (g *Z80Generator) isLocalRegister(reg ir.Register) bool {
	// Check if this register is in the current function's locals
	if g.currentFunc == nil {
		return false
	}
	for _, local := range g.currentFunc.Locals {
		if local.Reg == reg {
			return true
		}
	}
	return false
}

// needsPrintHelpers checks if any print functions are used in the module
func (g *Z80Generator) needsPrintHelpers() bool {
	// Check all functions for print-related operations
	for _, fn := range g.module.Functions {
		for _, inst := range fn.Instructions {
			switch inst.Op {
			case ir.OpPrintU8, ir.OpPrintU16, ir.OpPrintI8, ir.OpPrintI16,
			     ir.OpPrintBool, ir.OpPrintString:
				// These operations use helper functions
				return true
			case ir.OpCall:
				// Check if it's a print function call
				if inst.Symbol != "" && (strings.Contains(inst.Symbol, "print_") ||
				   strings.Contains(inst.Symbol, "@print")) {
					return true
				}
			}
		}
	}
	return false
}

// generatePrintHelpers generates runtime helper functions for print operations
func (g *Z80Generator) generatePrintHelpers() {
	g.emit("\n; Runtime print helper functions")
	
	// Print string function - prints length-prefixed string pointed to by HL
	// Auto-detects u8 vs u16 length format
	g.emit("print_string:")
	g.emit("    LD A, (HL)         ; A = first byte")
	g.emit("    CP 255             ; Check if extended format marker")
	g.emit("    JR Z, print_string_u16")
	g.emit("    ; Standard u8 format: [len:u8][data...]")
	g.emit("    LD B, A            ; B = length from first byte")
	g.emit("    INC HL             ; HL -> string data")
	g.emit("    OR A               ; Check if length is zero")
	g.emit("    RET Z              ; Return if empty string")
	g.emit("print_loop_u8:")
	g.emit("    LD A, (HL)         ; Load character")
	
	// Platform-specific character output
	switch g.targetPlatform {
	case "cpm":
		g.emit("    PUSH BC            ; Save counter")
		g.emit("    PUSH HL            ; Save string pointer")
		g.emit("    LD E, A            ; Character to E")
		g.emit("    LD C, 2            ; BDOS function 2")
		g.emit("    CALL 5             ; Call BDOS")
		g.emit("    POP HL             ; Restore string pointer")
		g.emit("    POP BC             ; Restore counter")
	case "msx":
		g.emit("    CALL $00A2         ; MSX BIOS CHPUT")
	case "cpc", "amstrad":
		g.emit("    CALL $BB5A         ; CPC TXT OUTPUT")
	default: // "zxspectrum" and others
		g.emit("    RST 16             ; ZX Spectrum ROM print")
	}
	g.emit("    INC HL             ; Next character")
	g.emit("    DJNZ print_loop_u8 ; Decrement B and loop")
	g.emit("    RET")
	g.emit("")
	
	g.emit("print_string_u16:")
	g.emit("    ; Extended u16 format: [255][len:u16][data...]")
	g.emit("    INC HL             ; Skip 255 marker")
	g.emit("    LD E, (HL)         ; E = low byte of length")
	g.emit("    INC HL")
	g.emit("    LD D, (HL)         ; D = high byte of length") 
	g.emit("    INC HL             ; HL -> string data")
	g.emit("    LD A, D            ; Check if length is zero")
	g.emit("    OR E")
	g.emit("    RET Z              ; Return if empty string")
	g.emit("    ; Use 16-bit counter for large strings")
	g.emit("print_loop_u16:")
	g.emit("    LD A, (HL)         ; Load character")
	
	// Platform-specific character output for u16 loop
	switch g.targetPlatform {
	case "cpm":
		g.emit("    PUSH DE            ; Save counter")
		g.emit("    PUSH HL            ; Save string pointer")
		g.emit("    LD E, A            ; Character to E")
		g.emit("    LD C, 2            ; BDOS function 2")
		g.emit("    CALL 5             ; Call BDOS")
		g.emit("    POP HL             ; Restore string pointer")
		g.emit("    POP DE             ; Restore counter")
	case "msx":
		g.emit("    CALL $00A2         ; MSX BIOS CHPUT")
	case "cpc", "amstrad":
		g.emit("    CALL $BB5A         ; CPC TXT OUTPUT")
	default: // "zxspectrum" and others
		g.emit("    RST 16             ; Print character")
	}
	g.emit("    INC HL             ; Next character")
	g.emit("    DEC DE             ; Decrement 16-bit counter")
	g.emit("    LD A, D            ; Check if counter is zero")
	g.emit("    OR E")
	g.emit("    JR NZ, print_loop_u16")
	g.emit("    RET")
	g.emit("")
	
	// Print u8 as decimal
	g.emit("print_u8_decimal:")
	g.emit("    LD H, 0            ; HL = A (zero extend)")
	g.emit("    LD L, A")
	g.emit("    CALL print_u16_decimal")
	g.emit("    RET")
	g.emit("")
	
	// Print u16 as decimal
	g.emit("print_u16_decimal:")
	g.emit("    LD BC, -10000")
	g.emit("    LD DE, -1000")
	g.emit("    CALL print_digit")
	g.emit("    LD BC, -1000")
	g.emit("    LD DE, -100")
	g.emit("    CALL print_digit")
	g.emit("    LD BC, -100")
	g.emit("    LD DE, -10")
	g.emit("    CALL print_digit")
	g.emit("    LD BC, -10")
	g.emit("    LD DE, -1")
	g.emit("    CALL print_digit")
	g.emit("    LD A, L")
	g.emit("    ADD A, '0'         ; Convert to ASCII")
	g.emit("    RST 16             ; Print last digit")
	g.emit("    RET")
	g.emit("")
	
	// Helper function for printing digits
	g.emit("print_digit:")
	g.emit("    LD A, '0'-1")
	g.emit("print_digit_loop:")
	g.emit("    INC A")
	g.emit("    ADD HL, BC         ; Subtract power of 10")
	g.emit("    JR C, print_digit_loop")
	g.emit("    ADD HL, DE         ; Add back one power of 10")
	g.emit("    RST 16             ; Print digit")
	g.emit("    RET")
	g.emit("")
	
	// Print signed integers (same as unsigned for now)
	g.emit("print_i8_decimal:")
	g.emit("    BIT 7, A           ; Check sign bit")
	g.emit("    JR Z, print_u8_decimal")
	g.emit("    PUSH AF")
	g.emit("    LD A, '-'          ; Print minus sign")
	g.emit("    RST 16")
	g.emit("    POP AF")
	g.emit("    NEG                ; Make positive")
	g.emit("    JR print_u8_decimal")
	g.emit("")
	
	g.emit("print_i16_decimal:")
	g.emit("    BIT 7, H           ; Check sign bit")
	g.emit("    JR Z, print_u16_decimal")
	g.emit("    PUSH HL")
	g.emit("    LD A, '-'          ; Print minus sign")
	g.emit("    RST 16")
	g.emit("    POP HL")
	g.emit("    LD A, H            ; Negate HL")
	g.emit("    CPL")
	g.emit("    LD H, A")
	g.emit("    LD A, L")
	g.emit("    CPL")
	g.emit("    LD L, A")
	g.emit("    INC HL")
	g.emit("    JR print_u16_decimal")
	g.emit("")
	
	// Print boolean
	g.emit("print_bool:")
	g.emit("    OR A               ; Test if A is zero")
	g.emit("    JR NZ, print_true")
	g.emit("    LD HL, bool_false_str")
	g.emit("    JR print_string")
	g.emit("print_true:")
	g.emit("    LD HL, bool_true_str")
	g.emit("    JR print_string")
	g.emit("")
	
	// Boolean string constants (length-prefixed)
	g.emit("bool_true_str:")
	g.emit("    DB 4, \"true\"      ; Length + data")
	g.emit("bool_false_str:")
	g.emit("    DB 5, \"false\"     ; Length + data")
	g.emit("")
}

