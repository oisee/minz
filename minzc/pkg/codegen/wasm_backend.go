package codegen

import (
	"bytes"
	"fmt"
	"github.com/minz/minzc/pkg/ir"
	"time"
)

// WASMBackend implements the Backend interface for WebAssembly code generation
type WASMBackend struct {
	options *BackendOptions
}

// NewWASMBackend creates a new WASM backend
func NewWASMBackend(options *BackendOptions) Backend {
	return &WASMBackend{
		options: options,
	}
}

// Name returns the name of this backend
func (b *WASMBackend) Name() string {
	return "wasm"
}

// Generate generates WebAssembly text format (WAT) code for the given IR module
func (b *WASMBackend) Generate(module *ir.Module) (string, error) {
	// WASM doesn't support SMC - use standard calling conventions
	for _, fn := range module.Functions {
		fn.IsSMCEnabled = false
	}
	
	var buf bytes.Buffer
	
	// Write header
	buf.WriteString(";; MinZ WebAssembly generated code\n")
	buf.WriteString(fmt.Sprintf(";; Generated: %s\n", time.Now().Format("2006-01-02 15:04:05")))
	buf.WriteString(";; Note: WASM uses stack-based calling convention, no SMC\n")
	buf.WriteString("\n")
	
	// Start module
	buf.WriteString("(module\n")
	
	// Import memory and print function
	buf.WriteString("  ;; Import memory\n")
	buf.WriteString("  (import \"env\" \"memory\" (memory 1))\n")
	buf.WriteString("  (import \"env\" \"print_char\" (func $print_char (param i32)))\n")
	buf.WriteString("  (import \"env\" \"print_i32\" (func $print_i32 (param i32)))\n")
	buf.WriteString("\n")
	
	// Declare globals
	if len(module.Globals) > 0 {
		buf.WriteString("  ;; Global variables\n")
		globalOffset := 0
		for _, global := range module.Globals {
			typeStr := b.wasmType(global.Type)
			buf.WriteString(fmt.Sprintf("  (global $%s (mut %s) (%s.const 0))\n", 
				global.Name, typeStr, typeStr))
			globalOffset += global.Type.Size()
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
	
	// Export main if it exists
	buf.WriteString("  ;; Export main function\n")
	buf.WriteString("  (export \"main\" (func $main))\n")
	
	// Close module
	buf.WriteString(")\n")
	
	return buf.String(), nil
}

// wasmType converts MinZ types to WASM types
func (b *WASMBackend) wasmType(t ir.Type) string {
	switch t.Size() {
	case 1:
		return "i32" // WASM doesn't have i8, use i32
	case 2:
		return "i32" // WASM doesn't have i16, use i32
	case 3, 4:
		return "i32"
	case 8:
		return "i64"
	default:
		return "i32"
	}
}

// generateFunction generates WASM code for a single function
func (b *WASMBackend) generateFunction(buf *bytes.Buffer, fn *ir.Function) error {
	buf.WriteString(fmt.Sprintf("  ;; Function: %s\n", fn.Name))
	buf.WriteString(fmt.Sprintf("  (func $%s", fn.Name))
	
	// Parameters
	if len(fn.Params) > 0 {
		buf.WriteString(" ")
		for _, param := range fn.Params {
			buf.WriteString(fmt.Sprintf("(param $%s %s) ", param.Name, b.wasmType(param.Type)))
		}
	}
	
	// Return type
	if fn.ReturnType != nil && fn.ReturnType.Size() > 0 {
		buf.WriteString(fmt.Sprintf(" (result %s)", b.wasmType(fn.ReturnType)))
	}
	
	buf.WriteString("\n")
	
	// Locals - including MIR virtual registers
	if len(fn.Locals) > 0 {
		for _, local := range fn.Locals {
			buf.WriteString(fmt.Sprintf("    (local $%s %s)\n", local.Name, b.wasmType(local.Type)))
		}
	}
	
	// Add locals for virtual registers used in instructions
	maxReg := 0
	for _, inst := range fn.Instructions {
		if int(inst.Dest) > maxReg {
			maxReg = int(inst.Dest)
		}
		if int(inst.Src1) > maxReg {
			maxReg = int(inst.Src1)
		}
		if int(inst.Src2) > maxReg {
			maxReg = int(inst.Src2)
		}
	}
	
	// Create locals for virtual registers r1 through rN
	for i := 1; i <= maxReg; i++ {
		buf.WriteString(fmt.Sprintf("    (local $r%d i32)\n", i))
	}
	
	// Generate instructions
	for _, inst := range fn.Instructions {
		if err := b.generateInstruction(buf, &inst, fn); err != nil {
			return err
		}
	}
	
	// Default return if needed
	if fn.ReturnType != nil && fn.ReturnType.Size() > 0 {
		// Only add default return if last instruction wasn't a return
		if len(fn.Instructions) == 0 || fn.Instructions[len(fn.Instructions)-1].Op != ir.OpReturn {
			buf.WriteString("    i32.const 0\n")
		}
	}
	
	buf.WriteString("  )\n")
	
	return nil
}

// GetFileExtension returns the file extension for WebAssembly text format
func (b *WASMBackend) GetFileExtension() string {
	return ".wat"
}

// SupportsFeature checks if the WASM backend supports a specific feature
func (b *WASMBackend) SupportsFeature(feature string) bool {
	switch feature {
	case FeatureSelfModifyingCode:
		return false // WASM doesn't support SMC
	case FeatureInterrupts:
		return false // No direct interrupt support
	case FeatureShadowRegisters:
		return false
	case Feature16BitPointers:
		return false // WASM uses 32-bit addresses
	case Feature24BitPointers:
		return false
	case Feature32BitPointers:
		return true
	case FeatureFloatingPoint:
		return true // WASM has f32 and f64
	case FeatureFixedPoint:
		return true // Can implement in software
	default:
		return false
	}
}

// generateInstruction generates WASM code for a single MIR instruction
func (b *WASMBackend) generateInstruction(buf *bytes.Buffer, inst *ir.Instruction, fn *ir.Function) error {
	switch inst.Op {
	case ir.OpLoadConst:
		buf.WriteString(fmt.Sprintf("    i32.const %d  ;; r%d = %d\n", inst.Imm, inst.Dest, inst.Imm))
		buf.WriteString(fmt.Sprintf("    local.set $r%d\n", inst.Dest))
		
	case ir.OpMove:
		buf.WriteString(fmt.Sprintf("    local.get $r%d  ;; r%d = r%d\n", inst.Src1, inst.Dest, inst.Src1))
		buf.WriteString(fmt.Sprintf("    local.set $r%d\n", inst.Dest))
		
	case ir.OpStoreVar:
		buf.WriteString(fmt.Sprintf("    local.get $r%d  ;; store %s\n", inst.Src1, inst.Symbol))
		buf.WriteString(fmt.Sprintf("    global.set $%s\n", inst.Symbol))
		
	case ir.OpLoadVar:
		buf.WriteString(fmt.Sprintf("    global.get $%s  ;; r%d = %s\n", inst.Symbol, inst.Dest, inst.Symbol))
		buf.WriteString(fmt.Sprintf("    local.set $r%d\n", inst.Dest))
		
	case ir.OpAdd:
		buf.WriteString(fmt.Sprintf("    local.get $r%d  ;; r%d = r%d + r%d\n", inst.Src1, inst.Dest, inst.Src1, inst.Src2))
		buf.WriteString(fmt.Sprintf("    local.get $r%d\n", inst.Src2))
		buf.WriteString("    i32.add\n")
		buf.WriteString(fmt.Sprintf("    local.set $r%d\n", inst.Dest))
		
	case ir.OpSub:
		buf.WriteString(fmt.Sprintf("    local.get $r%d  ;; r%d = r%d - r%d\n", inst.Src1, inst.Dest, inst.Src1, inst.Src2))
		buf.WriteString(fmt.Sprintf("    local.get $r%d\n", inst.Src2))
		buf.WriteString("    i32.sub\n")
		buf.WriteString(fmt.Sprintf("    local.set $r%d\n", inst.Dest))
		
	case ir.OpMul:
		buf.WriteString(fmt.Sprintf("    local.get $r%d  ;; r%d = r%d * r%d\n", inst.Src1, inst.Dest, inst.Src1, inst.Src2))
		buf.WriteString(fmt.Sprintf("    local.get $r%d\n", inst.Src2))
		buf.WriteString("    i32.mul\n")
		buf.WriteString(fmt.Sprintf("    local.set $r%d\n", inst.Dest))
		
	case ir.OpCall:
		// Push arguments (would need to handle this properly)
		buf.WriteString(fmt.Sprintf("    call $%s  ;; call %s\n", inst.Symbol, inst.Symbol))
		if inst.Dest != 0 {
			buf.WriteString(fmt.Sprintf("    local.set $r%d\n", inst.Dest))
		}
		
	case ir.OpReturn:
		if inst.Src1 != 0 {
			buf.WriteString(fmt.Sprintf("    local.get $r%d  ;; return\n", inst.Src1))
		}
		buf.WriteString("    return\n")
		
	case ir.OpLabel:
		// WASM doesn't have labels in the same way, would need block/loop structure
		buf.WriteString(fmt.Sprintf("    ;; Label: %s\n", inst.Label))
		
	case ir.OpJump:
		buf.WriteString(fmt.Sprintf("    ;; TODO: jump %s (needs block structure)\n", inst.Label))
		
	case ir.OpJumpIfNot:
		buf.WriteString(fmt.Sprintf("    ;; TODO: jump_if_not r%d, %s (needs block structure)\n", inst.Src1, inst.Label))
		
	case ir.OpEq:
		buf.WriteString(fmt.Sprintf("    local.get $r%d  ;; r%d = r%d == r%d\n", inst.Src1, inst.Dest, inst.Src1, inst.Src2))
		buf.WriteString(fmt.Sprintf("    local.get $r%d\n", inst.Src2))
		buf.WriteString("    i32.eq\n")
		buf.WriteString(fmt.Sprintf("    local.set $r%d\n", inst.Dest))
		
	case ir.OpNe:
		buf.WriteString(fmt.Sprintf("    local.get $r%d  ;; r%d = r%d != r%d\n", inst.Src1, inst.Dest, inst.Src1, inst.Src2))
		buf.WriteString(fmt.Sprintf("    local.get $r%d\n", inst.Src2))
		buf.WriteString("    i32.ne\n")
		buf.WriteString(fmt.Sprintf("    local.set $r%d\n", inst.Dest))
		
	case ir.OpLt:
		buf.WriteString(fmt.Sprintf("    local.get $r%d  ;; r%d = r%d < r%d\n", inst.Src1, inst.Dest, inst.Src1, inst.Src2))
		buf.WriteString(fmt.Sprintf("    local.get $r%d\n", inst.Src2))
		buf.WriteString("    i32.lt_s\n")
		buf.WriteString(fmt.Sprintf("    local.set $r%d\n", inst.Dest))
		
	case ir.OpPrint:
		// Print character
		buf.WriteString(fmt.Sprintf("    local.get $r%d  ;; print character\n", inst.Src1))
		buf.WriteString("    call $print_char\n")
		
	case ir.OpPrintU8:
		buf.WriteString(fmt.Sprintf("    local.get $r%d  ;; print u8\n", inst.Src1))
		buf.WriteString("    call $print_i32\n")
		
	case ir.OpPrintU16:
		buf.WriteString(fmt.Sprintf("    local.get $r%d  ;; print u16\n", inst.Src1))
		buf.WriteString("    call $print_i32\n")
		
	case ir.OpPrintString:
		buf.WriteString("    ;; TODO: print string (needs memory access)\n")
		
	case ir.OpPrintStringDirect:
		buf.WriteString("    ;; TODO: print string direct\n")
		
	case ir.OpLoadString:
		// Load string address (offset in memory)
		buf.WriteString(fmt.Sprintf("    i32.const 0  ;; TODO: string offset for %s\n", inst.Symbol))
		buf.WriteString(fmt.Sprintf("    local.set $r%d\n", inst.Dest))
		
	default:
		buf.WriteString(fmt.Sprintf("    ;; TODO: %s\n", inst.Op))
	}
	
	return nil
}

// Register the WASM backend
func init() {
	RegisterBackend("wasm", func(options *BackendOptions) Backend {
		return NewWASMBackend(options)
	})
}