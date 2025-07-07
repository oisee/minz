package codegen

import (
	"fmt"
	"io"
	"time"

	"github.com/minz/minzc/pkg/ir"
)

// Z80Generator generates Z80 assembly from IR
type Z80Generator struct {
	writer        io.Writer
	module        *ir.Module
	currentFunc   *ir.Function
	regAlloc      *RegisterAllocator
	stackOffset   int
	labelCounter  int
	useShadowRegs bool // Whether to use shadow registers for current function
}

// NewZ80Generator creates a new Z80 code generator
func NewZ80Generator(w io.Writer) *Z80Generator {
	return &Z80Generator{
		writer:   w,
		regAlloc: NewRegisterAllocator(),
	}
}

// Generate generates Z80 assembly for an IR module
func (g *Z80Generator) Generate(module *ir.Module) error {
	g.module = module

	// Write header
	g.writeHeader()

	// Generate data section
	if len(module.Globals) > 0 {
		g.emit("\n; Data section")
		for _, global := range module.Globals {
			g.generateGlobal(global)
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

// generateFunction generates code for a function
func (g *Z80Generator) generateFunction(fn *ir.Function) error {
	g.currentFunc = fn
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
	for _, inst := range fn.Instructions {
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

// generateSMCFunction generates an SMC-based function
func (g *Z80Generator) generateSMCFunction(fn *ir.Function) error {
	g.emit("%s:", fn.Name)
	
	// Generate SMC parameter slots
	for _, param := range fn.Params {
		offset := fn.SMCParamOffsets[param.Name]
		paramLabel := fmt.Sprintf("%s_param_%s", fn.Name, param.Name)
		g.emit("%s EQU %s + %d", paramLabel, fn.Name, offset)
		
		// Emit the actual parameter instruction
		if param.Type.Size() == 1 {
			g.emit("    LD A, #00      ; Parameter %s", param.Name)
		} else {
			g.emit("    LD HL, #0000   ; Parameter %s", param.Name)
		}
	}
	
	// For recursive functions, we need special handling
	if fn.IsRecursive && fn.RequiresContext {
		g.emit("    ; Recursive function - will need context save/restore")
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
		
	case ir.OpLoadParam:
		// For SMC, parameters are already in immediate values
		// Just reference the SMC location
		paramName := inst.Symbol
		paramLabel := fmt.Sprintf("%s_param_%s", g.currentFunc.Name, paramName)
		
		if inst.Type != nil && inst.Type.Size() == 1 {
			g.emit("    LD A, (%s)", paramLabel)
			g.storeFromA(inst.Dest)
		} else {
			g.emit("    LD HL, (%s)", paramLabel)
			g.storeFromHL(inst.Dest)
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
		
	case ir.OpReturn:
		if inst.Src1 != 0 {
			// Load return value to HL (Z80 convention)
			g.loadToHL(inst.Src1)
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
		offset := g.getLocalOffset(inst.Dest)
		g.emit("    LD L, (IX-%d)", offset)
		g.emit("    LD H, (IX-%d)", offset-1)
		g.storeFromHL(inst.Dest)
		
	case ir.OpStoreVar:
		// Store to variable
		g.loadToHL(inst.Src1)
		offset := g.getLocalOffset(inst.Dest)
		g.emit("    LD (IX-%d), L", offset)
		g.emit("    LD (IX-%d), H", offset-1)
		
	case ir.OpAdd:
		// Load operands
		g.loadToHL(inst.Src1)
		g.emit("    PUSH HL")
		g.loadToHL(inst.Src2)
		g.emit("    POP DE")
		g.emit("    ADD HL, DE")
		g.storeFromHL(inst.Dest)
		
	case ir.OpSub:
		// HL = Src1 - Src2
		g.loadToHL(inst.Src1)
		g.emit("    PUSH HL")
		g.loadToHL(inst.Src2)
		g.emit("    POP DE")
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
		
	case ir.OpAnd:
		// Bitwise AND
		g.loadToHL(inst.Src1)
		g.emit("    PUSH HL")
		g.loadToHL(inst.Src2)
		g.emit("    POP DE")
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
		g.emit("    PUSH HL")
		g.loadToHL(inst.Src2)
		g.emit("    POP DE")
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
			g.emit("    PUSH HL")
			g.loadToHL(inst.Src2)
			g.emit("    POP DE")
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
		// Push arguments (in reverse order for C calling convention)
		// TODO: Handle arguments properly
		g.emit("    CALL %s", inst.Symbol)
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
		
	default:
		return fmt.Errorf("unsupported opcode: %v", inst.Op)
	}

	return nil
}

// generateComparison generates code for comparison operations
func (g *Z80Generator) generateComparison(inst ir.Instruction) {
	// Load operands
	g.loadToHL(inst.Src1)
	g.emit("    PUSH HL")
	g.loadToHL(inst.Src2)
	g.emit("    POP DE")
	
	// Compare HL with DE
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
	
	offset := g.getLocalOffset(reg)
	g.emit("    LD A, (IX-%d)", offset)
}

// storeFromA stores A to a virtual register
func (g *Z80Generator) storeFromA(reg ir.Register) {
	offset := g.getLocalOffset(reg)
	g.emit("    LD (IX-%d), A", offset)
}

// loadToHL loads a virtual register to HL
func (g *Z80Generator) loadToHL(reg ir.Register) {
	if reg == ir.RegZero {
		g.emit("    LD HL, 0")
		return
	}
	
	offset := g.getLocalOffset(reg)
	g.emit("    LD L, (IX-%d)", offset)
	g.emit("    LD H, (IX-%d)", offset-1)
}

// storeFromHL stores HL to a virtual register
func (g *Z80Generator) storeFromHL(reg ir.Register) {
	offset := g.getLocalOffset(reg)
	g.emit("    LD (IX-%d), L", offset)
	g.emit("    LD (IX-%d), H", offset-1)
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