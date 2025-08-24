// Package codegen provides Crystal backend for MinZ compiler
// This enables MinZ → Crystal transpilation for modern development workflow
package codegen

import (
	"fmt"
	"strings"

	"github.com/minz/minzc/pkg/ir"
)

// CrystalBackend implements Backend interface for Crystal code generation
type CrystalBackend struct {
	options      *BackendOptions
	output       strings.Builder
	indent       int
	currentFunc  string
	labelCounter int
}

// NewCrystalBackend creates a new Crystal backend
func NewCrystalBackend(options *BackendOptions) Backend {
	return &CrystalBackend{
		options: options,
	}
}

// Name returns the name of this backend
func (c *CrystalBackend) Name() string {
	return "crystal"
}

// GetFileExtension returns the file extension for Crystal code
func (c *CrystalBackend) GetFileExtension() string {
	return ".cr"
}

// SupportsFeature checks if this backend supports a specific feature
func (c *CrystalBackend) SupportsFeature(feature string) bool {
	switch feature {
	case Feature32BitPointers, FeatureFloatingPoint, FeatureHardwareMultiply, FeatureHardwareDivide:
		return true
	case FeatureSelfModifyingCode, FeatureInlineAssembly, FeatureBitManipulation, FeatureZeroPage:
		return false // Crystal can't do these
	default:
		return false
	}
}

// Generate converts MIR module to Crystal source code (implements Backend interface)
func (c *CrystalBackend) Generate(module *ir.Module) (string, error) {
	c.output.Reset()
	c.indent = 0

	// Write Crystal module header with Ruby-style interpolation support
	c.writeLine("# Generated Crystal code from MinZ compiler v0.15.0")
	c.writeLine("# Ruby-style interpolation maps perfectly to Crystal syntax!")
	c.writeLine("")

	// Generate global constants
	for _, global := range module.Globals {
		if err := c.generateGlobal(global); err != nil {
			return "", fmt.Errorf("generating global %s: %w", global.Name, err)
		}
	}

	// Generate all functions
	for _, function := range module.Functions {
		if err := c.generateFunction(function); err != nil {
			return "", fmt.Errorf("generating function %s: %w", function.Name, err)
		}
	}

	return c.output.String(), nil
}

// generateFunction converts IR function to Crystal method
func (c *CrystalBackend) generateFunction(fn *ir.Function) error {
	c.currentFunc = fn.Name

	// Build parameter list
	params := make([]string, len(fn.Params))
	for i, param := range fn.Params {
		params[i] = fmt.Sprintf("%s : %s", param.Name, c.mapType(param.Type))
	}

	// Generate function signature
	returnType := c.mapType(fn.ReturnType)
	if len(params) > 0 {
		c.writeLine(fmt.Sprintf("def %s(%s) : %s", fn.Name, strings.Join(params, ", "), returnType))
	} else {
		c.writeLine(fmt.Sprintf("def %s : %s", fn.Name, returnType))
	}

	c.indent++

	// Generate local variables
	for _, local := range fn.Locals {
		crystalType := c.mapType(local.Type)
		c.writeLine(fmt.Sprintf("%s = uninitialized %s", local.Name, crystalType))
	}

	// Generate function body from IR instructions
	for _, instruction := range fn.Instructions {
		if err := c.generateInstruction(instruction); err != nil {
			return fmt.Errorf("generating instruction in %s: %w", fn.Name, err)
		}
	}

	c.indent--
	c.writeLine("end")
	c.writeLine("")

	return nil
}

