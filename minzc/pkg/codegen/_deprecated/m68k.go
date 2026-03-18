package codegen

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/minz/minzc/pkg/ir"
)

// M68kGenerator generates Motorola 68000 assembly from IR
type M68kGenerator struct {
	writer        io.Writer
	module        *ir.Module
	currentFunc   *ir.Function
	
	// Register allocation
	dataRegs      []string // D0-D7 for data
	addrRegs      []string // A0-A6 for addresses (A7 is SP)
	regAlloc      map[ir.Register]string // Virtual to physical mapping
	
	stackOffset   int
	labelCounter  int
	emittedParams map[string]bool
}

// NewM68kGenerator creates a new 68000 code generator
func NewM68kGenerator(w io.Writer) *M68kGenerator {
	return &M68kGenerator{
		writer:        w,
		dataRegs:      []string{"d0", "d1", "d2", "d3", "d4", "d5", "d6", "d7"},
		addrRegs:      []string{"a0", "a1", "a2", "a3", "a4", "a5", "a6"}, // a7 is SP
		regAlloc:      make(map[ir.Register]string),
		emittedParams: make(map[string]bool),
	}
}

// Generate generates 68000 assembly for an IR module
func (g *M68kGenerator) Generate(module *ir.Module) error {
	g.module = module

	// Write header
	g.writeHeader()

	// Generate data section
	if len(module.Globals) > 0 || len(module.Strings) > 0 {
		g.emit("\t.data")
		g.emit("")
		for _, global := range module.Globals {
			g.generateGlobal(global)
		}
		
		// Generate string literals
		for _, str := range module.Strings {
			g.generateString(*str)
		}
	}

	// Generate code section
	g.emit("\n\t.text")
	g.emit("\t.global _start")
	g.emit("")

	// Generate functions
	for _, fn := range module.Functions {
		if err := g.generateFunction(fn); err != nil {
			return err
		}
	}

	// Generate runtime helpers
	g.generatePrintHelpers()
	
	// Entry point
	g.emit("\n_start:")
	g.emit("\tjsr main")
	g.emit("\tmove.l #0,d0")
	g.emit("\ttrap #0\t\t| Exit")

	return nil
}

// writeHeader writes the assembly file header
func (g *M68kGenerator) writeHeader() {
	g.emit("| MinZ 68000 generated code")
	g.emit("| Generated: %s", time.Now().Format("2006-01-02 15:04:05"))
	g.emit("| Target: Motorola 68000/68010/68020/68030/68040/68060")
	g.emit("| Assembler: vasm/gas compatible")
	g.emit("")
}

// generateGlobal generates a global variable
func (g *M68kGenerator) generateGlobal(global ir.Global) {
	g.emit("%s:", global.Name)
	size := global.Type.Size()
	
	switch size {
	case 1:
		g.emit("\t.byte 0")
	case 2:
		g.emit("\t.word 0")
	case 4:
		g.emit("\t.long 0")
	default:
		g.emit("\t.space %d", size)
	}
}

// generateString generates a string literal
func (g *M68kGenerator) generateString(str ir.String) {
	g.emit("%s:", str.Label)
	// Length-prefixed string
	g.emit("\t.byte %d\t\t| Length", len(str.Value))
	if len(str.Value) > 0 {
		escaped := strings.ReplaceAll(str.Value, "\"", "\\\"")
		g.emit("\t.ascii \"%s\"", escaped)
	}
	g.emit("\t.align 2\t\t| Word align")
}

