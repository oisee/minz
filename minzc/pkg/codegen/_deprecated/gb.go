package codegen

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/minz/minzc/pkg/ir"
)

// GBGenerator generates Game Boy (LR35902) assembly from IR
type GBGenerator struct {
	writer        io.Writer
	module        *ir.Module
	currentFunc   *ir.Function
	
	// Register allocation - GB has no shadow registers or IX/IY
	regAlloc      *RegisterAllocator
	
	stackOffset   int
	labelCounter  int
	localVarBase  uint16 // Base address for local variables
	emittedParams map[string]bool // Track which SMC parameters have been emitted
}

// NewGBGenerator creates a new Game Boy code generator
func NewGBGenerator(w io.Writer) *GBGenerator {
	return &GBGenerator{
		writer:          w,
		regAlloc:        NewRegisterAllocator(),
		localVarBase:    0xC000, // WRAM starts at 0xC000 on GB
		emittedParams:   make(map[string]bool),
	}
}

// Generate generates Game Boy assembly for an IR module
func (g *GBGenerator) Generate(module *ir.Module) error {
	g.module = module

	// Write header
	g.writeHeader()

	// Generate data section in WRAM
	if len(module.Globals) > 0 || len(module.Strings) > 0 {
		g.emit("\n; Data section (WRAM)")
		g.emit("SECTION \"Variables\", WRAM0[$C000]")
		g.emit("")
		for _, global := range module.Globals {
			g.generateGlobal(global)
		}
		
		// String literals go in ROM
		if len(module.Strings) > 0 {
			g.emit("\nSECTION \"StringData\", ROM0")
			for _, str := range module.Strings {
				g.generateString(*str)
			}
		}
	}

	// Generate code section
	g.emit("\n; Code section")
	g.emit("SECTION \"Code\", ROM0[$0150]") // After GB header
	g.emit("")

	// Generate functions
	for _, fn := range module.Functions {
		if err := g.generateFunction(fn); err != nil {
			return err
		}
	}

	// Generate runtime helper functions
	g.generatePrintHelpers()
	
	return nil
}

// writeHeader writes the assembly file header
func (g *GBGenerator) writeHeader() {
	g.emit("; MinZ Game Boy generated code")
	g.emit("; Generated: %s", time.Now().Format("2006-01-02 15:04:05"))
	g.emit("; Target: Sharp LR35902 (Game Boy CPU)")
	g.emit("; Note: No shadow registers or IX/IY on GB")
	g.emit("")
	g.emit("; Using RGBDS assembler syntax")
	g.emit("")
}

// generateGlobal generates a global variable
func (g *GBGenerator) generateGlobal(global ir.Global) {
	g.emit("%s:", global.Name)
	size := global.Type.Size()
	// For now, just reserve space
	// TODO: Handle initialization values
	g.emit("    DS %d ; %s", size, global.Type.String())
}

// generateString generates a string literal
func (g *GBGenerator) generateString(str ir.String) {
	g.emit("%s:", str.Label)
	// Length-prefixed string
	g.emit("    DB %d ; Length", len(str.Value))
	if len(str.Value) > 0 {
		// Escape special characters
		escaped := strings.ReplaceAll(str.Value, "\"", "\\\"")
		g.emit("    DB \"%s\"", escaped)
	}
}