// generateInstruction converts single IR instruction to Crystal statement
func (c *CrystalBackend) generateInstruction(instr ir.Instruction) error {
	switch instr.Op {
	case ir.OpLoadConst:
		return c.generateLoadConst(&instr)
	case ir.OpLoadVar:
		return c.generateLoadVar(&instr)
	case ir.OpStoreVar:
		return c.generateStoreVar(&instr)
	case ir.OpCall:
		return c.generateCall(&instr)
	case ir.OpReturn:
		return c.generateReturn(&instr)
	case ir.OpLabel:
		return c.generateLabel(&instr)
	case ir.OpJump:
		return c.generateJump(&instr)
	case ir.OpJumpIf, ir.OpJumpIfNot:
		return c.generateConditionalJump(&instr)
	case ir.OpAdd, ir.OpSub, ir.OpMul, ir.OpDiv:
		return c.generateArithmetic(&instr)
	case ir.OpPrint, ir.OpPrintU8, ir.OpPrintString:
		return c.generatePrint(&instr)
	default:
		// Fallback for unhandled instructions
		c.writeLine(fmt.Sprintf("# TODO: Unhandled instruction: %s", instr.Op))
		return nil
	}
}

// generateLoadConst handles loading constants
func (c *CrystalBackend) generateLoadConst(inst *ir.Instruction) error {
	regName := c.getRegisterName(inst.Dest)
	c.writeLine(fmt.Sprintf("%s = %d", regName, inst.Imm))
	return nil
}

// generateLoadVar handles variable loads
func (c *CrystalBackend) generateLoadVar(inst *ir.Instruction) error {
	regName := c.getRegisterName(inst.Dest)
	c.writeLine(fmt.Sprintf("%s = %s", regName, inst.Symbol))
	return nil
}

// generateStoreVar handles variable stores
func (c *CrystalBackend) generateStoreVar(inst *ir.Instruction) error {
	srcName := c.getRegisterName(inst.Src1)
	c.writeLine(fmt.Sprintf("%s = %s", inst.Symbol, srcName))
	return nil
}

// generateCall handles function calls
func (c *CrystalBackend) generateCall(inst *ir.Instruction) error {
	funcName := inst.Symbol
	if funcName == "" {
		funcName = inst.FuncName
	}
	
	// Generate function call
	crystalCall := c.mapFunctionCall(funcName, []string{})
	
	if inst.Dest != 0 {
		destName := c.getRegisterName(inst.Dest)
		c.writeLine(fmt.Sprintf("%s = %s", destName, crystalCall))
	} else {
		c.writeLine(crystalCall)
	}
	return nil
}

// generateReturn handles return statements
func (c *CrystalBackend) generateReturn(inst *ir.Instruction) error {
	if inst.Src1 != 0 {
		srcName := c.getRegisterName(inst.Src1)
		c.writeLine(fmt.Sprintf("return %s", srcName))
	} else {
		c.writeLine("return")
	}
	return nil
}

// generateLabel handles label definitions
func (c *CrystalBackend) generateLabel(inst *ir.Instruction) error {
	c.writeLine(fmt.Sprintf("# label %s:", inst.Label))
	return nil
}

// generateJump handles unconditional jumps
func (c *CrystalBackend) generateJump(inst *ir.Instruction) error {
	c.writeLine(fmt.Sprintf("# goto %s", inst.Label))
	return nil
}

// generateConditionalJump handles conditional jumps
func (c *CrystalBackend) generateConditionalJump(inst *ir.Instruction) error {
	condReg := c.getRegisterName(inst.Src1)
	if inst.Op == ir.OpJumpIf {
		c.writeLine(fmt.Sprintf("if %s", condReg))
	} else {
		c.writeLine(fmt.Sprintf("if !%s", condReg))
	}
	c.indent++
	c.writeLine(fmt.Sprintf("# goto %s", inst.Label))
	c.indent--
	c.writeLine("end")
	return nil
}

// generateArithmetic handles arithmetic operations
func (c *CrystalBackend) generateArithmetic(inst *ir.Instruction) error {
	destName := c.getRegisterName(inst.Dest)
	src1Name := c.getRegisterName(inst.Src1)
	src2Name := c.getRegisterName(inst.Src2)
	
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
	}
	
	c.writeLine(fmt.Sprintf("%s = %s %s %s", destName, src1Name, op, src2Name))
	return nil
}

