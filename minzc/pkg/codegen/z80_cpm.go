package codegen

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/minz/minzc/pkg/ir"
)

// Z80CPMGenerator generates Z80 assembly with CP/M BDOS system calls
// Uses Z80 syntax (not i8080) but targets CP/M environment
type Z80CPMGenerator struct {
	writer        io.Writer
	module        *ir.Module
	currentFunc   *ir.Function
	
	// Register allocation (simplified for CP/M)
	regAlloc      *RegisterAllocator
	
	stackOffset   int
	labelCounter  int
	localVarBase  uint16
	emittedParams map[string]bool
}

// NewZ80CPMGenerator creates a new Z80 CP/M code generator
func NewZ80CPMGenerator(w io.Writer) *Z80CPMGenerator {
	return &Z80CPMGenerator{
		writer:        w,
		regAlloc:      NewRegisterAllocator(),
		localVarBase:  0xF000, // Same as standard Z80
		emittedParams: make(map[string]bool),
	}
}

// Generate generates Z80 assembly for CP/M environment
func (g *Z80CPMGenerator) Generate(module *ir.Module) error {
	g.module = module

	// Write header
	g.writeHeader()

	// Generate data section
	if len(module.Globals) > 0 || len(module.Strings) > 0 {
		g.emit("\n; Data section")
		g.emit("    ORG 0F000H")  // High memory for data
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
	g.emit("    ORG 8000H")      // CP/M TPA start
	g.emit("")

	// Generate functions
	for _, fn := range module.Functions {
		if err := g.generateFunction(fn); err != nil {
			return err
		}
	}

	// Generate CP/M runtime helpers
	g.generateCPMHelpers()
	
	// Write footer
	g.writeFooter()

	return nil
}

// writeHeader writes the assembly file header
func (g *Z80CPMGenerator) writeHeader() {
	g.emit("; MinZ Z80 CP/M generated code")
	g.emit("; Generated: %s", time.Now().Format("2006-01-02 15:04:05"))
	g.emit("; Target: Z80 CPU with CP/M BDOS")
	g.emit("; Syntax: Z80 (not i8080)")
	g.emit("")
}

// writeFooter writes the assembly file footer
func (g *Z80CPMGenerator) writeFooter() {
	g.emit("\n; CP/M program termination")
	g.emit("terminate:")
	g.emit("    LD C, 0         ; Function 0: System reset")
	g.emit("    JP 0005H        ; Jump to BDOS")
	g.emit("")
	g.emit("    END")
}

// generateCPMHelpers generates CP/M-specific helper functions
func (g *Z80CPMGenerator) generateCPMHelpers() {
	g.emit("\n; CP/M BDOS Helper Functions")
	
	// Print character using BDOS function 2
	g.emit("\nprint_char:")
	g.emit("    LD E, A         ; Character to print in E")
	g.emit("    LD C, 2         ; BDOS function 2: Console output")
	g.emit("    CALL 0005H      ; Call BDOS")
	g.emit("    RET")
	
	// Print newline (CR + LF)
	g.emit("\nprint_newline:")
	g.emit("    LD A, 0DH       ; Carriage return")
	g.emit("    CALL print_char")
	g.emit("    LD A, 0AH       ; Line feed")
	g.emit("    CALL print_char")
	g.emit("    RET")
	
	// Print hexadecimal byte
	g.emit("\nprint_hex:")
	g.emit("    PUSH AF")
	g.emit("    RRA")
	g.emit("    RRA")
	g.emit("    RRA")
	g.emit("    RRA")
	g.emit("    CALL print_nibble")
	g.emit("    POP AF")
	g.emit("    CALL print_nibble")
	g.emit("    RET")
	
	g.emit("\nprint_nibble:")
	g.emit("    AND 0FH")
	g.emit("    CP 0AH")
	g.emit("    JR C, digit")
	g.emit("    ADD A, 37H      ; 'A' - 10")
	g.emit("    JR print_char")
	g.emit("digit:")
	g.emit("    ADD A, 30H      ; '0'")
	g.emit("    JR print_char")
	
	// Print string (length-prefixed MinZ format)
	g.emit("\nprint_string:")
	g.emit("    LD A, (HL)      ; Get length")
	g.emit("    OR A")
	g.emit("    RET Z           ; Return if zero length")
	g.emit("    LD B, A         ; Length in B")
	g.emit("    INC HL          ; Point to first character")
	g.emit("ps_loop:")
	g.emit("    LD A, (HL)")
	g.emit("    CALL print_char")
	g.emit("    INC HL")
	g.emit("    DJNZ ps_loop    ; Z80 DJNZ - perfect for strings!")
	g.emit("    RET")
	
	// Print decimal (u8)
	g.emit("\nprint_decimal:")
	g.emit("    LD B, 100")
	g.emit("    CALL print_digit")
	g.emit("    LD B, 10")
	g.emit("    CALL print_digit")
	g.emit("    ADD A, 30H      ; Convert remainder to ASCII")
	g.emit("    JR print_char")
	
	g.emit("\nprint_digit:")
	g.emit("    LD C, 0         ; Digit counter")
	g.emit("digit_loop:")
	g.emit("    CP B")
	g.emit("    JR C, digit_done")
	g.emit("    SUB B")
	g.emit("    INC C")
	g.emit("    JR digit_loop")
	g.emit("digit_done:")
	g.emit("    PUSH AF")
	g.emit("    LD A, C")
	g.emit("    ADD A, 30H")
	g.emit("    CALL print_char")
	g.emit("    POP AF")
	g.emit("    RET")
}

// generateFunction generates a function with Z80 syntax
func (g *Z80CPMGenerator) generateFunction(fn *ir.Function) error {
	g.currentFunc = fn
	g.stackOffset = 0
	g.regAlloc.Reset()

	// Function label
	g.emit("\n; Function: %s", fn.Name)
	if fn.IsSMCEnabled {
		g.emit("; SMC enabled - Z80 CP/M optimized")
	}
	g.emit("%s:", fn.Name)

	// Generate SMC parameter anchors (Z80 style)
	if fn.IsSMCEnabled && len(fn.Params) > 0 {
		for i, param := range fn.Params {
			if i == 0 { // Only first parameter for now
				g.emit("%s_param_%s:", fn.Name, param.Name)
				g.emit("    LD A, 0         ; SMC anchor for %s", param.Name)
			}
		}
	}

	// Prologue (Z80 style)
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

// generatePrologue generates function prologue (Z80 style)
func (g *Z80CPMGenerator) generatePrologue() {
	// Z80 can push 16-bit registers directly
	g.emit("    PUSH BC")
	g.emit("    PUSH DE")
	g.emit("    PUSH HL")
}

// generateEpilogue generates function epilogue (Z80 style)
func (g *Z80CPMGenerator) generateEpilogue() {
	// Restore in reverse order
	g.emit("    POP HL")
	g.emit("    POP DE")
	g.emit("    POP BC")
	g.emit("    RET")
}

// generateInstruction generates code for a single instruction (Z80 optimized)
func (g *Z80CPMGenerator) generateInstruction(inst *ir.Instruction, index int) error {
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
	case ir.OpLoadString:
		return g.generateLoadString(inst)
	default:
		return fmt.Errorf("unsupported operation: %s", inst.Op)
	}
}

// generateLoadConst loads a constant (Z80 optimized)
func (g *Z80CPMGenerator) generateLoadConst(inst *ir.Instruction) error {
	addr := g.getMemoryAddr(inst.Dest)
	
	if inst.Type != nil && inst.Type.Size() == 1 {
		// 8-bit constant
		g.emit("    LD A, %02XH", inst.Imm)
		g.emit("    LD (%04XH), A", addr)
	} else {
		// 16-bit constant (Z80 direct)
		g.emit("    LD HL, %04XH", inst.Imm)
		g.emit("    LD (%04XH), HL", addr)
	}
	
	return nil
}

// Z80-specific optimized implementations
func (g *Z80CPMGenerator) generateAdd(inst *ir.Instruction) error {
	src1Addr := g.getMemoryAddr(inst.Src1)
	src2Addr := g.getMemoryAddr(inst.Src2)
	destAddr := g.getMemoryAddr(inst.Dest)
	
	if inst.Type != nil && inst.Type.Size() == 1 {
		// 8-bit addition (Z80 optimized)
		g.emit("    LD A, (%04XH)", src1Addr)
		g.emit("    LD B, (%04XH)", src2Addr)
		g.emit("    ADD A, B")
		g.emit("    LD (%04XH), A", destAddr)
	} else {
		// 16-bit addition (Z80 ADD HL, DE)
		g.emit("    LD HL, (%04XH)", src1Addr)
		g.emit("    LD DE, (%04XH)", src2Addr)
		g.emit("    ADD HL, DE")
		g.emit("    LD (%04XH), HL", destAddr)
	}
	
	return nil
}

// Print implementations using CP/M BDOS
func (g *Z80CPMGenerator) generatePrint(inst *ir.Instruction) error {
	srcAddr := g.getMemoryAddr(inst.Src1)
	g.emit("    LD A, (%04XH)", srcAddr)
	g.emit("    CALL print_char")
	return nil
}

func (g *Z80CPMGenerator) generatePrintU8(inst *ir.Instruction) error {
	srcAddr := g.getMemoryAddr(inst.Src1)
	g.emit("    LD A, (%04XH)", srcAddr)
	g.emit("    CALL print_decimal")
	return nil
}

func (g *Z80CPMGenerator) generatePrintString(inst *ir.Instruction) error {
	if inst.Src1 != 0 {
		srcAddr := g.getMemoryAddr(inst.Src1)
		g.emit("    LD HL, (%04XH)", srcAddr)
	} else if inst.Symbol != "" {
		g.emit("    LD HL, %s", inst.Symbol)
	}
	g.emit("    CALL print_string")
	return nil
}

func (g *Z80CPMGenerator) generatePrintStringDirect(inst *ir.Instruction) error {
	g.emit("    LD HL, %s", inst.Symbol)
	g.emit("    CALL print_string")
	return nil
}

func (g *Z80CPMGenerator) generateLoadString(inst *ir.Instruction) error {
	if inst.Symbol != "" {
		destAddr := g.getMemoryAddr(inst.Dest)
		g.emit("    LD HL, %s", inst.Symbol)
		g.emit("    LD (%04XH), HL", destAddr)
	}
	return nil
}

// Utility methods (similar to i8080 but Z80 syntax)
func (g *Z80CPMGenerator) getMemoryAddr(reg ir.Register) uint16 {
	return g.localVarBase + uint16(reg)*2
}

func (g *Z80CPMGenerator) generateGlobal(global *ir.Global) {
	g.emit("%s:", global.Name)
	size := global.Type.Size()
	switch size {
	case 1:
		g.emit("    DB 0")
	case 2:
		g.emit("    DW 0")
	default:
		g.emit("    DS %d", size)
	}
}

func (g *Z80CPMGenerator) generateString(str ir.String) {
	g.emit("%s:", str.Label)
	g.emit("    DB %d ; Length", len(str.Value))
	if len(str.Value) > 0 {
		bytes := []string{}
		for _, ch := range str.Value {
			bytes = append(bytes, fmt.Sprintf("%02XH", ch))
		}
		g.emit("    DB %s", strings.Join(bytes, ","))
	}
}

// Remaining implementations...
func (g *Z80CPMGenerator) generateLoadVar(inst *ir.Instruction) error {
	destAddr := g.getMemoryAddr(inst.Dest)
	if inst.Type != nil && inst.Type.Size() == 1 {
		g.emit("    LD A, (%s)", inst.Symbol)
		g.emit("    LD (%04XH), A", destAddr)
	} else {
		g.emit("    LD HL, (%s)", inst.Symbol)
		g.emit("    LD (%04XH), HL", destAddr)
	}
	return nil
}

func (g *Z80CPMGenerator) generateStoreVar(inst *ir.Instruction) error {
	srcAddr := g.getMemoryAddr(inst.Src1)
	if inst.Type != nil && inst.Type.Size() == 1 {
		g.emit("    LD A, (%04XH)", srcAddr)
		g.emit("    LD (%s), A", inst.Symbol)
	} else {
		g.emit("    LD HL, (%04XH)", srcAddr)
		g.emit("    LD (%s), HL", inst.Symbol)
	}
	return nil
}

func (g *Z80CPMGenerator) generateLoadParam(inst *ir.Instruction) error {
	destAddr := g.getMemoryAddr(inst.Dest)
	if inst.Imm == 0 && g.currentFunc.IsSMCEnabled {
		g.emit("    LD A, (%s_param_%s+1)", g.currentFunc.Name, g.currentFunc.Params[0].Name)
		g.emit("    LD (%04XH), A", destAddr)
	} else {
		g.emit("    LD A, 0    ; TODO: param %d", inst.Imm)
		g.emit("    LD (%04XH), A", destAddr)
	}
	return nil
}

func (g *Z80CPMGenerator) generateSub(inst *ir.Instruction) error {
	src1Addr := g.getMemoryAddr(inst.Src1)
	src2Addr := g.getMemoryAddr(inst.Src2)
	destAddr := g.getMemoryAddr(inst.Dest)
	
	g.emit("    LD A, (%04XH)", src1Addr)
	g.emit("    LD B, (%04XH)", src2Addr)
	g.emit("    SUB B")
	g.emit("    LD (%04XH), A", destAddr)
	
	return nil
}

func (g *Z80CPMGenerator) generateMul(inst *ir.Instruction) error {
	// Z80 multiply routine (similar to i8080 but Z80 syntax)
	src1Addr := g.getMemoryAddr(inst.Src1)
	src2Addr := g.getMemoryAddr(inst.Src2)
	destAddr := g.getMemoryAddr(inst.Dest)
	
	g.emit("    LD A, (%04XH)", src1Addr)
	g.emit("    LD B, (%04XH)", src2Addr)
	g.emit("    CALL multiply_8x8")
	g.emit("    LD (%04XH), A", destAddr)
	
	// Generate multiply routine if not already done
	if !g.emittedParams["multiply_8x8"] {
		g.emittedParams["multiply_8x8"] = true
		g.emit("\n; 8x8 Z80 multiply routine")
		g.emit("multiply_8x8:")
		g.emit("    LD C, A")
		g.emit("    XOR A")
		g.emit("mult_loop:")
		g.emit("    ADD A, C")
		g.emit("    DJNZ mult_loop  ; Z80 DJNZ is perfect here!")
		g.emit("    RET")
	}
	
	return nil
}

func (g *Z80CPMGenerator) generateCall(inst *ir.Instruction) error {
	// Handle SMC parameter patching
	if inst.Args != nil && len(inst.Args) > 0 {
		arg0Addr := g.getMemoryAddr(inst.Args[0])
		g.emit("    LD A, (%04XH)", arg0Addr)
		g.emit("    LD (%s_param_param+1), A", inst.Symbol)
	}
	
	g.emit("    CALL %s", inst.Symbol)
	
	// Store return value
	if inst.Dest != 0 {
		destAddr := g.getMemoryAddr(inst.Dest)
		g.emit("    LD (%04XH), A", destAddr)
	}
	
	return nil
}

func (g *Z80CPMGenerator) generateReturn(inst *ir.Instruction) error {
	if inst.Src1 != 0 {
		srcAddr := g.getMemoryAddr(inst.Src1)
		g.emit("    LD A, (%04XH)", srcAddr)
	}
	g.generateEpilogue()
	return nil
}

func (g *Z80CPMGenerator) generateJump(inst *ir.Instruction) error {
	g.emit("    JP %s", inst.Label)  // Z80 JP (not JMP)
	return nil
}

func (g *Z80CPMGenerator) generateJumpIfNot(inst *ir.Instruction) error {
	condAddr := g.getMemoryAddr(inst.Src1)
	g.emit("    LD A, (%04XH)", condAddr)
	g.emit("    OR A")
	g.emit("    JP Z, %s", inst.Label)  // Z80 JP (not JZ)
	return nil
}

func (g *Z80CPMGenerator) generateComparison(inst *ir.Instruction) error {
	src1Addr := g.getMemoryAddr(inst.Src1)
	src2Addr := g.getMemoryAddr(inst.Src2)
	destAddr := g.getMemoryAddr(inst.Dest)
	
	// Load operands
	g.emit("    LD A, (%04XH)", src1Addr)
	g.emit("    LD B, (%04XH)", src2Addr)
	g.emit("    CP B")
	
	// Generate Z80 conditional jumps
	label := g.newLabel()
	switch inst.Op {
	case ir.OpLt:
		g.emit("    JP C, true_%s", label)
	case ir.OpLe:
		g.emit("    JP C, true_%s", label)
		g.emit("    JP Z, true_%s", label)
	case ir.OpGt:
		g.emit("    JP NC, skip_%s", label)
		g.emit("    JP NZ, true_%s", label)
		g.emit("skip_%s:", label)
	case ir.OpGe:
		g.emit("    JP NC, true_%s", label)
	case ir.OpEq:
		g.emit("    JP Z, true_%s", label)
	case ir.OpNe:
		g.emit("    JP NZ, true_%s", label)
	}
	
	// False case
	g.emit("    XOR A")
	g.emit("    JP end_%s", label)
	
	// True case
	g.emit("true_%s:", label)
	g.emit("    LD A, 1")
	
	g.emit("end_%s:", label)
	g.emit("    LD (%04XH), A", destAddr)
	
	return nil
}

func (g *Z80CPMGenerator) emit(format string, args ...interface{}) {
	if len(args) > 0 {
		fmt.Fprintf(g.writer, format+"\n", args...)
	} else {
		fmt.Fprintf(g.writer, format+"\n")
	}
}

func (g *Z80CPMGenerator) newLabel() string {
	g.labelCounter++
	return fmt.Sprintf("L%d", g.labelCounter)
}

// Register the Z80 CP/M backend
func init() {
	RegisterBackend("z80cpm", func(options *BackendOptions) Backend {
		return &Z80CPMBackend{
			generator: NewZ80CPMGenerator(nil),
			options:   options,
		}
	})
}

// Z80CPMBackend implements the Backend interface
type Z80CPMBackend struct {
	generator *Z80CPMGenerator
	options   *BackendOptions
}

func (b *Z80CPMBackend) Name() string {
	return "z80cpm"
}

func (b *Z80CPMBackend) GetFileExtension() string {
	return ".z80"
}

func (b *Z80CPMBackend) SupportsFeature(feature string) bool {
	switch feature {
	case FeatureSelfModifyingCode:
		return true // Z80 SMC support
	case FeatureInterrupts:
		return true // Z80 interrupt support
	case FeatureShadowRegisters:
		return true // Z80 shadow registers
	case Feature16BitPointers:
		return true // Z80 is 16-bit
	case FeatureHardwareMultiply:
		return false // Z80 has no MUL
	case FeatureHardwareDivide:
		return false // Z80 has no DIV
	default:
		return false
	}
}

func (b *Z80CPMBackend) Generate(module *ir.Module) (string, error) {
	var buf strings.Builder
	b.generator.writer = &buf
	err := b.generator.Generate(module)
	return buf.String(), err
}