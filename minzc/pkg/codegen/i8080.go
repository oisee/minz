package codegen

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/minz/minzc/pkg/ir"
)

// I8080Generator generates Intel 8080 assembly from IR
// This is a simplified version of Z80Generator without:
// - IX/IY registers
// - Shadow registers
// - DJNZ loops
// - Relative jumps (JR)
// - Bit operations
type I8080Generator struct {
	writer        io.Writer
	module        *ir.Module
	currentFunc   *ir.Function
	
	// Simple register allocation (no shadow registers)
	regAlloc      *RegisterAllocator
	
	stackOffset   int
	labelCounter  int
	localVarBase  uint16
	emittedParams map[string]bool
}

// NewI8080Generator creates a new 8080 code generator
func NewI8080Generator(w io.Writer) *I8080Generator {
	return &I8080Generator{
		writer:        w,
		regAlloc:      NewRegisterAllocator(),
		localVarBase:  0xF000, // Same as Z80
		emittedParams: make(map[string]bool),
	}
}

// Generate generates 8080 assembly for an IR module
func (g *I8080Generator) Generate(module *ir.Module) error {
	g.module = module

	// Write header
	g.writeHeader()

	// Generate data section
	if len(module.Globals) > 0 || len(module.Strings) > 0 {
		g.emit("\n; Data section")
		g.emit("    ORG 0F000H")  // 8080 style hex notation
		g.emit("")
		for _, global := range module.Globals {
			g.generateGlobal(&global)
		}
		
		// Generate string literals
		for _, str := range module.Strings {
			g.generateString(*str)
		}
	}

	// Generate code section
	g.emit("\n; Code section")
	g.emit("    ORG 08000H")
	g.emit("")

	// Generate functions
	for _, fn := range module.Functions {
		if err := g.generateFunction(fn); err != nil {
			return err
		}
	}

	// Generate runtime helpers
	g.generatePrintHelpers()
	
	// Write footer
	g.writeFooter()

	return nil
}

// writeHeader writes the assembly file header
func (g *I8080Generator) writeHeader() {
	g.emit("; MinZ 8080 generated code")
	g.emit("; Generated: %s", time.Now().Format("2006-01-02 15:04:05"))
	g.emit("; Target: Intel 8080")
	g.emit("")
}

// writeFooter writes the assembly file footer
func (g *I8080Generator) writeFooter() {
	g.emit("\n; End of generated code")
	g.emit("    END")
}

// generateGlobal generates a global variable
func (g *I8080Generator) generateGlobal(global *ir.Global) {
	g.emit("%s:", global.Name)
	// Generate appropriate storage directive based on type
	size := global.Type.Size()
	switch size {
	case 1:
		g.emit("    DB 0")
	case 2:
		g.emit("    DW 0")
	default:
		g.emit("    DS %d", size) // Reserve space
	}
}

// generateString generates a string literal
func (g *I8080Generator) generateString(str ir.String) {
	g.emit("%s:", str.Label)
	// Length-prefixed string format
	g.emit("    DB %d ; Length", len(str.Value))
	if len(str.Value) > 0 {
		// Emit string data as bytes
		bytes := []string{}
		for _, ch := range str.Value {
			bytes = append(bytes, fmt.Sprintf("%02XH", ch))
		}
		g.emit("    DB %s", strings.Join(bytes, ","))
	}
}

// generateFunction generates a function
func (g *I8080Generator) generateFunction(fn *ir.Function) error {
	g.currentFunc = fn
	g.stackOffset = 0
	g.regAlloc.Reset()

	// Function label
	g.emit("\n; Function: %s", fn.Name)
	if fn.IsSMCEnabled {
		g.emit("; SMC enabled - parameters can be self-modified")
	}
	g.emit("%s:", fn.Name)

	// Generate SMC parameter anchors
	if fn.IsSMCEnabled && len(fn.Params) > 0 {
		for i, param := range fn.Params {
			if i == 0 { // Only first parameter for now
				g.emit("%s$param_%s:", fn.Name, param.Name)
				g.emit("    MVI A,0    ; SMC anchor for %s", param.Name)
			}
		}
	}

	// Prologue
	g.generatePrologue()

	// Generate instructions
	for i, inst := range fn.Instructions {
		if err := g.generateInstruction(&inst, i); err != nil {
			return err
		}
	}

	// Epilogue (if not already returned)
	if len(fn.Instructions) == 0 || fn.Instructions[len(fn.Instructions)-1].Op != ir.OpReturn {
		g.generateEpilogue()
	}

	return nil
}