// generateFunction generates a function
func (g *M68kGenerator) generateFunction(fn *ir.Function) error {
	g.currentFunc = fn
	g.stackOffset = 0
	g.regAlloc = make(map[ir.Register]string)

	// Function label
	g.emit("\n| Function: %s", fn.Name)
	if fn.IsSMCEnabled {
		g.emit("| SMC enabled - parameters can be patched")
	}
	g.emit("%s:", fn.Name)

	// Prologue
	g.generatePrologue()

	// Allocate registers for locals
	g.allocateRegisters()

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
func (g *M68kGenerator) generatePrologue() {
	// Save frame pointer and set up new frame
	frameSize := 0
	if g.currentFunc != nil {
		// Reserve space for locals
		frameSize = len(g.currentFunc.Locals) * 4
	}
	
	if frameSize > 0 {
		g.emit("\tlink a6,#-%d", frameSize)
	} else {
		g.emit("\tlink a6,#0")
	}
	
	// Save registers we'll use (d2-d7, a2-a5 are callee-saved)
	g.emit("\tmovem.l d2-d7/a2-a5,-(sp)")
	g.stackOffset = 40 // 10 registers * 4 bytes
	
	// For SMC-enabled functions, set up parameter anchors  
	if g.currentFunc != nil && g.currentFunc.IsSMCEnabled {
		for i, param := range g.currentFunc.Params {
			if i == 0 {
				// First parameter in d0
				g.emit("%s$param_%s:", g.currentFunc.Name, param.Name)
				g.emit("\tmove.l #0,d0\t\t| SMC anchor for %s", param.Name)
			}
		}
	}
}

// generateEpilogue generates function epilogue
func (g *M68kGenerator) generateEpilogue() {
	// Restore registers
	g.emit("\tmovem.l (sp)+,d2-d7/a2-a5")
	
	// Restore frame pointer
	g.emit("\tunlk a6")
	g.emit("\trts")
}

// allocateRegisters assigns physical registers to virtual ones
func (g *M68kGenerator) allocateRegisters() {
	// Simple allocation: use data registers for most things
	regIndex := 0
	
	for _, inst := range g.currentFunc.Instructions {
		// Allocate destination register
		if inst.Dest != 0 && g.regAlloc[inst.Dest] == "" {
			if regIndex < len(g.dataRegs) {
				g.regAlloc[inst.Dest] = g.dataRegs[regIndex]
				regIndex++
			} else {
				// Spill to stack
				g.regAlloc[inst.Dest] = fmt.Sprintf("-%d(a6)", g.stackOffset)
				g.stackOffset += 4
			}
		}
	}
}

// generateInstruction generates code for a single instruction
func (g *M68kGenerator) generateInstruction(inst *ir.Instruction, index int) error {
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
	case ir.OpDiv:
		return g.generateDiv(inst)
	case ir.OpMod:
		return g.generateMod(inst)
	case ir.OpAnd:
		return g.generateAnd(inst)
	case ir.OpOr:
		return g.generateOr(inst)
	case ir.OpXor:
		return g.generateXor(inst)
	case ir.OpNot:
		return g.generateNot(inst)
	case ir.OpShl:
		return g.generateShl(inst)
	case ir.OpShr:
		return g.generateShr(inst)
	case ir.OpLt:
		return g.generateLT(inst)
	case ir.OpLe:
		return g.generateLE(inst)
	case ir.OpGt:
		return g.generateGT(inst)
	case ir.OpGe:
		return g.generateGE(inst)
	case ir.OpEq:
		return g.generateEQ(inst)
	case ir.OpNe:
		return g.generateNE(inst)
	case ir.OpCall:
		return g.generateCall(inst)
	case ir.OpReturn:
		return g.generateReturn(inst)
	case ir.OpJump:
		return g.generateJump(inst)
	case ir.OpJumpIf:
		return g.generateJumpIf(inst)
	case ir.OpJumpIfNot:
		return g.generateJumpIfNot(inst)
	case ir.OpLabel:
		g.emit("%s:", inst.Label)
		return nil
	case ir.OpPrint:
		return g.generatePrint(inst)
	case ir.OpPrintU8:
		return g.generatePrintU8(inst)
	case ir.OpPrintStringDirect:
		return g.generatePrintStringDirect(inst)
	case ir.OpAsm:
		return g.generateAsm(inst)
	default:
		// For now, emit a comment for unsupported operations
		g.emit("\t| TODO: %s", inst.Op)
		return nil
	}
}

// generateLoadConst generates code for loading a constant
func (g *M68kGenerator) generateLoadConst(inst *ir.Instruction) error {
	dest := g.getReg(inst.Dest)
	value := inst.Imm
	
	if strings.HasPrefix(dest, "d") {
		// Data register
		if value >= -128 && value <= 127 {
			g.emit("\tmoveq #%d,%s", value, dest)
		} else {
			g.emit("\tmove.l #%d,%s", value, dest)
		}
	} else {
		// Memory location
		g.emit("\tmove.l #%d,%s", value, dest)
	}
	
	return nil
}

