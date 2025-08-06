package codegen

import (
	"fmt"
	"strings"
	
	"github.com/minz/minzc/pkg/ir"
)

func init() {
	RegisterBackend("llvm", func(options *BackendOptions) Backend {
		return &LLVMBackend{
			options: options,
		}
	})
}

// LLVMBackend generates LLVM IR from MinZ IR
type LLVMBackend struct {
	BaseBackend
	options *BackendOptions
}

func (b *LLVMBackend) Name() string {
	return "llvm"
}

func (b *LLVMBackend) GetFileExtension() string {
	return "ll"
}

func (b *LLVMBackend) SupportsFeature(feature string) bool {
	switch feature {
	case Feature32BitPointers,
	     FeatureFloatingPoint,
	     FeatureIndirectCalls,
	     FeatureBitManipulation,
	     FeatureHardwareMultiply,
	     FeatureHardwareDivide:
		return true
	default:
		return false
	}
}

func (b *LLVMBackend) Generate(module *ir.Module) (string, error) {
	var sb strings.Builder
	
	// LLVM IR header
	b.writeHeader(&sb)
	
	// Forward declarations for functions
	b.writeFunctionDeclarations(&sb, module)
	
	// Global variables
	b.writeGlobals(&sb, module)
	
	// String constants
	b.writeStrings(&sb, module)
	
	// Functions
	for _, fn := range module.Functions {
		if err := b.writeFunction(&sb, fn); err != nil {
			return "", err
		}
	}
	
	// Runtime functions
	b.writeRuntimeFunctions(&sb)
	
	return sb.String(), nil
}

func (b *LLVMBackend) writeHeader(sb *strings.Builder) {
	sb.WriteString("; MinZ LLVM IR generated code\n")
	sb.WriteString("; Target: LLVM IR (compatible with LLVM 10+)\n\n")
	
	// Standard declarations
	sb.WriteString("declare i32 @printf(i8*, ...)\n")
	sb.WriteString("declare i32 @putchar(i32)\n")
	sb.WriteString("declare void @exit(i32)\n")
	sb.WriteString("declare i8* @malloc(i64)\n")
	sb.WriteString("declare void @free(i8*)\n\n")
}

func (b *LLVMBackend) writeFunctionDeclarations(sb *strings.Builder, module *ir.Module) {
	sb.WriteString("; Function declarations\n")
	// For now, skip external function declarations
	// TODO: Add external function tracking to IR
	sb.WriteString("\n")
}

func (b *LLVMBackend) writeGlobals(sb *strings.Builder, module *ir.Module) {
	if len(module.Globals) == 0 {
		return
	}
	
	sb.WriteString("; Global variables\n")
	for _, g := range module.Globals {
		sb.WriteString(fmt.Sprintf("@%s = global %s %s\n",
			b.mangledName(g.Name),
			b.llvmType(g.Type),
			b.llvmInitializer(&g)))
	}
	sb.WriteString("\n")
}

func (b *LLVMBackend) writeStrings(sb *strings.Builder, module *ir.Module) {
	if len(module.Strings) == 0 {
		return
	}
	
	sb.WriteString("; String constants\n")
	for _, s := range module.Strings {
		// Escape the string properly for LLVM
		escaped := b.escapeString(s.Value)
		sb.WriteString(fmt.Sprintf("@%s = private constant [%d x i8] c\"%s\\00\"\n",
			s.Label,
			len(s.Value)+1,
			escaped))
	}
	sb.WriteString("\n")
}

func (b *LLVMBackend) writeFunction(sb *strings.Builder, fn *ir.Function) error {
	// Function signature
	sb.WriteString(fmt.Sprintf("define %s @%s(",
		b.llvmType(fn.ReturnType),
		b.mangledName(fn.Name)))
	
	// Parameters
	for i, param := range fn.Params {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(fmt.Sprintf("%s %%%s", b.llvmType(param.Type), param.Name))
	}
	sb.WriteString(") {\n")
	
	// Entry block
	sb.WriteString("entry:\n")
	
	// Allocate space for locals
	for _, local := range fn.Locals {
		sb.WriteString(fmt.Sprintf("  %%%s.addr = alloca %s\n",
			local.Name,
			b.llvmType(local.Type)))
	}
	
	// Generate instructions
	for _, inst := range fn.Instructions {
		if err := b.writeInstruction(sb, &inst); err != nil {
			return err
		}
	}
	
	// Default return if needed
	if !b.hasReturn(fn) {
		if fn.ReturnType.String() == "void" {
			sb.WriteString("  ret void\n")
		} else {
			sb.WriteString(fmt.Sprintf("  ret %s 0\n", b.llvmType(fn.ReturnType)))
		}
	}
	
	sb.WriteString("}\n\n")
	return nil
}