// generatePrologue generates function prologue
func (g *I8080Generator) generatePrologue() {
	// 8080 doesn't have PUSH HL etc, must use PUSH H
	// For simplicity, always save BC, DE, HL
	g.emit("    PUSH B")
	g.emit("    PUSH D")
	g.emit("    PUSH H")
}

// generateEpilogue generates function epilogue
func (g *I8080Generator) generateEpilogue() {
	// Restore registers in reverse order
	g.emit("    POP H")
	g.emit("    POP D")
	g.emit("    POP B")
	g.emit("    RET")
}

// generateInstruction generates code for a single instruction
func (g *I8080Generator) generateInstruction(inst *ir.Instruction, index int) error {
	switch inst.Op {
	case ir.OpLoadConst:
		return g.generateLoadConst(inst)
	case ir.OpLoadVar:
		return g.generateLoadVar(inst)
	case ir.OpStoreVar:
		return g.generateStoreVar(inst)
	case ir.OpLoadParam:
		return g.generateLoadParam(inst)
	case ir.OpAdd:
		return g.generateAdd(inst)
	case ir.OpSub:
		return g.generateSub(inst)
	case ir.OpMul:
		return g.generateMul(inst)
	case ir.OpCall:
		return g.generateCall(inst)
	case ir.OpReturn:
		return g.generateReturn(inst)
	case ir.OpJump:
		return g.generateJump(inst)
	case ir.OpJumpIfNot:
		return g.generateJumpIfNot(inst)
	case ir.OpLabel:
		g.emit("%s:", inst.Label)
		return nil
	case ir.OpLt, ir.OpGt, ir.OpLe, ir.OpGe, ir.OpEq, ir.OpNe:
		return g.generateComparison(inst)
	case ir.OpPrint:
		return g.generatePrint(inst)
	case ir.OpPrintU8:
		return g.generatePrintU8(inst)
	case ir.OpPrintString:
		return g.generatePrintString(inst)
	case ir.OpPrintStringDirect:
		return g.generatePrintStringDirect(inst)
	case ir.OpLoadIndex:
		return g.generateLoadIndex(inst)
	case ir.OpLoadAddr:
		return g.generateLoadAddr(inst)
	case ir.OpLoadString:
		// Load string address into HL
		if inst.Symbol != "" {
			destAddr := g.getMemoryAddr(inst.Dest)
			g.emit("    LXI H, %s    ; Load string address", inst.Symbol)
			// Store HL to memory (16-bit store)
			g.emit("    SHLD %04XH   ; Store string address", destAddr)
		}
		return nil
	default:
		return fmt.Errorf("unsupported operation: %s", inst.Op)
	}
}

// generateLoadConst loads a constant value
func (g *I8080Generator) generateLoadConst(inst *ir.Instruction) error {
	addr := g.getMemoryAddr(inst.Dest)
	
	if inst.Type != nil && inst.Type.Size() == 1 {
		// 8-bit constant
		g.emit("    MVI A,%02XH", inst.Imm)
		g.emit("    STA %04XH", addr)
	} else {
		// 16-bit constant
		g.emit("    LXI H,%04XH", inst.Imm)
		g.emit("    SHLD %04XH", addr)
	}
	
	return nil
}

// getMemoryAddr returns a memory address for a virtual register
func (g *I8080Generator) getMemoryAddr(reg ir.Register) uint16 {
	// Simple allocation: each register gets 2 bytes
	return g.localVarBase + uint16(reg)*2
}

// generateLoadVar loads a variable
func (g *I8080Generator) generateLoadVar(inst *ir.Instruction) error {
	destAddr := g.getMemoryAddr(inst.Dest)
	
	// Simple variable load
	if inst.Type != nil && inst.Type.Size() == 1 {
		g.emit("    LDA %s", inst.Symbol)
		g.emit("    STA %04XH", destAddr)
	} else {
		g.emit("    LHLD %s", inst.Symbol)
		g.emit("    SHLD %04XH", destAddr)
	}
	
	return nil
}

