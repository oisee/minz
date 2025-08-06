package codegen

import (
	"fmt"
	"io"
	"strings"
	"github.com/minz/minzc/pkg/ir"
)

// CGenerator generates C code from IR
type CGenerator struct {
	backend    *CBackend
	module     *ir.Module
	output     io.Writer
	indent     int
	currentFunc *ir.Function
	varTypes   map[string]string // Track variable types
	timestamp  string
	tempCounter int
}

func (g *CGenerator) Generate() error {
	// Generate header
	g.emit("// MinZ C generated code")
	g.emit("// Generated: %s", g.timestamp)
	g.emit("// Target: Standard C (C99)")
	g.emit("")
	g.emit("#include <stdio.h>")
	g.emit("#include <stdint.h>")
	g.emit("#include <stdbool.h>")
	g.emit("#include <stdlib.h>")
	g.emit("#include <string.h>")
	g.emit("")
	
	// Generate type definitions
	g.emit("// Type definitions")
	g.emit("typedef uint8_t u8;")
	g.emit("typedef uint16_t u16;")
	g.emit("typedef uint32_t u24; // 24-bit emulated as 32-bit")
	g.emit("typedef uint32_t u32;")
	g.emit("typedef int8_t i8;")
	g.emit("typedef int16_t i16;")
	g.emit("typedef int32_t i24; // 24-bit emulated as 32-bit")
	g.emit("typedef int32_t i32;")
	g.emit("")
	
	// Fixed-point type helpers
	g.emit("// Fixed-point arithmetic helpers")
	g.emit("typedef int16_t f8_8;   // 8.8 fixed-point")
	g.emit("typedef int16_t f_8;    // .8 fixed-point")
	g.emit("typedef int16_t f_16;   // .16 fixed-point")
	g.emit("typedef int32_t f16_8;  // 16.8 fixed-point")
	g.emit("typedef int32_t f8_16;  // 8.16 fixed-point")
	g.emit("")
	g.emit("#define F8_8_SHIFT 8")
	g.emit("#define F_8_SHIFT 8")
	g.emit("#define F_16_SHIFT 16")
	g.emit("#define F16_8_SHIFT 8")
	g.emit("#define F8_16_SHIFT 16")
	g.emit("")
	
	// Generate string type
	g.emit("// String type (length-prefixed)")
	g.emit("typedef struct {")
	g.emit("    uint16_t len;")
	g.emit("    char* data;")
	g.emit("} String;")
	g.emit("")
	
	// Generate print helpers
	g.generatePrintHelpers()
	
	// Forward declare all functions
	g.emit("// Function declarations")
	for _, fn := range g.module.Functions {
		g.generateFunctionDeclaration(fn)
	}
	g.emit("")
	
	// Generate global variables
	if len(g.module.Globals) > 0 {
		g.emit("// Global variables")
		for _, global := range g.module.Globals {
			g.generateGlobal(&global)
		}
		g.emit("")
	}
	
	// Generate string literals
	if len(g.module.Strings) > 0 {
		g.emit("// String literals")
		for _, str := range g.module.Strings {
			g.generateString(str)
		}
		g.emit("")
	}
	
	// Generate function implementations
	for _, fn := range g.module.Functions {
		if err := g.generateFunction(fn); err != nil {
			return fmt.Errorf("generating function %s: %w", fn.Name, err)
		}
		g.emit("")
	}
	
	// Generate main wrapper if needed
	g.generateMainWrapper()
	
	return nil
}

func (g *CGenerator) generatePrintHelpers() {
	g.emit("// Print helper functions")
	g.emit("void print_char(u8 ch) {")
	g.emit("    putchar(ch);")
	g.emit("}")
	g.emit("")
	g.emit("void print_u8(u8 value) {")
	g.emit("    printf(\"%%u\", value);")
	g.emit("}")
	g.emit("")
	g.emit("void print_u8_decimal(u8 value) {")
	g.emit("    printf(\"%%u\", value);")
	g.emit("}")
	g.emit("")
	g.emit("void print_u16(u16 value) {")
	g.emit("    printf(\"%%u\", value);")
	g.emit("}")
	g.emit("")
	g.emit("void print_u24(u24 value) {")
	g.emit("    printf(\"%%u\", value);")
	g.emit("}")
	g.emit("")
	g.emit("void print_i8(i8 value) {")
	g.emit("    printf(\"%%d\", value);")
	g.emit("}")
	g.emit("")
	g.emit("void print_i16(i16 value) {")
	g.emit("    printf(\"%%d\", value);")
	g.emit("}")
	g.emit("")
	g.emit("void print_newline() {")
	g.emit("    printf(\"\\n\");")
	g.emit("}")
	g.emit("")
	g.emit("void print_string(String* str) {")
	g.emit("    if (str && str->data) {")
	g.emit("        printf(\"%%.*s\", str->len, str->data);")
	g.emit("    }")
	g.emit("}")
	g.emit("")
}

