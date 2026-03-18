package codegen

import (
	"bytes"
	"fmt"
	"strings"
	"time"
	
	"github.com/minz/minzc/pkg/ir"
)

// M6502Backend implements the Backend interface for 6502 code generation
type M6502Backend struct {
	BaseBackend
}

// NewM6502Backend creates a new 6502 backend
func NewM6502Backend(options *BackendOptions) Backend {
	backend := &M6502Backend{
		BaseBackend: NewBaseBackend(options),
	}
	
	// Configure 6502-specific features
	backend.SetFeature(FeatureSelfModifyingCode, true)  // 6502 supports SMC
	backend.SetFeature(FeatureInterrupts, true)
	backend.SetFeature(Feature16BitPointers, true)
	backend.SetFeature(Feature24BitPointers, false)
	backend.SetFeature(Feature32BitPointers, false)
	backend.SetFeature(FeatureFloatingPoint, false)
	backend.SetFeature(FeatureFixedPoint, true)
	backend.SetFeature(FeatureZeroPage, true)  // 6502 has zero page
	
	return backend
}

// Name returns the name of this backend
func (b *M6502Backend) Name() string {
	return "6502"
}

// Generate generates 6502 assembly code for the given IR module
func (b *M6502Backend) Generate(module *ir.Module) (string, error) {
	// Use optimized generator if SMC is enabled
	if b.options != nil && (b.options.EnableSMC || b.options.OptimizationLevel > 0) {
		gen := NewM6502Generator(b, module)
		return gen.Generate()
	}
	
	// Fall back to basic generator
	var buf bytes.Buffer
	
	// Write header
	buf.WriteString("; MinZ 6502 generated code\n")
	buf.WriteString(fmt.Sprintf("; Generated: %s\n", time.Now().Format("2006-01-02 15:04:05")))
	buf.WriteString("\n")
	
	// 6502 specific setup
	origin := uint16(0x0800) // Default origin for 6502 programs
	if b.options != nil && b.options.TargetAddress != 0 {
		origin = b.options.TargetAddress
	}
	
	buf.WriteString(fmt.Sprintf("    * = $%04X\n", origin))
	buf.WriteString("\n")
	
	// Generate data section for globals
	if len(module.Globals) > 0 {
		buf.WriteString("; Global variables\n")
		for _, global := range module.Globals {
			buf.WriteString(fmt.Sprintf("%s: .byte 0", global.Name))
			if global.Type.Size() > 1 {
				buf.WriteString(fmt.Sprintf(" ; %d bytes\n", global.Type.Size()))
				for i := 1; i < global.Type.Size(); i++ {
					buf.WriteString("    .byte 0\n")
				}
			} else {
				buf.WriteString("\n")
			}
		}
		buf.WriteString("\n")
	}
	
	// Generate functions
	for _, fn := range module.Functions {
		if err := b.generateFunction(&buf, fn); err != nil {
			return "", err
		}
		buf.WriteString("\n")
	}
	
	// Generate helper routines
	b.generateHelpers(&buf)
	
	return buf.String(), nil
}

// generateFunction generates code for a single function
func (b *M6502Backend) generateFunction(buf *bytes.Buffer, fn *ir.Function) error {
	buf.WriteString(fmt.Sprintf("; Function: %s\n", fn.Name))
	buf.WriteString(fmt.Sprintf("%s:\n", fn.Name))
	
	// Basic MIR to 6502 code generation
	for _, inst := range fn.Instructions {
		if err := b.generateInstruction(buf, &inst, fn); err != nil {
			return err
		}
	}
	
	// Handle return
	if fn.Name == "main" {
		buf.WriteString("    brk        ; End program\n")
	} else {
		buf.WriteString("    rts        ; Return\n")
	}
	
	return nil
}