// generateLoadParam loads a parameter value from SMC location
func (g *I8080Generator) generateLoadParam(inst *ir.Instruction) error {
	destAddr := g.getMemoryAddr(inst.Dest)
	
	// For SMC, parameters are loaded from the patched immediate value
	if inst.Imm == 0 && g.currentFunc.IsSMCEnabled {
		// First parameter - load from SMC anchor
		g.emit("    LDA %s$param_%s+1", g.currentFunc.Name, g.currentFunc.Params[0].Name)
		g.emit("    STA %04XH", destAddr)
	} else {
		// Other parameters not yet supported in SMC mode
		g.emit("    MVI A,0    ; TODO: param %d", inst.Imm)
		g.emit("    STA %04XH", destAddr)
	}
	
	return nil
}

// generateAdd generates addition
func (g *I8080Generator) generateAdd(inst *ir.Instruction) error {
	src1Addr := g.getMemoryAddr(inst.Src1)
	src2Addr := g.getMemoryAddr(inst.Src2)
	destAddr := g.getMemoryAddr(inst.Dest)
	
	if inst.Type != nil && inst.Type.Size() == 1 {
		// 8-bit addition
		g.emit("    LDA %04XH", src1Addr)
		g.emit("    MOV B,A")
		g.emit("    LDA %04XH", src2Addr)
		g.emit("    ADD B")
		g.emit("    STA %04XH", destAddr)
	} else {
		// 16-bit addition
		g.emit("    LHLD %04XH", src1Addr)
		g.emit("    XCHG")  // DE = src1
		g.emit("    LHLD %04XH", src2Addr)
		g.emit("    DAD D")  // HL = HL + DE
		g.emit("    SHLD %04XH", destAddr)
	}
	
	return nil
}

// generateCall generates a function call
func (g *I8080Generator) generateCall(inst *ir.Instruction) error {
	// Handle arguments
	if inst.Args != nil && len(inst.Args) > 0 {
		// For SMC, patch the first parameter
		arg0Addr := g.getMemoryAddr(inst.Args[0])
		g.emit("    LDA %04XH", arg0Addr)
		g.emit("    STA %s$param_%s+1", inst.Symbol, "param") // Patch immediate value
	}
	
	g.emit("    CALL %s", inst.Symbol)
	
	// Store return value if needed
	if inst.Dest != 0 {
		destAddr := g.getMemoryAddr(inst.Dest)
		g.emit("    STA %04XH", destAddr)
	}
	
	return nil
}

// generatePrintHelpers generates print helper functions
func (g *I8080Generator) generatePrintHelpers() {
	g.emit("\n; Print helpers")
	
	// Print character (A register contains character)
	g.emit("print_char:")
	g.emit("    ; Platform-specific print routine")
	g.emit("    ; For CP/M: CALL 0005H with C=02H, E=char")
	g.emit("    MOV E,A")
	g.emit("    MVI C,02H")
	g.emit("    CALL 0005H")
	g.emit("    RET")
	
	// Print newline
	g.emit("\nprint_newline:")
	g.emit("    MVI A,0DH    ; CR")
	g.emit("    CALL print_char")
	g.emit("    MVI A,0AH    ; LF")
	g.emit("    CALL print_char")
	g.emit("    RET")
}

// Helper methods...

func (g *I8080Generator) generateStoreVar(inst *ir.Instruction) error {
	srcAddr := g.getMemoryAddr(inst.Src1)
	
	if inst.Type != nil && inst.Type.Size() == 1 {
		g.emit("    LDA %04XH", srcAddr)
		g.emit("    STA %s", inst.Symbol)
	} else {
		g.emit("    LHLD %04XH", srcAddr)
		g.emit("    SHLD %s", inst.Symbol)
	}
	
	return nil
}

func (g *I8080Generator) generateSub(inst *ir.Instruction) error {
	src1Addr := g.getMemoryAddr(inst.Src1)
	src2Addr := g.getMemoryAddr(inst.Src2)
	destAddr := g.getMemoryAddr(inst.Dest)
	
	g.emit("    LDA %04XH", src1Addr)
	g.emit("    MOV B,A")
	g.emit("    LDA %04XH", src2Addr)
	g.emit("    SUB B")
	g.emit("    STA %04XH", destAddr)
	
	return nil
}

