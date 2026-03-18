package codegen

import (
	"bytes"
	"fmt"
	"strings"
	
	"github.com/minz/minzc/pkg/ir"
)

// M6502Generator generates optimized 6502 code with zero-page SMC
type M6502Generator struct {
	backend     *M6502Backend
	module      *ir.Module
	currentFunc *ir.Function
	output      *bytes.Buffer
	optimizer   *M6502SMCOptimizer
	enhancement *M6502SMCEnhancement
	
	// Track current register contents
	accValue    ir.Register  // What's in accumulator
	xValue      ir.Register  // What's in X register  
	yValue      ir.Register  // What's in Y register
	
	labelCounter int
}

// NewM6502Generator creates a new optimized 6502 code generator
func NewM6502Generator(backend *M6502Backend, module *ir.Module) *M6502Generator {
	optimizer := NewM6502SMCOptimizer()
	gen := &M6502Generator{
		backend:     backend,
		module:      module,
		output:      &bytes.Buffer{},
		optimizer:   optimizer,
		enhancement: NewM6502SMCEnhancement(optimizer),
	}
	return gen
}

// Generate generates optimized 6502 assembly
func (g *M6502Generator) Generate() (string, error) {
	// Header
	g.emit("; MinZ 6502 generated code with zero-page optimization")
	g.emit("; SMC/TSMC optimizations enabled")
	g.emit("")
	
	// Origin
	origin := uint16(0x0800)
	if g.backend.options != nil && g.backend.options.TargetAddress != 0 {
		origin = g.backend.options.TargetAddress
	}
	g.emit("    * = $%04X", origin)
	g.emit("")
	
	// Optimize all functions first
	for _, fn := range g.module.Functions {
		if err := g.optimizer.OptimizeFunction(fn); err != nil {
			return "", fmt.Errorf("optimizing function %s: %w", fn.Name, err)
		}
	}
	
	// Output zero-page map
	g.output.WriteString(g.optimizer.GetZeroPageMap())
	g.emit("")
	
	// Generate globals
	if len(g.module.Globals) > 0 {
		g.emit("; Global variables")
		for _, global := range g.module.Globals {
			g.generateGlobal(&global)
		}
		g.emit("")
	}
	
	// Generate functions
	for _, fn := range g.module.Functions {
		if err := g.generateFunction(fn); err != nil {
			return "", fmt.Errorf("generating function %s: %w", fn.Name, err)
		}
		g.emit("")
	}
	
	// Generate helper routines
	g.generateHelpers()
	
	return g.output.String(), nil
}

func (g *M6502Generator) generateFunction(fn *ir.Function) error {
	g.currentFunc = fn
	
	g.emit("; Function: %s", fn.Name)
	if fn.IsSMCEnabled {
		g.emit("; SMC enabled - parameters in zero page")
		g.emit("; Enhanced optimizations applied")
	}
	g.emit("%s:", g.sanitizeName(fn.Name))
	
	// Generate SMC parameter anchors
	if fn.IsSMCEnabled {
		for _, param := range fn.Params {
			if addr, exists := g.optimizer.paramToZeroPage[param.Name]; exists {
				g.emit("%s_param_%s = $%02X  ; SMC parameter in zero page", 
					g.sanitizeName(fn.Name), param.Name, addr)
			}
		}
		g.emit("")
		
		// Apply enhanced SMC optimizations
		g.enhancement.EnhanceSMCFunction(fn, g)
	}
	
	// Generate instructions
	for _, inst := range fn.Instructions {
		if err := g.generateInstruction(&inst); err != nil {
			return err
		}
	}
	
	// Return
	if fn.Name != "main" && !strings.HasSuffix(fn.Name, ".main") {
		g.emit("    rts")
	} else {
		g.emit("    brk        ; End program")
	}
	
	return nil
}