// generateAdd generates addition
func (g *M68kGenerator) generateAdd(inst *ir.Instruction) error {
	src1 := g.getReg(inst.Src1)
	src2 := g.getReg(inst.Src2)
	dest := g.getReg(inst.Dest)
	
	// 68k can add directly to destination
	if dest == src1 {
		g.emit("\tadd.l %s,%s", src2, dest)
	} else if dest == src2 {
		g.emit("\tadd.l %s,%s", src1, dest)
	} else {
		// Need to move first
		g.emit("\tmove.l %s,%s", src1, dest)
		g.emit("\tadd.l %s,%s", src2, dest)
	}
	
	return nil
}

// generatePrintHelpers generates print routines
func (g *M68kGenerator) generatePrintHelpers() {
	g.emit("\n| Print helpers")
	
	// Print character (platform-specific)
	g.emit("print_char:")
	g.emit("\t| Character in d0")
	g.emit("\t| Platform-specific implementation needed")
	g.emit("\t| Amiga: dos.library/Write")
	g.emit("\t| Atari ST: GEMDOS Cconout")
	g.emit("\t| Mac: _PBWrite trap")
	g.emit("\trts")
	
	// Print hex byte
	g.emit("\nprint_hex:")
	g.emit("\tmove.b d0,d1")
	g.emit("\tlsr.b #4,d0")
	g.emit("\tbsr print_nibble")
	g.emit("\tmove.b d1,d0")
	g.emit("\tbsr print_nibble")
	g.emit("\trts")
	
	g.emit("\nprint_nibble:")
	g.emit("\tand.b #$0F,d0")
	g.emit("\tcmp.b #10,d0")
	g.emit("\tblt .digit")
	g.emit("\tadd.b #'A'-10,d0")
	g.emit("\tbra print_char")
	g.emit(".digit:")
	g.emit("\tadd.b #'0',d0")
	g.emit("\tbra print_char")
}

// Helper methods for other operations...

func (g *M68kGenerator) getReg(reg ir.Register) string {
	if reg == 0 {
		return "d0" // Use d0 for temporary/return values
	}
	if r, ok := g.regAlloc[reg]; ok {
		return r
	}
	// Fallback to d0
	return "d0"
}

func (g *M68kGenerator) generateLoadVar(inst *ir.Instruction) error {
	dest := g.getReg(inst.Dest)
	g.emit("\tmove.l %s,%s", inst.Symbol, dest)
	return nil
}

func (g *M68kGenerator) generateStoreVar(inst *ir.Instruction) error {
	src := g.getReg(inst.Src1)
	if inst.Symbol == "" {
		// Local variable - use stack offset
		g.emit("\tmove.l %s,-%d(a6)", src, g.stackOffset)
		g.stackOffset += 4
	} else {
		// Global variable
		g.emit("\tmove.l %s,%s", src, inst.Symbol)
	}
	return nil
}

func (g *M68kGenerator) generateSub(inst *ir.Instruction) error {
	src1 := g.getReg(inst.Src1)
	src2 := g.getReg(inst.Src2)
	dest := g.getReg(inst.Dest)
	
	if dest != src1 {
		g.emit("\tmove.l %s,%s", src1, dest)
	}
	g.emit("\tsub.l %s,%s", src2, dest)
	
	return nil
}

func (g *M68kGenerator) generateMul(inst *ir.Instruction) error {
	// 68000 multiply is signed 16x16->32
	src1 := g.getReg(inst.Src1)
	src2 := g.getReg(inst.Src2)
	dest := g.getReg(inst.Dest)
	
	g.emit("\tmove.w %s,d0", src1)
	g.emit("\tmuls.w %s,d0", src2)
	g.emit("\tmove.l d0,%s", dest)
	
	return nil
}

func (g *M68kGenerator) generateCall(inst *ir.Instruction) error {
	// Handle arguments - they should be in inst.Args
	if inst.Args != nil {
		for i, arg := range inst.Args {
			src := g.getReg(arg)
			if i < 4 {
				// First 4 args in d0-d3
				if src != fmt.Sprintf("d%d", i) {
					g.emit("\tmove.l %s,d%d", src, i)
				}
			} else if i < 8 {
				// Next 4 args in a0-a3
				g.emit("\tmove.l %s,a%d", src, i-4)
			} else {
				// Rest on stack
				g.emit("\tmove.l %s,-(sp)", src)
			}
		}
	}
	
	g.emit("\tjsr %s", inst.Symbol)
	
	// Clean up stack args if any
	if inst.Args != nil && len(inst.Args) > 8 {
		stackArgs := len(inst.Args) - 8
		g.emit("\tadd.l #%d,sp", stackArgs*4)
	}
	
	if inst.Dest != 0 {
		dest := g.getReg(inst.Dest)
		if dest != "d0" {
			g.emit("\tmove.l d0,%s", dest)
		}
	}
	return nil
}