func (g *I8080Generator) generateMul(inst *ir.Instruction) error {
	// 8080 has no multiply - must use subroutine
	src1Addr := g.getMemoryAddr(inst.Src1)
	src2Addr := g.getMemoryAddr(inst.Src2)
	destAddr := g.getMemoryAddr(inst.Dest)
	
	g.emit("    LDA %04XH", src1Addr)
	g.emit("    MOV B,A")
	g.emit("    LDA %04XH", src2Addr)
	g.emit("    CALL multiply_8x8")
	g.emit("    STA %04XH", destAddr)
	
	// Generate multiply routine if not already done
	if !g.emittedParams["multiply_8x8"] {
		g.emittedParams["multiply_8x8"] = true
		g.emit("\n; 8x8 multiply routine")
		g.emit("multiply_8x8:")
		g.emit("    ; A = multiplicand, B = multiplier")
		g.emit("    MOV C,A")
		g.emit("    XRA A")
		g.emit("mult_loop:")
		g.emit("    ADD C")
		g.emit("    DCR B")
		g.emit("    JNZ mult_loop")
		g.emit("    RET")
	}
	
	return nil
}

func (g *I8080Generator) generateReturn(inst *ir.Instruction) error {
	if inst.Src1 != 0 {
		srcAddr := g.getMemoryAddr(inst.Src1)
		g.emit("    LDA %04XH", srcAddr)
	}
	g.generateEpilogue()
	return nil
}

func (g *I8080Generator) generateJump(inst *ir.Instruction) error {
	g.emit("    JMP %s", inst.Label)
	return nil
}

func (g *I8080Generator) generateJumpIfNot(inst *ir.Instruction) error {
	condAddr := g.getMemoryAddr(inst.Src1)
	g.emit("    LDA %04XH", condAddr)
	g.emit("    ORA A")
	g.emit("    JZ %s", inst.Label)
	return nil
}

func (g *I8080Generator) generateComparison(inst *ir.Instruction) error {
	src1Addr := g.getMemoryAddr(inst.Src1)
	src2Addr := g.getMemoryAddr(inst.Src2)
	destAddr := g.getMemoryAddr(inst.Dest)
	
	// Load operands
	g.emit("    LDA %04XH", src1Addr)
	g.emit("    MOV B,A")
	g.emit("    LDA %04XH", src2Addr)
	g.emit("    CMP B")  // Compare A with B
	
	// Generate appropriate conditional jump
	label := g.newLabel()
	switch inst.Op {
	case ir.OpLt:
		g.emit("    JC true_%s", label)
	case ir.OpLe:
		g.emit("    JC true_%s", label)
		g.emit("    JZ true_%s", label)
	case ir.OpGt:
		g.emit("    JNC skip_%s", label)
		g.emit("    JNZ true_%s", label)
		g.emit("skip_%s:", label)
	case ir.OpGe:
		g.emit("    JNC true_%s", label)
	case ir.OpEq:
		g.emit("    JZ true_%s", label)
	case ir.OpNe:
		g.emit("    JNZ true_%s", label)
	}
	
	// False case
	g.emit("    XRA A")
	g.emit("    JMP end_%s", label)
	
	// True case
	g.emit("true_%s:", label)
	g.emit("    MVI A,1")
	
	g.emit("end_%s:", label)
	g.emit("    STA %04XH", destAddr)
	
	return nil
}

func (g *I8080Generator) generatePrint(inst *ir.Instruction) error {
	g.emit("    CALL print_char")
	return nil
}

func (g *I8080Generator) generatePrintU8(inst *ir.Instruction) error {
	srcAddr := g.getMemoryAddr(inst.Src1)
	g.emit("    LDA %04XH", srcAddr)
	g.emit("    CALL print_hex")
	
	// Generate print_hex if not already done
	if !g.emittedParams["print_hex"] {
		g.emittedParams["print_hex"] = true
		g.emit("\n; Print hex byte")
		g.emit("print_hex:")
		g.emit("    PUSH PSW")
		g.emit("    RRC")
		g.emit("    RRC")
		g.emit("    RRC")
		g.emit("    RRC")
		g.emit("    CALL print_nibble")
		g.emit("    POP PSW")
		g.emit("    CALL print_nibble")
		g.emit("    RET")
		
		g.emit("\nprint_nibble:")
		g.emit("    ANI 0FH")
		g.emit("    CPI 0AH")
		g.emit("    JC digit")
		g.emit("    ADI 37H  ; 'A'-10")
		g.emit("    JMP print_char")
		g.emit("digit:")
		g.emit("    ADI 30H  ; '0'")
		g.emit("    JMP print_char")
	}
	
	return nil
}