// generatePrint handles print operations
func (c *CrystalBackend) generatePrint(inst *ir.Instruction) error {
	switch inst.Op {
	case ir.OpPrint:
		srcName := c.getRegisterName(inst.Src1)
		c.writeLine(fmt.Sprintf("print %s.chr", srcName))
	case ir.OpPrintU8:
		srcName := c.getRegisterName(inst.Src1)
		c.writeLine(fmt.Sprintf("print %s", srcName))
	case ir.OpPrintString:
		c.writeLine(fmt.Sprintf("print \"string_%d\"", inst.StringID))
	}
	return nil
}

// getRegisterName returns a Crystal variable name for a register
func (c *CrystalBackend) getRegisterName(reg ir.Register) string {
	if reg == 0 {
		return "nil"
	}
	if reg < 0 {
		switch reg {
		case ir.RegSP:
			return "sp"
		case ir.RegFP:
			return "fp"
		case ir.RegRet:
			return "ret_val"
		default:
			return fmt.Sprintf("special_%d", reg)
		}
	}
	return fmt.Sprintf("r%d", reg)
}

// mapType converts MinZ types to Crystal types
func (c *CrystalBackend) mapType(t ir.Type) string {
	switch typ := t.(type) {
	case *ir.BasicType:
		switch typ.Kind {
		case ir.TypeU8:
			return "UInt8"
		case ir.TypeU16:
			return "UInt16"
		case ir.TypeI8:
			return "Int8"
		case ir.TypeI16:
			return "Int16"
		case ir.TypeBool:
			return "Bool"
		case ir.TypeVoid:
			return "Nil"
		default:
			return "Unknown"
		}
	case *ir.PointerType:
		return fmt.Sprintf("Pointer(%s)", c.mapType(typ.Base))
	case *ir.ArrayType:
		return fmt.Sprintf("StaticArray(%s, %d)", c.mapType(typ.Element), typ.Length)
	case *ir.StructType:
		return typ.Name
	default:
		return "Unknown"
	}
}


// mapFunctionCall converts MinZ function calls to Crystal equivalents
func (c *CrystalBackend) mapFunctionCall(name string, args []string) string {
	switch name {
	case "print_u8":
		if len(args) > 0 {
			return fmt.Sprintf("print %s", args[0])
		}
		return "print"
	case "print":
		if len(args) > 0 {
			return fmt.Sprintf("print %s", args[0])
		}
		return "print"
	case "strlen":
		if len(args) > 0 {
			return fmt.Sprintf("%s.size", args[0])
		}
		return "0"
	default:
		// Regular function call
		if len(args) > 0 {
			return fmt.Sprintf("%s(%s)", name, strings.Join(args, ", "))
		}
		return fmt.Sprintf("%s()", name)
	}
}

// generateGlobal creates Crystal module constants
func (c *CrystalBackend) generateGlobal(global ir.Global) error {
	crystalType := c.mapType(global.Type)
	
	if global.Value != nil {
		c.writeLine(fmt.Sprintf("%s : %s = %v", strings.ToUpper(global.Name), crystalType, global.Value))
	} else if global.Init != nil {
		c.writeLine(fmt.Sprintf("%s : %s = %v", strings.ToUpper(global.Name), crystalType, global.Init))
	} else {
		c.writeLine(fmt.Sprintf("%s = uninitialized %s", strings.ToUpper(global.Name), crystalType))
	}
	
	return nil
}

// writeLine writes a line with proper indentation
func (c *CrystalBackend) writeLine(line string) {
	if line != "" {
		c.output.WriteString(strings.Repeat("  ", c.indent))
		c.output.WriteString(line)
	}
	c.output.WriteString("\n")
}

// Register the Crystal backend
func init() {
	RegisterBackend("crystal", func(options *BackendOptions) Backend {
		return NewCrystalBackend(options)
	})
}