func (g *M68kGenerator) generateReturn(inst *ir.Instruction) error {
	if inst.Src1 != 0 {
		src := g.getReg(inst.Src1)
		if src != "d0" {
			g.emit("\tmove.l %s,d0", src)
		}
	}
	g.generateEpilogue()
	return nil
}

func (g *M68kGenerator) generateJump(inst *ir.Instruction) error {
	g.emit("\tbra %s", inst.Label)
	return nil
}

func (g *M68kGenerator) generateJumpIfNot(inst *ir.Instruction) error {
	cond := g.getReg(inst.Src1)
	g.emit("\ttst.l %s", cond)
	g.emit("\tbeq %s", inst.Label)
	return nil
}

func (g *M68kGenerator) generatePrint(inst *ir.Instruction) error {
	g.emit("\tbsr print_char")
	return nil
}

func (g *M68kGenerator) generatePrintU8(inst *ir.Instruction) error {
	src := g.getReg(inst.Src1)
	if src != "d0" {
		g.emit("\tmove.b %s,d0", src)
	}
	g.emit("\tbsr print_hex")
	return nil
}

func (g *M68kGenerator) generatePrintStringDirect(inst *ir.Instruction) error {
	g.emit("\t| TODO: Print string direct")
	return nil
}

func (g *M68kGenerator) generateAsm(inst *ir.Instruction) error {
	// Pass through inline assembly
	lines := strings.Split(inst.Symbol, "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			g.emit("\t%s", line)
		}
	}
	return nil
}

// generateLoadParam loads a function parameter
func (g *M68kGenerator) generateLoadParam(inst *ir.Instruction) error {
	dest := g.getReg(inst.Dest)
	// Parameters are passed in registers d0-d3 and a0-a3, then on stack
	paramIndex := inst.Imm
	
	if paramIndex < 4 {
		// First 4 params in d0-d3
		g.emit("\tmove.l d%d,%s", paramIndex, dest)
	} else if paramIndex < 8 {
		// Next 4 params in a0-a3
		g.emit("\tmove.l a%d,%s", paramIndex-4, dest)
	} else {
		// Rest on stack (after return address and old a6)
		offset := 8 + (paramIndex-8)*4
		g.emit("\tmove.l %d(a6),%s", offset, dest)
	}
	return nil
}

// generateDiv generates division
func (g *M68kGenerator) generateDiv(inst *ir.Instruction) error {
	// 68000 division is 32/16->16q,16r
	src1 := g.getReg(inst.Src1)
	src2 := g.getReg(inst.Src2)
	dest := g.getReg(inst.Dest)
	
	g.emit("\tmove.l %s,d0", src1)
	g.emit("\tmove.w %s,d1", src2)
	g.emit("\text.l d0\t\t| Sign extend for signed division")
	g.emit("\tdivs.w d1,d0")
	g.emit("\tmove.w d0,%s", dest)
	
	return nil
}

// generateMod generates modulo
func (g *M68kGenerator) generateMod(inst *ir.Instruction) error {
	src1 := g.getReg(inst.Src1)
	src2 := g.getReg(inst.Src2)
	dest := g.getReg(inst.Dest)
	
	g.emit("\tmove.l %s,d0", src1)
	g.emit("\tmove.w %s,d1", src2)
	g.emit("\text.l d0")
	g.emit("\tdivs.w d1,d0")
	g.emit("\tswap d0\t\t| Remainder is in upper word")
	g.emit("\tmove.w d0,%s", dest)
	
	return nil
}

// Bitwise operations
func (g *M68kGenerator) generateAnd(inst *ir.Instruction) error {
	src1 := g.getReg(inst.Src1)
	src2 := g.getReg(inst.Src2)
	dest := g.getReg(inst.Dest)
	
	if dest != src1 {
		g.emit("\tmove.l %s,%s", src1, dest)
	}
	g.emit("\tand.l %s,%s", src2, dest)
	
	return nil
}