func (g *M6502Generator) generateInstruction(inst *ir.Instruction) error {
	// Add comment for debugging
	if inst.Comment != "" {
		g.emit("    ; %s", inst.Comment)
	}
	
	switch inst.Op {
	case ir.OpLoadConst:
		return g.genLoadConst(inst)
		
	case ir.OpLoadVar:
		return g.genLoadVar(inst)
		
	case ir.OpStoreVar:
		return g.genStoreVar(inst)
		
	case ir.OpLoadParam:
		return g.genLoadParam(inst)
		
	case ir.OpAdd:
		return g.genAdd(inst)
		
	case ir.OpSub:
		return g.genSub(inst)
		
	case ir.OpInc:
		return g.genInc(inst)
		
	case ir.OpDec:
		return g.genDec(inst)
		
	case ir.OpCmp, ir.OpLt, ir.OpGt, ir.OpLe, ir.OpGe, ir.OpEq, ir.OpNe:
		return g.genCompare(inst)
		
	case ir.OpJump:
		g.emit("    jmp %s", inst.Label)
		
	case ir.OpJumpIf:
		return g.genJumpIf(inst)
		
	case ir.OpJumpIfNot:
		return g.genJumpIfNot(inst)
		
	case ir.OpLabel:
		g.emit("%s:", inst.Label)
		
	case ir.OpCall:
		return g.genCall(inst)
		
	case ir.OpReturn:
		if inst.Src1 != 0 {
			g.loadToA(inst.Src1)
		}
		g.emit("    ; return")
		
	case ir.OpPrint:
		g.loadToA(inst.Src1)
		g.emit("    jsr print_char")
		
	case ir.OpAsm:
		// Inline assembly
		if inst.AsmCode != "" {
			lines := strings.Split(inst.AsmCode, "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line != "" {
					g.emit("    %s", line)
				}
			}
		}
		
	default:
		g.emit("    ; TODO: %s", inst.Op)
	}
	
	return nil
}

func (g *M6502Generator) genLoadConst(inst *ir.Instruction) error {
	value := inst.Imm
	
	// Check if this goes to zero page
	if zpAddr, exists := g.optimizer.regToZeroPage[inst.Dest]; exists {
		if value <= 255 {
			g.emit("    lda #$%02X", value)
			g.emit(g.optimizer.GenerateZeroPageAccess("store8", zpAddr, fmt.Sprintf("r%d = %d", inst.Dest, value)))
		} else {
			// 16-bit constant
			g.emit("    lda #$%02X", value&0xFF)
			g.emit(g.optimizer.GenerateZeroPageAccess("store8", zpAddr, fmt.Sprintf("r%d low", inst.Dest)))
			g.emit("    lda #$%02X", (value>>8)&0xFF)
			g.emit(g.optimizer.GenerateZeroPageAccess("store8", zpAddr+1, fmt.Sprintf("r%d high", inst.Dest)))
		}
	} else {
		// Regular load
		if value <= 255 {
			g.emit("    lda #$%02X      ; r%d = %d", value, inst.Dest, value)
			g.accValue = inst.Dest
		} else {
			g.emit("    lda #$%02X      ; r%d = %d (low)", value&0xFF, inst.Dest, value)
			g.emit("    ldx #$%02X      ; r%d = %d (high)", (value>>8)&0xFF, inst.Dest, value)
			g.accValue = inst.Dest
			g.xValue = inst.Dest // High byte in X
		}
	}
	
	return nil
}

func (g *M6502Generator) genLoadVar(inst *ir.Instruction) error {
	// Check if dest is in zero page
	if zpAddr, exists := g.optimizer.regToZeroPage[inst.Dest]; exists {
		g.emit("    lda %s", inst.Symbol)
		g.emit(g.optimizer.GenerateZeroPageAccess("store8", zpAddr, fmt.Sprintf("r%d = %s", inst.Dest, inst.Symbol)))
		g.accValue = inst.Dest
	} else {
		g.emit("    lda %s        ; r%d = %s", inst.Symbol, inst.Dest, inst.Symbol)
		g.accValue = inst.Dest
	}
	
	return nil
}

func (g *M6502Generator) genStoreVar(inst *ir.Instruction) error {
	// Load source to accumulator if needed
	g.loadToA(inst.Src1)
	g.emit("    sta %s        ; %s = r%d", inst.Symbol, inst.Symbol, inst.Src1)
	return nil
}

func (g *M6502Generator) genLoadParam(inst *ir.Instruction) error {
	// Check if this is an SMC parameter in zero page
	if g.currentFunc.IsSMCEnabled {
		if zpAddr, exists := g.optimizer.paramToZeroPage[inst.Symbol]; exists {
			// Load from zero page SMC slot
			g.emit(g.optimizer.GenerateZeroPageAccess("load8", zpAddr, 
				fmt.Sprintf("r%d = param %s (SMC)", inst.Dest, inst.Symbol)))
			g.accValue = inst.Dest
			
			// Also store to destination register if in zero page
			if destAddr, exists := g.optimizer.regToZeroPage[inst.Dest]; exists && destAddr != zpAddr {
				g.emit(g.optimizer.GenerateZeroPageAccess("store8", destAddr, 
					fmt.Sprintf("r%d", inst.Dest)))
			}
			return nil
		}
	}
	
	// Regular parameter load
	return g.genLoadVar(inst)
}