func (g *CGenerator) generateFunctionDeclaration(fn *ir.Function) {
	returnType := g.getCType(fn.ReturnType)
	g.emit("%s %s(%s);", returnType, g.sanitizeName(fn.Name), g.getParameterList(fn))
}

func (g *CGenerator) generateFunction(fn *ir.Function) error {
	g.currentFunc = fn
	returnType := g.getCType(fn.ReturnType)
	
	// Function signature
	g.emit("%s %s(%s) {", returnType, g.sanitizeName(fn.Name), g.getParameterList(fn))
	g.indent++
	
	// Declare locals based on virtual registers used
	// We'll use a simple approach - declare r0 through rN based on instructions
	maxReg := g.findMaxRegister(fn)
	for i := ir.Register(1); i <= maxReg; i++ {
		// Use uintptr_t for registers that might hold addresses
		g.emit("uintptr_t r%d = 0;", i)
		g.varTypes[fmt.Sprintf("r%d", i)] = "uintptr_t"
	}
	
	// Declare local variables from the function's Locals slice
	if len(fn.Locals) > 0 {
		g.emit("")
		g.emit("// Local variables")
		for _, local := range fn.Locals {
			if local.Name != "" { // Skip empty variable names
				cType := g.getCType(local.Type)
				g.emit("%s %s = 0;", cType, local.Name)
				g.varTypes[local.Name] = cType
			}
		}
	}
	
	if maxReg > 0 {
		g.emit("")
	}
	
	// Track argument registers
	argRegs := make(map[string]ir.Register)
	for i, param := range fn.Params {
		argRegs[param.Name] = ir.Register(i + 1) // Simple mapping
	}
	
	// Generate instructions
	for _, inst := range fn.Instructions {
		if err := g.generateInstruction(&inst); err != nil {
			return err
		}
	}
	
	g.indent--
	g.emit("}")
	
	return nil
}

func (g *CGenerator) findMaxRegister(fn *ir.Function) ir.Register {
	var max ir.Register
	for _, inst := range fn.Instructions {
		if inst.Dest > max {
			max = inst.Dest
		}
		if inst.Src1 > max {
			max = inst.Src1
		}
		if inst.Src2 > max {
			max = inst.Src2
		}
	}
	return max
}