func (g *I8080Generator) generatePrintString(inst *ir.Instruction) error {
	// Handle printing a string from a register (address in register)
	if inst.Src1 != 0 {
		srcAddr := g.getMemoryAddr(inst.Src1)
		g.emit("    LHLD %04XH   ; Load string address", srcAddr)
		g.emit("    CALL print_string")
	} else if inst.Symbol != "" {
		// Direct string label
		g.emit("    LXI H,%s", inst.Symbol)
		g.emit("    CALL print_string")
	}

	// Generate print_string helper if not already done
	if !g.emittedParams["print_string"] {
		g.emittedParams["print_string"] = true
		g.emit("\n; Print string")
		g.emit("print_string:")
		g.emit("    MOV A,M      ; Get length")
		g.emit("    ORA A")
		g.emit("    RZ           ; Return if zero")
		g.emit("    MOV B,A")
		g.emit("    INX H")
		g.emit("ps_loop:")
		g.emit("    MOV A,M")
		g.emit("    CALL print_char")
		g.emit("    INX H")
		g.emit("    DCR B")
		g.emit("    JNZ ps_loop")
		g.emit("    RET")
	}

	return nil
}

func (g *I8080Generator) generatePrintStringDirect(inst *ir.Instruction) error {
	g.emit("    LXI H,%s", inst.Symbol)
	g.emit("    CALL print_string")
	
	// Generate print_string if not already done
	if !g.emittedParams["print_string"] {
		g.emittedParams["print_string"] = true
		g.emit("\n; Print string")
		g.emit("print_string:")
		g.emit("    MOV A,M      ; Get length")
		g.emit("    ORA A")
		g.emit("    RZ           ; Return if zero")
		g.emit("    MOV B,A")
		g.emit("    INX H")
		g.emit("ps_loop:")
		g.emit("    MOV A,M")
		g.emit("    CALL print_char")
		g.emit("    INX H")
		g.emit("    DCR B")
		g.emit("    JNZ ps_loop")
		g.emit("    RET")
	}
	
	return nil
}

// generateLoadIndex generates array indexing
func (g *I8080Generator) generateLoadIndex(inst *ir.Instruction) error {
	arrayAddr := g.getMemoryAddr(inst.Src1)
	indexAddr := g.getMemoryAddr(inst.Src2)
	destAddr := g.getMemoryAddr(inst.Dest)
	
	// Load array pointer to HL
	g.emit("    LHLD %04XH", arrayAddr)
	g.emit("    PUSH H")  // Save array pointer
	
	// Load index to DE
	if inst.Type != nil && inst.Type.Size() == 1 {
		// For single byte elements
		g.emit("    LDA %04XH", indexAddr)
		g.emit("    MOV E,A")
		g.emit("    MVI D,0")
	} else {
		// For multi-byte elements  
		g.emit("    LHLD %04XH", indexAddr)
		g.emit("    XCHG")  // Move index to DE
	}
	
	// Restore array pointer
	g.emit("    POP H")
	
	// Calculate address: array + index
	// For now assuming byte arrays (element size = 1)
	// TODO: Handle different element sizes  
	g.emit("    DAD D")  // HL = HL + DE (array + index)
	
	// Load value from array[index]
	g.emit("    MOV A,M")  // A = (HL)
	g.emit("    STA %04XH", destAddr)
	
	return nil
}

// generateLoadAddr generates address loading
func (g *I8080Generator) generateLoadAddr(inst *ir.Instruction) error {
	destAddr := g.getMemoryAddr(inst.Dest)
	
	if inst.Symbol != "" {
		// Load address of named variable/array
		g.emit("    LXI H,%s", inst.Symbol)
		g.emit("    SHLD %04XH", destAddr)
	} else {
		// Load address from register (for nested arrays)
		srcAddr := g.getMemoryAddr(inst.Src1)
		g.emit("    LHLD %04XH", srcAddr)
		g.emit("    SHLD %04XH", destAddr)
	}
	
	return nil
}

// emit writes a line to the output
func (g *I8080Generator) emit(format string, args ...interface{}) {
	if len(args) > 0 {
		fmt.Fprintf(g.writer, format+"\n", args...)
	} else {
		fmt.Fprintf(g.writer, format+"\n")
	}
}

// newLabel generates a unique label
func (g *I8080Generator) newLabel() string {
	g.labelCounter++
	return fmt.Sprintf("L%d", g.labelCounter)
}