func (g *M6502Generator) genAdd(inst *ir.Instruction) error {
	// Load first operand
	g.loadToA(inst.Src1)
	g.emit("    clc")
	
	// Add second operand
	if zpAddr, exists := g.optimizer.regToZeroPage[inst.Src2]; exists {
		g.emit("    adc $%02X        ; + r%d (from zero page)", zpAddr, inst.Src2)
	} else {
		g.emit("    adc temp_%d     ; + r%d", inst.Src2, inst.Src2)
	}
	
	// Store result
	if zpAddr, exists := g.optimizer.regToZeroPage[inst.Dest]; exists {
		g.emit(g.optimizer.GenerateZeroPageAccess("store8", zpAddr, fmt.Sprintf("r%d = result", inst.Dest)))
	}
	
	g.accValue = inst.Dest
	return nil
}

func (g *M6502Generator) genSub(inst *ir.Instruction) error {
	// Load first operand
	g.loadToA(inst.Src1)
	g.emit("    sec")
	
	// Subtract second operand
	if zpAddr, exists := g.optimizer.regToZeroPage[inst.Src2]; exists {
		g.emit("    sbc $%02X        ; - r%d (from zero page)", zpAddr, inst.Src2)
	} else {
		g.emit("    sbc temp_%d     ; - r%d", inst.Src2, inst.Src2)
	}
	
	// Store result
	if zpAddr, exists := g.optimizer.regToZeroPage[inst.Dest]; exists {
		g.emit(g.optimizer.GenerateZeroPageAccess("store8", zpAddr, fmt.Sprintf("r%d = result", inst.Dest)))
	}
	
	g.accValue = inst.Dest
	return nil
}

func (g *M6502Generator) genInc(inst *ir.Instruction) error {
	// Check if register is in zero page
	if zpAddr, exists := g.optimizer.regToZeroPage[inst.Src1]; exists {
		// Direct increment in zero page!
		g.emit(g.optimizer.GenerateZeroPageAccess("inc", zpAddr, fmt.Sprintf("r%d++", inst.Src1)))
		if inst.Dest != inst.Src1 {
			// Copy to dest if different
			g.emit(g.optimizer.GenerateZeroPageAccess("load8", zpAddr, fmt.Sprintf("r%d", inst.Src1)))
			g.accValue = inst.Src1
			if destAddr, exists := g.optimizer.regToZeroPage[inst.Dest]; exists {
				g.emit(g.optimizer.GenerateZeroPageAccess("store8", destAddr, fmt.Sprintf("r%d = r%d", inst.Dest, inst.Src1)))
			}
		}
	} else {
		// Regular increment
		g.loadToA(inst.Src1)
		g.emit("    clc")
		g.emit("    adc #1")
		g.accValue = inst.Dest
	}
	
	return nil
}

func (g *M6502Generator) genDec(inst *ir.Instruction) error {
	// Check if register is in zero page
	if zpAddr, exists := g.optimizer.regToZeroPage[inst.Src1]; exists {
		// Direct decrement in zero page!
		g.emit(g.optimizer.GenerateZeroPageAccess("dec", zpAddr, fmt.Sprintf("r%d--", inst.Src1)))
		if inst.Dest != inst.Src1 {
			// Copy to dest if different
			g.emit(g.optimizer.GenerateZeroPageAccess("load8", zpAddr, fmt.Sprintf("r%d", inst.Src1)))
			g.accValue = inst.Src1
			if destAddr, exists := g.optimizer.regToZeroPage[inst.Dest]; exists {
				g.emit(g.optimizer.GenerateZeroPageAccess("store8", destAddr, fmt.Sprintf("r%d = r%d", inst.Dest, inst.Src1)))
			}
		}
	} else {
		// Regular decrement
		g.loadToA(inst.Src1)
		g.emit("    sec")
		g.emit("    sbc #1")
		g.accValue = inst.Dest
	}
	
	return nil
}

func (g *M6502Generator) genCompare(inst *ir.Instruction) error {
	// Load first operand
	g.loadToA(inst.Src1)
	
	// Compare with second operand
	if zpAddr, exists := g.optimizer.regToZeroPage[inst.Src2]; exists {
		g.emit("    cmp $%02X        ; compare with r%d (zero page)", zpAddr, inst.Src2)
	} else {
		g.emit("    cmp temp_%d     ; compare with r%d", inst.Src2, inst.Src2)
	}
	
	// Set result based on comparison type
	switch inst.Op {
	case ir.OpLt:
		g.emit("    lda #0         ; assume false")
		g.emit("    bcs +3         ; skip if >=")
		g.emit("    lda #1         ; true if <")
	case ir.OpGt:
		g.emit("    lda #0         ; assume false")
		g.emit("    beq +5         ; skip if =")
		g.emit("    bcc +3         ; skip if <")
		g.emit("    lda #1         ; true if >")
	case ir.OpEq:
		g.emit("    lda #0         ; assume false")
		g.emit("    bne +3         ; skip if !=")
		g.emit("    lda #1         ; true if =")
	}
	
	// Store result
	if zpAddr, exists := g.optimizer.regToZeroPage[inst.Dest]; exists {
		g.emit(g.optimizer.GenerateZeroPageAccess("store8", zpAddr, fmt.Sprintf("r%d = comparison result", inst.Dest)))
	}
	
	g.accValue = inst.Dest
	return nil
}