// generateInstruction generates 6502 code for a single MIR instruction
func (b *M6502Backend) generateInstruction(buf *bytes.Buffer, inst *ir.Instruction, fn *ir.Function) error {
	switch inst.Op {
	case ir.OpLoadConst:
		// Load immediate value into accumulator
		if inst.Imm <= 255 {
			buf.WriteString(fmt.Sprintf("    lda #$%02X      ; r%d = %d\n", inst.Imm, inst.Dest, inst.Imm))
		} else {
			// For 16-bit values, use X for high byte
			buf.WriteString(fmt.Sprintf("    lda #$%02X      ; r%d = %d (low)\n", inst.Imm&0xFF, inst.Dest, inst.Imm))
			buf.WriteString(fmt.Sprintf("    ldx #$%02X      ; r%d = %d (high)\n", (inst.Imm>>8)&0xFF, inst.Dest, inst.Imm))
		}
		
	case ir.OpMove:
		buf.WriteString(fmt.Sprintf("    ; move r%d = r%d (register allocation needed)\n", inst.Dest, inst.Src1))
		
	case ir.OpStoreVar:
		// Store accumulator to variable
		symbol := inst.Symbol
		if symbol == "" {
			// For local variables, use placeholder
			symbol = fmt.Sprintf("local_%d", inst.Src1)
		}
		buf.WriteString(fmt.Sprintf("    sta %s        ; store %s\n", symbol, symbol))
		
	case ir.OpLoadVar:
		// Load variable into accumulator
		buf.WriteString(fmt.Sprintf("    lda %s        ; r%d = %s\n", inst.Symbol, inst.Dest, inst.Symbol))
		
	case ir.OpAdd:
		buf.WriteString(fmt.Sprintf("    ; r%d = r%d + r%d (needs register allocation)\n", inst.Dest, inst.Src1, inst.Src2))
		buf.WriteString("    clc\n")
		buf.WriteString("    adc $00        ; placeholder\n")
		
	case ir.OpSub:
		buf.WriteString(fmt.Sprintf("    ; r%d = r%d - r%d (needs register allocation)\n", inst.Dest, inst.Src1, inst.Src2))
		buf.WriteString("    sec\n")
		buf.WriteString("    sbc $00        ; placeholder\n")
		
	case ir.OpCall:
		buf.WriteString(fmt.Sprintf("    jsr %s        ; call %s\n", inst.Symbol, inst.Symbol))
		
	case ir.OpReturn:
		// Return is handled by the function epilogue
		buf.WriteString("    ; return\n")
		
	case ir.OpLabel:
		buf.WriteString(fmt.Sprintf("%s:\n", inst.Label))
		
	case ir.OpJump:
		buf.WriteString(fmt.Sprintf("    jmp %s\n", inst.Label))
		
	case ir.OpJumpIfNot:
		buf.WriteString("    ; conditional jump (needs implementation)\n")
		buf.WriteString(fmt.Sprintf("    beq %s        ; if zero\n", inst.Label))
		
	case ir.OpPrint:
		// Print character in accumulator
		buf.WriteString("    jsr print_char ; print character\n")
		
	case ir.OpPrintU8:
		buf.WriteString("    ; TODO: print u8 as decimal\n")
		buf.WriteString("    jsr print_u8\n")
		
	case ir.OpPrintU16:
		buf.WriteString("    ; TODO: print u16 as decimal\n")
		buf.WriteString("    jsr print_u16\n")
		
	case ir.OpPrintString:
		buf.WriteString("    ; TODO: print string\n")
		
	case ir.OpPrintStringDirect:
		buf.WriteString("    ; TODO: print string direct\n")
		
	case ir.OpLoadString:
		// Load string address
		buf.WriteString(fmt.Sprintf("    lda #<%s      ; Load string address (low)\n", inst.Symbol))
		buf.WriteString(fmt.Sprintf("    ldx #>%s      ; Load string address (high)\n", inst.Symbol))
		
	case ir.OpAsm:
		// Inline assembly
		if inst.AsmCode != "" {
			// Split the assembly code by newlines and indent each line
			lines := strings.Split(inst.AsmCode, "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line != "" {
					buf.WriteString("    " + line + "\n")
				}
			}
		}
		
	default:
		buf.WriteString(fmt.Sprintf("    ; TODO: %s\n", inst.Op))
	}
	
	return nil
}

// generateHelpers generates helper routines
func (b *M6502Backend) generateHelpers(buf *bytes.Buffer) {
	buf.WriteString("; Helper routines\n")
	buf.WriteString("print_char:\n")
	buf.WriteString("    ; Platform-specific character output\n")
	buf.WriteString("    ; For C64: sta $FFD2\n")
	buf.WriteString("    ; For Apple II: jsr $FDED\n")
	buf.WriteString("    rts\n")
}

// GetFileExtension returns the file extension for 6502 assembly
func (b *M6502Backend) GetFileExtension() string {
	return ".s"
}

// SupportsFeature checks if the 6502 backend supports a specific feature
func (b *M6502Backend) SupportsFeature(feature string) bool {
	switch feature {
	case FeatureSelfModifyingCode:
		return true // 6502 can do SMC
	case FeatureInterrupts:
		return true
	case FeatureShadowRegisters:
		return false // No shadow registers
	case Feature16BitPointers:
		return true
	case Feature24BitPointers:
		return false
	case FeatureFloatingPoint:
		return false
	case FeatureFixedPoint:
		return true // Can implement in software
	default:
		return false
	}
}

// Register the 6502 backend
func init() {
	RegisterBackend("6502", func(options *BackendOptions) Backend {
		return NewM6502Backend(options)
	})
}