func (g *CGenerator) generateInstruction(inst *ir.Instruction) error {
	switch inst.Op {
	case ir.OpLoadConst:
		varName := g.getVarName(inst.Dest)
		// Use Imm field for constants
		g.emit("%s = %d;", varName, inst.Imm)
		
	case ir.OpLoadVar:
		dest := g.getVarName(inst.Dest)
		src := inst.Symbol
		g.emit("%s = %s;", dest, src)
		
	case ir.OpStoreVar:
		varName := inst.Symbol
		src := g.getVarName(inst.Src1)
		if varName == "" {
			// Skip store instructions with empty variable names
			g.emit("// Skipping store to empty variable name")
		} else {
			g.emit("%s = %s;", varName, src)
		}
		
	case ir.OpLoadParam:
		dest := g.getVarName(inst.Dest)
		paramName := inst.Symbol
		g.emit("%s = %s;", dest, paramName)
		
	case ir.OpAdd, ir.OpSub, ir.OpMul, ir.OpDiv, ir.OpMod:
		g.generateBinaryOp(inst)
		
	case ir.OpAnd, ir.OpOr, ir.OpXor:
		g.generateBitwiseOp(inst)
		
	case ir.OpNot:
		dest := g.getVarName(inst.Dest)
		src := g.getVarName(inst.Src1)
		g.emit("%s = ~%s;", dest, src)
		
	case ir.OpShl, ir.OpShr:
		g.generateShiftOp(inst)
		
	case ir.OpLt, ir.OpLe, ir.OpGt, ir.OpGe, ir.OpEq, ir.OpNe:
		g.generateComparisonOp(inst)
		
	case ir.OpJump:
		label := inst.Label
		g.emit("goto %s;", label)
		
	case ir.OpJumpIf:
		cond := g.getVarName(inst.Src1)
		label := inst.Label
		g.emit("if (%s) goto %s;", cond, label)
		
	case ir.OpJumpIfNot:
		cond := g.getVarName(inst.Src1)
		label := inst.Label
		g.emit("if (!%s) goto %s;", cond, label)
		
	case ir.OpLabel:
		label := inst.Label
		g.indent--
		g.emit("%s:", label)
		g.indent++
		
	case ir.OpCall:
		g.generateCall(inst)
		
	case ir.OpReturn:
		if inst.Src1 != 0 {
			g.emit("return %s;", g.getVarName(inst.Src1))
		} else {
			g.emit("return;")
		}
		
	case ir.OpPrint, ir.OpPrintU8, ir.OpPrintU16, ir.OpPrintI8, ir.OpPrintI16,
	     ir.OpPrintBool, ir.OpPrintString, ir.OpPrintStringDirect:
		g.generatePrint(inst)
		
	case ir.OpLoadString:
		// Load address of string literal
		dest := g.getVarName(inst.Dest)
		if inst.Symbol != "" {
			// The symbol contains the string label, not the actual string
			// We need to emit the actual string value
			g.emit("%s = (uintptr_t)%s;", dest, inst.Symbol)
		}
		
	case ir.OpAsm:
		g.emit("// Inline assembly not supported in C backend")
		g.emit("// %s", inst.AsmCode)
		
	case ir.OpInc:
		// Increment optimization for C
		varName := g.getVarName(inst.Dest)
		src := g.getVarName(inst.Src1)
		if varName == src {
			g.emit("%s++;", varName)
		} else {
			g.emit("%s = %s + 1;", varName, src)
		}
		
	case ir.OpDec:
		// Decrement optimization for C  
		varName := g.getVarName(inst.Dest)
		src := g.getVarName(inst.Src1)
		if varName == src {
			g.emit("%s--;", varName)
		} else {
			g.emit("%s = %s - 1;", varName, src)
		}
		
	case ir.OpLoadIndex:
		// Load element from array: dest = array[index]
		dest := g.getVarName(inst.Dest)
		array := g.getVarName(inst.Src1)
		index := g.getVarName(inst.Src2)
		
		// Determine element type for proper casting
		if inst.Type != nil {
			// Cast array pointer to appropriate element type pointer
			elementType := g.getCType(inst.Type)
			g.emit("%s = ((%s*)%s)[%s];", dest, elementType, array, index)
		} else {
			// Default to u8 arrays
			g.emit("%s = ((u8*)%s)[%s];", dest, array, index)
		}
		
	case ir.OpLoadAddr:
		// Load address of a variable/array
		dest := g.getVarName(inst.Dest)
		if inst.Symbol != "" {
			// Load address of named variable/array
			g.emit("%s = (u32)&%s;", dest, inst.Symbol)
		} else {
			// Load address from register (for nested arrays)
			src := g.getVarName(inst.Src1)
			g.emit("%s = %s;", dest, src)
		}
		
	default:
		return fmt.Errorf("unsupported operation: %v", inst.Op)
	}
	
	return nil
}

func (g *CGenerator) generateBinaryOp(inst *ir.Instruction) {
	dest := g.getVarName(inst.Dest)
	src1 := g.getVarName(inst.Src1)
	src2 := g.getVarName(inst.Src2)
	
	var op string
	switch inst.Op {
	case ir.OpAdd:
		op = "+"
	case ir.OpSub:
		op = "-"
	case ir.OpMul:
		op = "*"
	case ir.OpDiv:
		op = "/"
	case ir.OpMod:
		op = "%"
	}
	
	g.emit("%s = %s %s %s;", dest, src1, op, src2)
}

func (g *CGenerator) generateBitwiseOp(inst *ir.Instruction) {
	dest := g.getVarName(inst.Dest)
	src1 := g.getVarName(inst.Src1)
	src2 := g.getVarName(inst.Src2)
	
	var op string
	switch inst.Op {
	case ir.OpAnd:
		op = "&"
	case ir.OpOr:
		op = "|"
	case ir.OpXor:
		op = "^"
	}
	
	g.emit("%s = %s %s %s;", dest, src1, op, src2)
}

func (g *CGenerator) generateShiftOp(inst *ir.Instruction) {
	dest := g.getVarName(inst.Dest)
	src1 := g.getVarName(inst.Src1)
	src2 := g.getVarName(inst.Src2)
	
	var op string
	switch inst.Op {
	case ir.OpShl:
		op = "<<"
	case ir.OpShr:
		op = ">>"
	}
	
	g.emit("%s = %s %s %s;", dest, src1, op, src2)
}