func (b *LLVMBackend) writeInstruction(sb *strings.Builder, inst *ir.Instruction) error {
	switch inst.Op {
	case ir.OpLabel:
		sb.WriteString(fmt.Sprintf("%s:\n", inst.Label))
		
	case ir.OpLoadConst:
		// Use simple assignment for constants
		sb.WriteString(fmt.Sprintf("  %%r%d = add %s 0, %d\n",
			inst.Dest,
			b.llvmType(inst.Type),
			inst.Imm))
			
	case ir.OpMove:
		// In LLVM, we use store/load for moves
		sb.WriteString(fmt.Sprintf("  %%r%d = %s %s %%r%d\n",
			inst.Dest,
			b.llvmMoveOp(inst.Type),
			b.llvmType(inst.Type),
			inst.Src1))
			
	case ir.OpAdd:
		sb.WriteString(fmt.Sprintf("  %%r%d = add %s %%r%d, %%r%d\n",
			inst.Dest,
			b.llvmType(inst.Type),
			inst.Src1,
			inst.Src2))
			
	case ir.OpSub:
		sb.WriteString(fmt.Sprintf("  %%r%d = sub %s %%r%d, %%r%d\n",
			inst.Dest,
			b.llvmType(inst.Type),
			inst.Src1,
			inst.Src2))
			
	case ir.OpMul:
		sb.WriteString(fmt.Sprintf("  %%r%d = mul %s %%r%d, %%r%d\n",
			inst.Dest,
			b.llvmType(inst.Type),
			inst.Src1,
			inst.Src2))
			
	case ir.OpDiv:
		op := "sdiv" // signed div
		if b.isUnsigned(inst.Type) {
			op = "udiv"
		}
		sb.WriteString(fmt.Sprintf("  %%r%d = %s %s %%r%d, %%r%d\n",
			inst.Dest,
			op,
			b.llvmType(inst.Type),
			inst.Src1,
			inst.Src2))
			
	case ir.OpEq:
		sb.WriteString(fmt.Sprintf("  %%r%d = icmp eq %s %%r%d, %%r%d\n",
			inst.Dest,
			b.llvmType(inst.Type),
			inst.Src1,
			inst.Src2))
			
	case ir.OpNe:
		sb.WriteString(fmt.Sprintf("  %%r%d = icmp ne %s %%r%d, %%r%d\n",
			inst.Dest,
			b.llvmType(inst.Type),
			inst.Src1,
			inst.Src2))
			
	case ir.OpLt:
		op := "slt" // signed less than
		if b.isUnsigned(inst.Type) {
			op = "ult"
		}
		sb.WriteString(fmt.Sprintf("  %%r%d = icmp %s %s %%r%d, %%r%d\n",
			inst.Dest,
			op,
			b.llvmType(inst.Type),
			inst.Src1,
			inst.Src2))
			
	case ir.OpGt:
		op := "sgt" // signed greater than
		if b.isUnsigned(inst.Type) {
			op = "ugt"
		}
		sb.WriteString(fmt.Sprintf("  %%r%d = icmp %s %s %%r%d, %%r%d\n",
			inst.Dest,
			op,
			b.llvmType(inst.Type),
			inst.Src1,
			inst.Src2))
			
	case ir.OpJump:
		sb.WriteString(fmt.Sprintf("  br label %%%s\n", inst.Label))
		
	case ir.OpJumpIf:
		// Need to get the next label somehow - for now use a generic "next"
		sb.WriteString(fmt.Sprintf("  br i1 %%r%d, label %%%s, label %%next\n",
			inst.Src1,
			inst.Label))
			
	case ir.OpCall:
		if inst.Dest != 0 {
			sb.WriteString(fmt.Sprintf("  %%r%d = call %s @%s(",
				inst.Dest,
				b.llvmType(inst.Type),
				b.mangledName(inst.Symbol)))
		} else {
			sb.WriteString(fmt.Sprintf("  call void @%s(", b.mangledName(inst.Symbol)))
		}
		
		// Arguments
		for i, arg := range inst.Args {
			if i > 0 {
				sb.WriteString(", ")
			}
			// TODO: Get proper type for each argument
			sb.WriteString(fmt.Sprintf("i8 %%r%d", arg))
		}
		sb.WriteString(")\n")
		
	case ir.OpReturn:
		if inst.Src1 != 0 {
			// TODO: Get proper return type from function signature
			sb.WriteString(fmt.Sprintf("  ret %s %%r%d\n", b.llvmType(inst.Type), inst.Src1))
		} else {
			sb.WriteString("  ret void\n")
		}
		
	case ir.OpStoreVar:
		// Store register to local variable
		varType := b.llvmType(inst.Type)
		if varType == "void" {
			varType = "i8" // Default to i8 if type not set
		}
		sb.WriteString(fmt.Sprintf("  store %s %%r%d, %s* %%%s.addr\n",
			varType,
			inst.Src1,
			varType,
			inst.Symbol))
		
	case ir.OpLoadVar:
		// Load local variable to register
		varType := b.llvmType(inst.Type)
		if varType == "void" {
			varType = "i8" // Default to i8 if type not set
		}
		sb.WriteString(fmt.Sprintf("  %%r%d = load %s, %s* %%%s.addr\n",
			inst.Dest,
			varType,
			varType,
			inst.Symbol))
			
	case ir.OpPrintU8:
		// Call print_u8 function
		sb.WriteString(fmt.Sprintf("  call void @print_u8(i8 %%r%d)\n", inst.Src1))
		
	default:
		// Add comment for unimplemented instructions
		sb.WriteString(fmt.Sprintf("  ; TODO: %s\n", inst.Op))
	}
	
	return nil
}