// generateFunction generates a function
func (g *GBGenerator) generateFunction(fn *ir.Function) error {
	g.currentFunc = fn
	g.stackOffset = 0
	g.regAlloc.Reset()

	// Function label
	g.emit("\n; Function: %s", fn.Name)
	if fn.IsSMCEnabled {
		g.emit("; SMC enabled")
	}
	g.emit("%s:", fn.Name)

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
func (g *GBGenerator) generatePrologue() {
	// Save registers if needed
	if g.currentFunc.UsedRegisters != 0 {
		g.emit("    PUSH BC")
		g.emit("    PUSH DE")
		g.emit("    PUSH HL")
		g.stackOffset += 6
	}
}

// generateEpilogue generates function epilogue
func (g *GBGenerator) generateEpilogue() {
	// Restore registers if needed
	if g.currentFunc.UsedRegisters != 0 {
		g.emit("    POP HL")
		g.emit("    POP DE")
		g.emit("    POP BC")
	}
	g.emit("    RET")
}

// generateInstruction generates code for a single instruction
func (g *GBGenerator) generateInstruction(inst *ir.Instruction, index int) error {
	switch inst.Op {
	case ir.OpLoadConst:
		return g.generateLoadConst(inst)
	case ir.OpLoadVar:
		return g.generateLoadVar(inst)
	case ir.OpStoreVar:
		return g.generateStoreVar(inst)
	case ir.OpAdd:
		return g.generateAdd(inst)
	case ir.OpSub:
		return g.generateSub(inst)
	case ir.OpCall:
		return g.generateCall(inst)
	case ir.OpReturn:
		return g.generateReturn(inst)
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
		g.emit("    ; TODO: %s", inst.Op)
		return nil
	}
}

// generateLoadConst generates code for loading a constant
func (g *GBGenerator) generateLoadConst(inst *ir.Instruction) error {
	// For now, use simple register allocation
	value := inst.Imm
	
	if value <= 255 {
		g.emit("    LD A, %d", value)
		g.emit("    ; Store to r%d", inst.Dest)
	} else {
		g.emit("    LD HL, %d", value)
		g.emit("    ; Store to r%d", inst.Dest)
	}
	
	return nil
}

// generateAdd generates addition
func (g *GBGenerator) generateAdd(inst *ir.Instruction) error {
	// Simplified for now
	g.emit("    ; ADD r%d + r%d -> r%d", inst.Src1, inst.Src2, inst.Dest)
	g.emit("    ; TODO: Implement register allocation")
	
	return nil
}

// generatePrintHelpers generates GB-specific print routines
func (g *GBGenerator) generatePrintHelpers() {
	g.emit("\n; Print helpers for Game Boy")
	
	// Print character routine using GB's tile system
	g.emit("print_char:")
	g.emit("    ; Wait for VBlank")
	g.emit("    LD HL, $FF44  ; LY register")
	g.emit(".wait_vblank:")
	g.emit("    LD A, [HL]")
	g.emit("    CP 144")
	g.emit("    JR C, .wait_vblank")
	g.emit("    ; Character in A, write to tile map")
	g.emit("    ; This is a simplified version")
	g.emit("    RET")
	
	// Print hex byte
	g.emit("\nprint_hex:")
	g.emit("    PUSH AF")
	g.emit("    SWAP A")
	g.emit("    CALL print_nibble")
	g.emit("    POP AF")
	g.emit("    CALL print_nibble")
	g.emit("    RET")
	
	g.emit("\nprint_nibble:")
	g.emit("    AND $0F")
	g.emit("    CP 10")
	g.emit("    JR C, .digit")
	g.emit("    ADD A, 'A' - 10")
	g.emit("    JR print_char")
	g.emit(".digit:")
	g.emit("    ADD A, '0'")
	g.emit("    JR print_char")
}

// Helper methods for other operations...

func (g *GBGenerator) generateLoadVar(inst *ir.Instruction) error {
	// Simplified for now
	g.emit("    ; Load var %s to r%d", inst.Symbol, inst.Dest)
	return nil
}

func (g *GBGenerator) generateStoreVar(inst *ir.Instruction) error {
	// Simplified for now
	g.emit("    ; Store r%d to var %s", inst.Src1, inst.Symbol)
	return nil
}

func (g *GBGenerator) generateSub(inst *ir.Instruction) error {
	// Similar to add but with SUB
	g.emit("    ; SUB r%d - r%d -> r%d", inst.Src1, inst.Src2, inst.Dest)
	return nil
}

func (g *GBGenerator) generateCall(inst *ir.Instruction) error {
	g.emit("    CALL %s", inst.Symbol)
	return nil
}

func (g *GBGenerator) generateReturn(inst *ir.Instruction) error {
	g.generateEpilogue()
	return nil
}

func (g *GBGenerator) generatePrint(inst *ir.Instruction) error {
	g.emit("    ; Print character")
	g.emit("    CALL print_char")
	return nil
}

func (g *GBGenerator) generatePrintU8(inst *ir.Instruction) error {
	g.emit("    ; Print u8 as hex")
	g.emit("    CALL print_hex")
	return nil
}

func (g *GBGenerator) generatePrintStringDirect(inst *ir.Instruction) error {
	g.emit("    ; Print string direct")
	return nil
}

func (g *GBGenerator) generateAsm(inst *ir.Instruction) error {
	// Pass through inline assembly, converting Z80 syntax to GB where needed
	asmCode := inst.Symbol
	
	// Replace Z80-specific instructions with GB equivalents
	asmCode = strings.ReplaceAll(asmCode, "EXX", "; EXX not available on GB")
	asmCode = strings.ReplaceAll(asmCode, "EX AF,AF'", "; No shadow registers on GB")
	
	// Emit the assembly
	lines := strings.Split(asmCode, "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			g.emit("    %s", line)
		}
	}
	
	return nil
}

// emit writes a line to the output
func (g *GBGenerator) emit(format string, args ...interface{}) {
	if len(args) > 0 {
		fmt.Fprintf(g.writer, format+"\n", args...)
	} else {
		fmt.Fprintf(g.writer, format+"\n")
	}
}