func (g *M68kGenerator) generateOr(inst *ir.Instruction) error {
	src1 := g.getReg(inst.Src1)
	src2 := g.getReg(inst.Src2)
	dest := g.getReg(inst.Dest)
	
	if dest != src1 {
		g.emit("\tmove.l %s,%s", src1, dest)
	}
	g.emit("\tor.l %s,%s", src2, dest)
	
	return nil
}

func (g *M68kGenerator) generateXor(inst *ir.Instruction) error {
	src1 := g.getReg(inst.Src1)
	src2 := g.getReg(inst.Src2)
	dest := g.getReg(inst.Dest)
	
	if dest != src1 {
		g.emit("\tmove.l %s,%s", src1, dest)
	}
	g.emit("\teor.l %s,%s", src2, dest)
	
	return nil
}

func (g *M68kGenerator) generateNot(inst *ir.Instruction) error {
	src := g.getReg(inst.Src1)
	dest := g.getReg(inst.Dest)
	
	if dest != src {
		g.emit("\tmove.l %s,%s", src, dest)
	}
	g.emit("\tnot.l %s", dest)
	
	return nil
}

func (g *M68kGenerator) generateShl(inst *ir.Instruction) error {
	src1 := g.getReg(inst.Src1)
	src2 := g.getReg(inst.Src2)
	dest := g.getReg(inst.Dest)
	
	g.emit("\tmove.l %s,%s", src1, dest)
	g.emit("\tmove.b %s,d0", src2)
	g.emit("\tlsl.l d0,%s", dest)
	
	return nil
}

func (g *M68kGenerator) generateShr(inst *ir.Instruction) error {
	src1 := g.getReg(inst.Src1)
	src2 := g.getReg(inst.Src2)
	dest := g.getReg(inst.Dest)
	
	g.emit("\tmove.l %s,%s", src1, dest)
	g.emit("\tmove.b %s,d0", src2)
	g.emit("\tlsr.l d0,%s", dest)
	
	return nil
}

// Comparison operations - set dest to 1 if true, 0 if false
func (g *M68kGenerator) generateLT(inst *ir.Instruction) error {
	return g.generateComparison(inst, "blt")
}

func (g *M68kGenerator) generateLE(inst *ir.Instruction) error {
	return g.generateComparison(inst, "ble")
}

func (g *M68kGenerator) generateGT(inst *ir.Instruction) error {
	return g.generateComparison(inst, "bgt")
}

func (g *M68kGenerator) generateGE(inst *ir.Instruction) error {
	return g.generateComparison(inst, "bge")
}

func (g *M68kGenerator) generateEQ(inst *ir.Instruction) error {
	return g.generateComparison(inst, "beq")
}

func (g *M68kGenerator) generateNE(inst *ir.Instruction) error {
	return g.generateComparison(inst, "bne")
}

// Helper for comparison operations
func (g *M68kGenerator) generateComparison(inst *ir.Instruction, branch string) error {
	src1 := g.getReg(inst.Src1)
	src2 := g.getReg(inst.Src2)
	dest := g.getReg(inst.Dest)
	
	label := g.newLabel()
	
	g.emit("\tcmp.l %s,%s", src2, src1)
	g.emit("\t%s .true_%s", branch, label)
	g.emit("\tmoveq #0,%s", dest)
	g.emit("\tbra .end_%s", label)
	g.emit(".true_%s:", label)
	g.emit("\tmoveq #1,%s", dest)
	g.emit(".end_%s:", label)
	
	return nil
}

func (g *M68kGenerator) generateJumpIf(inst *ir.Instruction) error {
	cond := g.getReg(inst.Src1)
	g.emit("\ttst.l %s", cond)
	g.emit("\tbne %s", inst.Label)
	return nil
}

// newLabel generates a unique label
func (g *M68kGenerator) newLabel() string {
	g.labelCounter++
	return fmt.Sprintf("L%d", g.labelCounter)
}

// emit writes a line to the output
func (g *M68kGenerator) emit(format string, args ...interface{}) {
	if len(args) > 0 {
		fmt.Fprintf(g.writer, format+"\n", args...)
	} else {
		fmt.Fprintf(g.writer, format+"\n")
	}
}