func (g *CGenerator) generateComparisonOp(inst *ir.Instruction) {
	dest := g.getVarName(inst.Dest)
	src1 := g.getVarName(inst.Src1)
	src2 := g.getVarName(inst.Src2)
	
	var op string
	switch inst.Op {
	case ir.OpLt:
		op = "<"
	case ir.OpLe:
		op = "<="
	case ir.OpGt:
		op = ">"
	case ir.OpGe:
		op = ">="
	case ir.OpEq:
		op = "=="
	case ir.OpNe:
		op = "!="
	}
	
	g.emit("%s = (%s %s %s);", dest, src1, op, src2)
}

func (g *CGenerator) generateCall(inst *ir.Instruction) {
	funcName := inst.Symbol
	
	// Build argument list from Args registers
	args := make([]string, len(inst.Args))
	for i, argReg := range inst.Args {
		args[i] = g.getVarName(argReg)
	}
	
	// Check if this is a print function or other void-returning function
	isVoidFunction := g.isVoidFunction(funcName)
	
	if inst.Dest != 0 && !isVoidFunction {
		dest := g.getVarName(inst.Dest)
		g.emit("%s = %s(%s);", dest, g.sanitizeName(funcName), strings.Join(args, ", "))
	} else {
		g.emit("%s(%s);", g.sanitizeName(funcName), strings.Join(args, ", "))
		// If trying to assign from a void function, set destination to 0
		if inst.Dest != 0 && isVoidFunction {
			dest := g.getVarName(inst.Dest)
			g.emit("%s = 0; // void function call result", dest)
		}
	}
}


func (g *CGenerator) generatePrint(inst *ir.Instruction) {
	switch inst.Op {
	case ir.OpPrint:
		// Print single character
		value := g.getVarName(inst.Src1)
		g.emit("putchar(%s);", value)
		
	case ir.OpPrintU8:
		value := g.getVarName(inst.Src1)
		g.emit("printf(\"%%u\", (unsigned)%s);", value)
		
	case ir.OpPrintU16:
		value := g.getVarName(inst.Src1)
		g.emit("printf(\"%%u\", (unsigned)%s);", value)
		
	case ir.OpPrintI8:
		value := g.getVarName(inst.Src1)
		g.emit("printf(\"%%d\", (int)%s);", value)
		
	case ir.OpPrintI16:
		value := g.getVarName(inst.Src1)
		g.emit("printf(\"%%d\", (int)%s);", value)
		
	case ir.OpPrintBool:
		value := g.getVarName(inst.Src1)
		g.emit("printf(\"%%s\", %s ? \"true\" : \"false\");", value)
		
	case ir.OpPrintString:
		// Print string literal
		if inst.Symbol != "" {
			// String literal - symbol contains the label
			g.emit("printf(\"%%s\", %s);", inst.Symbol)
		} else {
			// String from register/variable (which is a pointer to the string)
			value := g.getVarName(inst.Src1)
			g.emit("printf(\"%%s\", (const char*)%s);", value)
		}
		
	case ir.OpPrintStringDirect:
		// Direct string literal from Symbol field
		if inst.Symbol != "" {
			// Escape the string properly for C
			escaped := strings.ReplaceAll(inst.Symbol, "\\", "\\\\")
			escaped = strings.ReplaceAll(escaped, "\"", "\\\"")
			escaped = strings.ReplaceAll(escaped, "\n", "\\n")
			escaped = strings.ReplaceAll(escaped, "\t", "\\t")
			escaped = strings.ReplaceAll(escaped, "\r", "\\r")
			g.emit("printf(\"%s\");", escaped)
		}
		
	default:
		// Fallback for generic print
		value := g.getVarName(inst.Src1)
		g.emit("printf(\"%%d\", %s);", value)
	}
}

func (g *CGenerator) generateGlobal(global *ir.Global) {
	cType := g.getCType(global.Type)
	if global.Init != nil {
		g.emit("%s %s = %s;", cType, global.Name, g.formatConstant(global.Init))
	} else {
		g.emit("%s %s;", cType, global.Name)
	}
}

func (g *CGenerator) generateString(str *ir.String) {
	// Generate a C string literal as a const char array
	escaped := strings.ReplaceAll(str.Value, "\\", "\\\\")
	escaped = strings.ReplaceAll(escaped, "\"", "\\\"")
	escaped = strings.ReplaceAll(escaped, "\n", "\\n")
	escaped = strings.ReplaceAll(escaped, "\t", "\\t")
	escaped = strings.ReplaceAll(escaped, "\r", "\\r")
	
	g.emit("const char %s[] = \"%s\";", str.Label, escaped)
}

