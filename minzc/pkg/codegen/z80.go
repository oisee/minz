package codegen

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/minz/minzc/pkg/ir"
)

// Z80Generator generates Z80 assembly from IR
type Z80Generator struct {
	writer        io.Writer
	module        *ir.Module
	currentFunc   *ir.Function
	currentFunction *ir.Function // For DJNZ optimization
	currentInstructionIndex int  // For DJNZ optimization
	regAlloc      *RegisterAllocator
	stackOffset   int
	labelCounter  int
	useShadowRegs bool // Whether to use shadow registers for current function
	localVarBase  uint16 // Base address for local variables (absolute addressing)
	useAbsoluteLocals bool // Whether to use absolute addressing for locals
	emittedParams map[string]bool // Track which SMC parameters have been emitted
	currentRegister ir.Register // Track which virtual register is currently in HL
}

// NewZ80Generator creates a new Z80 code generator
func NewZ80Generator(w io.Writer) *Z80Generator {
	return &Z80Generator{
		writer:   w,
		regAlloc: NewRegisterAllocator(),
		localVarBase: 0xF000, // Default local variable area at 0xF000
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
		for _, global := range module.Globals {
			g.generateGlobal(global)
		}
		
		// Generate string literals
		for _, str := range module.Strings {
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
	
	switch global.Type.(type) {
	case *ir.BasicType:
		if global.Init != nil {
			g.emit("    DW %v", global.Init)
		} else {
			g.emit("    DW 0")
		}
	case *ir.ArrayType:
		// TODO: Handle array initialization
		size := global.Type.Size()
		g.emit("    DS %d", size)
	default:
		g.emit("    ; TODO: %s type", global.Type.String())
	}
}

// generateString generates a string literal
func (g *Z80Generator) generateString(str *ir.String) {
	g.emit("%s:", str.Label)
	
	// Escape special characters and emit as DB directive
	escaped := ""
	for _, ch := range str.Value {
		if ch >= 32 && ch <= 126 && ch != '"' && ch != '\\' {
			escaped += string(ch)
		} else {
			// If we have accumulated string content, emit it
			if escaped != "" {
				g.emit("    DB \"%s\"", escaped)
				escaped = ""
			}
			// Emit special character as numeric value
			g.emit("    DB %d", ch)
		}
	}
	
	// Emit any remaining string content
	if escaped != "" {
		g.emit("    DB \"%s\"", escaped)
	}
	
	// Null terminator
	g.emit("    DB 0")
}

// generateFunction generates code for a function
func (g *Z80Generator) generateFunction(fn *ir.Function) error {
	g.currentFunc = fn
	g.currentFunction = fn
	g.currentInstructionIndex = 0
	g.stackOffset = 0
	g.regAlloc.Reset()

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

	// Function prologue
	g.generatePrologue(fn)

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
		// 8-bit parameter - use LD A, n
		g.emit("    LD A, 0        ; %s anchor (will be patched)", param.Name)
		g.emit("%s EQU %s+1", anchorImm, anchorOp)
		// Value is now in A, store to destination
		g.storeFromA(destReg)
	} else {
		// 16-bit parameter - use LD HL, nn
		g.emit("    LD HL, 0       ; %s anchor (will be patched)", param.Name)
		g.emit("%s EQU %s+1", anchorImm, anchorOp)
		// Value is now in HL, store to destination
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
	
	// Generate instructions with SMC awareness
	for _, inst := range fn.Instructions {
		if err := g.generateSMCInstruction(inst); err != nil {
			return err
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
		g.emit("    RET")
	}
	
	return nil
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
		g.emit("    JR NC, .no_err_%d", g.labelCounter)
		g.emit("    INC HL         ; Error detected")
		g.emit(".no_err_%d:", g.labelCounter)
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
			
			// Emit the parameter label and instruction
			// The instruction's immediate value IS the parameter
			g.emit("%s:", paramLabel)
			
			// For the first use, we need to emit the load instruction
			if param.Type.Size() == 1 {
				// For u8, load into HL as u16 to avoid conversions
				g.emit("    LD HL, #0000   ; SMC parameter %s (u8->u16)", paramName)
				// Store to the destination
				g.storeFromHL(inst.Dest)
			} else {
				g.emit("    LD HL, #0000   ; SMC parameter %s", paramName)
				// Store to the destination
				g.storeFromHL(inst.Dest)
			}
		} else {
			// Subsequent use - load from the parameter location
			// The parameter value is embedded in the instruction
			if inst.Type != nil && inst.Type.Size() == 1 {
				g.emit("    LD A, (%s)", paramLabel)
				g.storeFromA(inst.Dest)
			} else {
				g.emit("    LD HL, (%s)", paramLabel)
				g.storeFromHL(inst.Dest)
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
	
	// Always save frame pointer for functions with locals or parameters
	if len(fn.Locals) > 0 || len(fn.Params) > 0 {
		g.emit("    PUSH IX")
		g.emit("    LD IX, SP")
	}
	
	// Check if we should use shadow registers for this function
	if fn.UsedRegisters.Contains(ir.Z80_BC_SHADOW | ir.Z80_DE_SHADOW | ir.Z80_HL_SHADOW) {
		g.useShadowRegs = true
		g.emit("    EXX           ; Switch to shadow registers")
	}

	// Allocate space for locals
	localSpace := len(fn.Locals) * 2 // 2 bytes per local
	if localSpace > 0 {
		g.emit("    LD HL, -%d", localSpace)
		g.emit("    ADD HL, SP")
		g.emit("    LD SP, HL")
		g.stackOffset = localSpace
	}

	// Load parameters from stack to registers/locals
	for i, param := range fn.Params {
		// Parameters are at positive offsets from IX
		// First param at IX+4 (after return address and saved IX)
		offset := 4 + i*2
		g.emit("    ; Parameter %s", param.Name)
		
		// For now, load to accumulator then store to local
		g.emit("    LD L, (IX+%d)", offset)
		g.emit("    LD H, (IX+%d)", offset+1)
		
		// Store in local variable space
		localOffset := g.allocateLocal(param.Reg)
		g.emit("    LD (IX-%d), L", localOffset)
		g.emit("    LD (IX-%d), H", localOffset-1)
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
		g.emit("%s:", inst.Label)
		
	case ir.OpJump:
		g.emit("    JP %s", inst.Label)
		
	case ir.OpJumpIf:
		// Load condition to A
		g.loadToA(inst.Src1)
		g.emit("    OR A")
		g.emit("    JP NZ, %s", inst.Label)
		
	case ir.OpJumpIfNot:
		// Load condition to A
		g.loadToA(inst.Src1)
		g.emit("    OR A")
		g.emit("    JP Z, %s", inst.Label)
		
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
		// Load variable - for now, assume it's a local
		if g.useAbsoluteLocals {
			addr := g.getAbsoluteAddr(inst.Src1) // Note: source register for load
			g.emit("    LD HL, ($%04X)", addr)
		} else {
			offset := g.getLocalOffset(inst.Src1)
			g.emit("    LD L, (IX-%d)", offset)
			g.emit("    LD H, (IX-%d)", offset-1)
		}
		g.storeFromHL(inst.Dest)
		
	case ir.OpStoreVar:
		// Store to variable
		g.loadToHL(inst.Src1)
		if g.useAbsoluteLocals {
			addr := g.getAbsoluteAddr(inst.Dest)
			g.emit("    LD ($%04X), HL", addr)
		} else {
			offset := g.getLocalOffset(inst.Dest)
			g.emit("    LD (IX-%d), L", offset)
			g.emit("    LD (IX-%d), H", offset-1)
		}
		
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
		g.loadToHL(inst.Src1)
		g.emit("    LD D, H")
		g.emit("    LD E, L")
		g.loadToHL(inst.Src2)
		g.emit("    EX DE, HL")
		g.emit("    OR A      ; Clear carry")
		g.emit("    SBC HL, DE")
		g.storeFromHL(inst.Dest)
		
	case ir.OpMul:
		// Simple multiplication for small numbers
		// TODO: Implement proper 16-bit multiplication
		g.emit("    ; TODO: Multiplication")
		g.emit("    LD HL, 0")
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
		
	case ir.OpNot:
		// Bitwise NOT (one's complement)
		g.loadToHL(inst.Src1)
		g.emit("    LD A, L")
		g.emit("    CPL")
		g.emit("    LD L, A")
		g.emit("    LD A, H")
		g.emit("    CPL")
		g.emit("    LD H, A")
		g.storeFromHL(inst.Dest)
		
	case ir.OpShl:
		// Shift left
		// TODO: Implement shift left
		g.emit("    ; TODO: Shift left")
		g.loadToHL(inst.Src1)
		g.storeFromHL(inst.Dest)
		
	case ir.OpShr:
		// Shift right
		// TODO: Implement shift right
		g.emit("    ; TODO: Shift right")
		g.loadToHL(inst.Src1)
		g.storeFromHL(inst.Dest)
		
	case ir.OpEq, ir.OpNe, ir.OpLt, ir.OpGt, ir.OpLe, ir.OpGe:
		g.generateComparison(inst)
		
	case ir.OpCall:
		// Check if calling a TRUE SMC function
		g.emit("    ; Call to %s (args: %d)", inst.Symbol, len(inst.Args))
		targetFunc := g.findFunction(inst.Symbol)
		if targetFunc != nil {
			g.emit("    ; Found function, UsesTrueSMC=%v", targetFunc.UsesTrueSMC)
			if targetFunc.UsesTrueSMC {
				// Generate TRUE SMC patching before call
				g.generateTrueSMCCall(inst, targetFunc)
			} else {
				g.emit("    CALL %s", inst.Symbol)
			}
		} else {
			// Try with just the function name (without module prefix)
			shortName := inst.Symbol
			if idx := strings.LastIndex(inst.Symbol, "."); idx >= 0 {
				shortName = inst.Symbol[idx+1:]
			}
			targetFunc = g.findFunction(shortName)
			if targetFunc != nil && targetFunc.UsesTrueSMC {
				// Generate TRUE SMC patching before call
				g.generateTrueSMCCall(inst, targetFunc)
			} else {
				// Regular call
				// TODO: Handle arguments properly
				g.emit("    CALL %s", shortName)
			}
		}
		// Result is in HL
		g.storeFromHL(inst.Dest)
		
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
		
	default:
		return fmt.Errorf("unsupported opcode: %v (%d)", inst.Op, int(inst.Op))
	}

	return nil
}

// generateComparison generates code for comparison operations
func (g *Z80Generator) generateComparison(inst ir.Instruction) {
	// Load operands
	g.loadToHL(inst.Src1)
	g.emit("    LD D, H")
	g.emit("    LD E, L")
	g.loadToHL(inst.Src2)
	
	// Compare DE with HL (reversed for correct comparison)
	g.emit("    EX DE, HL")
	g.emit("    OR A      ; Clear carry")
	g.emit("    SBC HL, DE")
	
	// Generate appropriate test
	trueLabel := g.newLabel()
	endLabel := g.newLabel()
	
	switch inst.Op {
	case ir.OpEq:
		g.emit("    JP Z, %s", trueLabel)
	case ir.OpNe:
		g.emit("    JP NZ, %s", trueLabel)
	case ir.OpLt:
		g.emit("    JP M, %s", trueLabel)
	case ir.OpGe:
		g.emit("    JP P, %s", trueLabel)
		g.emit("    JP Z, %s", trueLabel)
	case ir.OpGt:
		g.emit("    JP Z, %s", endLabel)
		g.emit("    JP P, %s", trueLabel)
	case ir.OpLe:
		g.emit("    JP M, %s", trueLabel)
		g.emit("    JP Z, %s", trueLabel)
		
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
		// Src1 = counter register, Label = target
		g.loadToB(inst.Src1)
		g.emit("    DJNZ %s", inst.Label)
		// Update register with new value
		g.emit("    LD A, B")
		g.storeFromA(inst.Src1)
		
	case ir.OpLoadImm:
		// Load immediate value
		if inst.Imm < 256 {
			g.emit("    LD A, %d", inst.Imm)
			g.storeFromA(inst.Dest)
		} else {
			g.emit("    LD HL, %d", inst.Imm)
			g.storeFromHL(inst.Dest)
		}
		
	case ir.OpAddImm:
		// Add immediate to register
		g.loadToHL(inst.Src1)
		if inst.Imm < 256 {
			g.emit("    LD DE, %d", inst.Imm)
			g.emit("    ADD HL, DE")
		} else {
			g.emit("    LD DE, %d", inst.Imm)
			g.emit("    ADD HL, DE")
		}
		g.storeFromHL(inst.Dest)
		
	case ir.OpCmp:
		// Compare two registers
		g.loadToHL(inst.Src1)
		g.emit("    PUSH HL")
		g.loadToHL(inst.Src2)
		g.emit("    EX DE, HL")
		g.emit("    POP HL")
		g.emit("    OR A            ; Clear carry")
		g.emit("    SBC HL, DE      ; HL = Src1 - Src2")
		// Result in flags, store comparison result
		g.emit("    LD HL, 0        ; Default false")
		g.emit("    JR NZ, cmp_%d", g.labelCounter)
		g.emit("    INC HL          ; Equal")
		g.emit("cmp_%d:", g.labelCounter)
		g.labelCounter++
		g.storeFromHL(inst.Dest)
		
	case ir.OpLoadDirect:
		// Load from direct memory address
		addr := uint16(inst.Imm)
		if inst.Type != nil && inst.Type.Size() == 1 {
			g.emit("    LD A, ($%04X)", addr)
			g.storeFromA(inst.Dest)
		} else {
			g.emit("    LD HL, ($%04X)", addr)
			g.storeFromHL(inst.Dest)
		}
		
	case ir.OpStoreDirect:
		// Store to direct memory address
		addr := uint16(inst.Imm)
		if inst.Type != nil && inst.Type.Size() == 1 {
			g.loadToA(inst.Src1)
			g.emit("    LD ($%04X), A", addr)
		} else {
			g.loadToHL(inst.Src1)
			g.emit("    LD ($%04X), HL", addr)
		}
	}
	
	// False path
	g.emit("    LD HL, 0")
	g.emit("    JP %s", endLabel)
	
	// True path
	g.emit("%s:", trueLabel)
	g.emit("    LD HL, 1")
	
	// End
	g.emit("%s:", endLabel)
	g.storeFromHL(inst.Dest)
}

// Register management helpers

// loadToA loads a virtual register to A
func (g *Z80Generator) loadToA(reg ir.Register) {
	if reg == ir.RegZero {
		g.emit("    XOR A")
		return
	}
	
	if g.useAbsoluteLocals {
		addr := g.getAbsoluteAddr(reg)
		g.emit("    LD A, ($%04X)", addr)
	} else {
		offset := g.getLocalOffset(reg)
		g.emit("    LD A, (IX-%d)", offset)
	}
}

// storeFromA stores A to a virtual register
func (g *Z80Generator) storeFromA(reg ir.Register) {
	if g.useAbsoluteLocals {
		addr := g.getAbsoluteAddr(reg)
		g.emit("    LD ($%04X), A", addr)
	} else {
		offset := g.getLocalOffset(reg)
		g.emit("    LD (IX-%d), A", offset)
	}
}

// loadToHL loads a virtual register to HL
func (g *Z80Generator) loadToHL(reg ir.Register) {
	if reg == ir.RegZero {
		g.emit("    LD HL, 0")
		return
	}
	
	if g.useAbsoluteLocals {
		addr := g.getAbsoluteAddr(reg)
		g.emit("    LD HL, ($%04X)", addr)
	} else {
		offset := g.getLocalOffset(reg)
		g.emit("    LD L, (IX-%d)", offset)
		g.emit("    LD H, (IX-%d)", offset-1)
	}
}

// loadToDE loads a virtual register to DE
func (g *Z80Generator) loadToDE(reg ir.Register) {
	if reg == ir.RegZero {
		g.emit("    LD DE, 0")
		return
	}
	
	if g.useAbsoluteLocals {
		addr := g.getAbsoluteAddr(reg)
		// Z80 doesn't have direct LD DE, (addr), so we use HL as intermediate
		g.emit("    LD HL, ($%04X)", addr)
		g.emit("    EX DE, HL")
	} else {
		offset := g.getLocalOffset(reg)
		g.emit("    LD E, (IX-%d)", offset)
		g.emit("    LD D, (IX-%d)", offset-1)
	}
}

// storeFromHL stores HL to a virtual register
func (g *Z80Generator) storeFromHL(reg ir.Register) {
	if g.useAbsoluteLocals {
		addr := g.getAbsoluteAddr(reg)
		g.emit("    LD ($%04X), HL", addr)
	} else {
		offset := g.getLocalOffset(reg)
		g.emit("    LD (IX-%d), L", offset)
		g.emit("    LD (IX-%d), H", offset-1)
	}
}

// allocateLocal allocates stack space for a local variable
func (g *Z80Generator) allocateLocal(reg ir.Register) int {
	// For now, simple allocation - each register gets 2 bytes
	offset := g.stackOffset + int(reg)*2
	return offset
}

// getLocalOffset gets the stack offset for a register
func (g *Z80Generator) getLocalOffset(reg ir.Register) int {
	// For now, simple mapping
	return g.stackOffset + int(reg)*2
}

// getAbsoluteAddr gets the absolute address for a local variable
func (g *Z80Generator) getAbsoluteAddr(reg ir.Register) uint16 {
	// Each register gets 2 bytes
	return g.localVarBase + uint16(reg)*2
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
}

// NewRegisterAllocator creates a new register allocator
func NewRegisterAllocator() *RegisterAllocator {
	return &RegisterAllocator{
		allocation: make(map[ir.Register]string),
		inUse:      make(map[string]bool),
	}
}

// Reset clears the allocator state
func (r *RegisterAllocator) Reset() {
	r.allocation = make(map[ir.Register]string)
	r.inUse = make(map[string]bool)
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