func (g *M6502Generator) genJumpIf(inst *ir.Instruction) error {
	// Load condition
	g.loadToA(inst.Src1)
	g.emit("    bne %s         ; jump if true", inst.Label)
	return nil
}

func (g *M6502Generator) genJumpIfNot(inst *ir.Instruction) error {
	// Load condition
	g.loadToA(inst.Src1)
	g.emit("    beq %s         ; jump if false", inst.Label)
	return nil
}

func (g *M6502Generator) genCall(inst *ir.Instruction) error {
	// For SMC functions, patch parameters before call
	targetFunc := g.findFunction(inst.Symbol)
	if targetFunc != nil && targetFunc.IsSMCEnabled {
		// Patch each argument into zero page
		for i, argReg := range inst.Args {
			if i < len(targetFunc.Params) {
				paramName := targetFunc.Params[i].Name
				if zpAddr, exists := g.optimizer.paramToZeroPage[paramName]; exists {
					g.loadToA(argReg)
					g.emit("    sta $%02X        ; Patch SMC param %s", zpAddr, paramName)
				}
			}
		}
	}
	
	g.emit("    jsr %s", g.sanitizeName(inst.Symbol))
	
	// Result in accumulator
	if inst.Dest != 0 {
		g.accValue = inst.Dest
		// Store to zero page if allocated
		if zpAddr, exists := g.optimizer.regToZeroPage[inst.Dest]; exists {
			g.emit(g.optimizer.GenerateZeroPageAccess("store8", zpAddr, fmt.Sprintf("r%d = result", inst.Dest)))
		}
	}
	
	return nil
}

func (g *M6502Generator) loadToA(reg ir.Register) {
	if g.accValue == reg {
		return // Already in accumulator
	}
	
	// Check if in zero page
	if zpAddr, exists := g.optimizer.regToZeroPage[reg]; exists {
		g.emit(g.optimizer.GenerateZeroPageAccess("load8", zpAddr, fmt.Sprintf("load r%d", reg)))
	} else {
		g.emit("    lda temp_%d     ; load r%d", reg, reg)
	}
	
	g.accValue = reg
}

func (g *M6502Generator) generateGlobal(global *ir.Global) {
	g.emit("%s:", global.Name)
	if global.Type.Size() == 1 {
		if global.Init != nil {
			g.emit("    .byte %s", g.formatInit(global.Init))
		} else {
			g.emit("    .byte 0")
		}
	} else {
		// Multi-byte global
		g.emit("    .res %d         ; %d bytes", global.Type.Size(), global.Type.Size())
	}
}

func (g *M6502Generator) generateHelpers() {
	g.emit("; Helper routines")
	g.emit("print_char:")
	g.emit("    ; Platform-specific character output")
	g.emit("    ; For C64:")
	g.emit("    jsr $FFD2      ; CHROUT")
	g.emit("    rts")
	g.emit("")
	g.emit("    ; For Apple II:")
	g.emit("    ; jsr $FDED    ; COUT")
	g.emit("    ; rts")
}

func (g *M6502Generator) findFunction(name string) *ir.Function {
	for _, fn := range g.module.Functions {
		if fn.Name == name {
			return fn
		}
	}
	return nil
}

func (g *M6502Generator) sanitizeName(name string) string {
	return strings.ReplaceAll(name, ".", "_")
}

func (g *M6502Generator) formatInit(value interface{}) string {
	switch v := value.(type) {
	case int:
		return fmt.Sprintf("$%02X", v)
	case int64:
		return fmt.Sprintf("$%02X", v)
	default:
		return "0"
	}
}

func (g *M6502Generator) emit(format string, args ...interface{}) {
	fmt.Fprintf(g.output, format, args...)
	fmt.Fprintln(g.output)
}

func (g *M6502Generator) getLabel() string {
	g.labelCounter++
	return fmt.Sprintf(".L%d", g.labelCounter)
}