func (g *CGenerator) generateMainWrapper() {
	// If there's a main function, generate C main that calls it
	for _, fn := range g.module.Functions {
		if fn.Name == "main" || strings.HasSuffix(fn.Name, ".main") {
			g.emit("// C main wrapper")
			g.emit("int main(int argc, char** argv) {")
			g.indent++
			// Check if return type is void
			isVoid := false
			if fn.ReturnType == nil {
				isVoid = true
			} else if bt, ok := fn.ReturnType.(*ir.BasicType); ok && bt.Kind == ir.TypeVoid {
				isVoid = true
			}
			
			if !isVoid {
				g.emit("return (int)%s();", g.sanitizeName(fn.Name))
			} else {
				g.emit("%s();", g.sanitizeName(fn.Name))
				g.emit("return 0;")
			}
			g.indent--
			g.emit("}")
			break
		}
	}
}

func (g *CGenerator) getCType(t ir.Type) string {
	if t == nil {
		return "void"
	}
	
	// Check if it's a basic type
	if bt, ok := t.(*ir.BasicType); ok {
		switch bt.Kind {
		case ir.TypeU8:
			return "u8"
		case ir.TypeU16:
			return "u16"
		case ir.TypeU24:
			return "u24"
		case ir.TypeI8:
			return "i8"
		case ir.TypeI16:
			return "i16"
		case ir.TypeI24:
			return "i24"
		case ir.TypeBool:
			return "bool"
		case ir.TypeVoid:
			return "void"
		case ir.TypeF8_8:
			return "f8_8"
		case ir.TypeF_8:
			return "f_8"
		case ir.TypeF_16:
			return "f_16"
		case ir.TypeF16_8:
			return "f16_8"
		case ir.TypeF8_16:
			return "f8_16"
		default:
			return "void*"
		}
	}
	
	// Check for other types
	if _, ok := t.(*ir.StringType); ok {
		return "String"
	}
	if _, ok := t.(*ir.LStringType); ok {
		return "String" // Use same C type for both
	}
	if pt, ok := t.(*ir.PointerType); ok {
		return g.getCType(pt.Base) + "*"
	}
	if at, ok := t.(*ir.ArrayType); ok {
		return g.getCType(at.Element) + "*" // Arrays are pointers in C
	}
	
	return "void*" // Unknown type
}

func (g *CGenerator) getVarName(reg ir.Register) string {
	if reg == 0 {
		return ""
	}
	return fmt.Sprintf("r%d", reg)
}

func (g *CGenerator) getParameterList(fn *ir.Function) string {
	if len(fn.Params) == 0 {
		return "void"
	}
	
	params := make([]string, len(fn.Params))
	for i, param := range fn.Params {
		params[i] = fmt.Sprintf("%s %s", g.getCType(param.Type), param.Name)
	}
	
	return strings.Join(params, ", ")
}

func (g *CGenerator) formatConstant(value interface{}) string {
	switch v := value.(type) {
	case int:
		return fmt.Sprintf("%d", v)
	case int64:
		return fmt.Sprintf("%d", v)
	case uint64:
		return fmt.Sprintf("%d", v)
	case float64:
		return fmt.Sprintf("%f", v)
	case bool:
		if v {
			return "true"
		}
		return "false"
	case string:
		return fmt.Sprintf("\"%s\"", v)
	default:
		return "0"
	}
}

func (g *CGenerator) sanitizeName(name string) string {
	// Replace dots with underscores for C compatibility
	return strings.ReplaceAll(name, ".", "_")
}

func (g *CGenerator) isVoidFunction(funcName string) bool {
	// List of known void functions
	voidFunctions := []string{
		"print_char", "print_u8", "print_u16", "print_i8", "print_i16", 
		"print_newline", "print_string", "print_u8_decimal",
	}
	
	for _, voidFunc := range voidFunctions {
		if strings.Contains(funcName, voidFunc) {
			return true
		}
	}
	
	// Check if function exists in module and is void
	for _, fn := range g.module.Functions {
		if fn.Name == funcName {
			if fn.ReturnType == nil {
				return true
			}
			if bt, ok := fn.ReturnType.(*ir.BasicType); ok && bt.Kind == ir.TypeVoid {
				return true
			}
		}
	}
	
	return false
}

func (g *CGenerator) emit(format string, args ...interface{}) {
	for i := 0; i < g.indent; i++ {
		fmt.Fprint(g.output, "    ")
	}
	fmt.Fprintf(g.output, format, args...)
	fmt.Fprintln(g.output)
}