func (b *LLVMBackend) llvmType(t ir.Type) string {
	if t == nil {
		return "void"
	}
	
	switch typ := t.(type) {
	case *ir.BasicType:
		switch typ.Kind {
		case ir.TypeU8, ir.TypeI8:
			return "i8"
		case ir.TypeU16, ir.TypeI16:
			return "i16"
		// Note: MinZ doesn't have u32/i32 built-in types
	// case ir.TypeU32, ir.TypeI32:
	//	return "i32"
		case ir.TypeBool:
			return "i1"
		case ir.TypeVoid:
			return "void"
		default:
			return "i32" // Default
		}
	case *ir.PointerType:
		return fmt.Sprintf("%s*", b.llvmType(typ.Base))
	case *ir.ArrayType:
		return fmt.Sprintf("[%d x %s]", typ.Length, b.llvmType(typ.Element))
	default:
		return "i32" // Default
	}
}

func (b *LLVMBackend) llvmParams(params []ir.Parameter) string {
	var parts []string
	for _, p := range params {
		parts = append(parts, b.llvmType(p.Type))
	}
	return strings.Join(parts, ", ")
}

func (b *LLVMBackend) llvmInitializer(g *ir.Global) string {
	// Simple zero initializer for now
	switch g.Type.(type) {
	case *ir.BasicType:
		return "0"
	case *ir.PointerType:
		return "null"
	default:
		return "zeroinitializer"
	}
}

func (b *LLVMBackend) llvmConstOp(t ir.Type) string {
	// For constants, we typically use 'add' with 0
	return "add"
}

func (b *LLVMBackend) llvmMoveOp(t ir.Type) string {
	// For moves, we typically use 'add' with 0
	return "add"
}

func (b *LLVMBackend) isUnsigned(t ir.Type) bool {
	if basic, ok := t.(*ir.BasicType); ok {
		switch basic.Kind {
		case ir.TypeU8, ir.TypeU16:
			return true
		}
	}
	return false
}

func (b *LLVMBackend) hasReturn(fn *ir.Function) bool {
	for _, inst := range fn.Instructions {
		if inst.Op == ir.OpReturn {
			return true
		}
	}
	return false
}

func (b *LLVMBackend) mangledName(name string) string {
	// Simple name mangling - replace special chars
	name = strings.ReplaceAll(name, ".", "_")
	name = strings.ReplaceAll(name, "$", "_")
	name = strings.ReplaceAll(name, "?", "_qm")
	return name
}

func (b *LLVMBackend) escapeString(s string) string {
	// Escape special characters for LLVM
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "\\r")
	s = strings.ReplaceAll(s, "\t", "\\t")
	return s
}

func (b *LLVMBackend) writeRuntimeFunctions(sb *strings.Builder) {
	// print_u8 implementation
	sb.WriteString(`
; Runtime functions
define void @print_u8(i8 %value) {
  %1 = zext i8 %value to i32
  %2 = call i32 (i8*, ...) @printf(i8* getelementptr inbounds ([4 x i8], [4 x i8]* @.str.u8, i32 0, i32 0), i32 %1)
  ret void
}

@.str.u8 = private constant [4 x i8] c"%u\0A\00"

; main wrapper
define i32 @main() {
  call void @examples_test_llvm_main()
  ret i32 0
}
`)
}