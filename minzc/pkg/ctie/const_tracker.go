package ctie

import (
	"fmt"
	"github.com/minz/minzc/pkg/ir"
)

// ConstTracker tracks which values are compile-time constants
type ConstTracker struct {
	module    *ir.Module
	constants map[string]Value  // Variable/register -> constant value
	callSites map[int]*CallSite // Instruction index -> call site info
}

// CallSite represents a function call with its arguments
type CallSite struct {
	FunctionName string
	InstIndex    int
	Function     *ir.Function
	ArgValues    []Value
	IsConst      bool
}

// NewConstTracker creates a new constant tracker
func NewConstTracker(module *ir.Module) *ConstTracker {
	return &ConstTracker{
		module:    module,
		constants: make(map[string]Value),
		callSites: make(map[int]*CallSite),
	}
}

// AnalyzeFunction tracks constants through a function
func (c *ConstTracker) AnalyzeFunction(fn *ir.Function) {
	// Track constants through the function
	for i, inst := range fn.Instructions {
		switch inst.Op {
		case ir.OpLoadConst:
			// Track constant loads
			if inst.Dest != 0 {
				c.constants[c.regName(inst.Dest)] = IntValue{
					Val:  inst.Imm,
					Size: &ir.BasicType{Kind: ir.TypeU8},
				}
			}
			
		case ir.OpCall:
			// Check if all arguments are const
			c.analyzeCallSite(fn, i, &inst)
			
		case ir.OpStoreVar:
			// Track variable assignments
			if src, ok := c.constants[c.regName(inst.Src1)]; ok {
				c.constants[inst.Symbol] = src
			}
			
		case ir.OpLoadVar:
			// Propagate constants through loads
			if val, ok := c.constants[inst.Symbol]; ok {
				c.constants[c.regName(inst.Dest)] = val
			}
			
		case ir.OpAdd, ir.OpSub, ir.OpMul, ir.OpDiv:
			// Compute arithmetic on constants
			c.computeArithmetic(&inst)
			
		default:
			// Conservative: clear dest register if not const
			if inst.Dest != 0 {
				delete(c.constants, c.regName(inst.Dest))
			}
		}
	}
}

// analyzeCallSite analyzes a function call for const arguments
func (c *ConstTracker) analyzeCallSite(fn *ir.Function, index int, inst *ir.Instruction) {
	// Find the called function
	var calledFn *ir.Function
	for _, f := range c.module.Functions {
		if f.Name == inst.Symbol {
			calledFn = f
			break
		}
	}
	
	if calledFn == nil {
		return
	}
	
	// Special case: if function has no parameters, it's always const!
	if len(calledFn.Params) == 0 {
		c.callSites[index] = &CallSite{
			FunctionName: inst.Symbol,
			InstIndex:    index,
			Function:     calledFn,
			ArgValues:    []Value{},
			IsConst:      true,
		}
		return
	}
	
	// Check if all arguments are const
	argValues := make([]Value, 0, len(inst.Args))
	allConst := true
	
	// Look back in the instruction stream for argument setup
	// In MinZ, arguments are often loaded just before the call
	if index > 0 {
		// Check previous instructions for LoadConst operations
		for i := index - 1; i >= 0 && i >= index-10; i-- {
			prevInst := &fn.Instructions[i]
			if prevInst.Op == ir.OpLoadConst && prevInst.Dest != 0 {
				// Track this constant
				c.constants[c.regName(prevInst.Dest)] = IntValue{
					Val:  prevInst.Imm,
					Size: &ir.BasicType{Kind: ir.TypeU8},
				}
			}
		}
	}
	
	// Now check Args
	for _, argReg := range inst.Args {
		if val, ok := c.constants[c.regName(argReg)]; ok {
			argValues = append(argValues, val)
		} else {
			allConst = false
			break
		}
	}
	
	// If not enough info from Args, try to infer from function signature
	if len(argValues) == 0 && len(calledFn.Params) > 0 {
		// Try to get args from stack/registers
		// This is simplified - real implementation would track stack
		for i := 0; i < len(calledFn.Params); i++ {
			// Check if we have a const value for this parameter position
			paramReg := ir.Register(i + 1) // Simplified: assume args in r1, r2, etc
			if val, ok := c.constants[c.regName(paramReg)]; ok {
				argValues = append(argValues, val)
			} else {
				allConst = false
				break
			}
		}
	}
	
	// Record the call site
	c.callSites[index] = &CallSite{
		FunctionName: inst.Symbol,
		InstIndex:    index,
		Function:     calledFn,
		ArgValues:    argValues,
		IsConst:      allConst && len(argValues) == len(calledFn.Params),
	}
}

// computeArithmetic computes arithmetic operations on constants
func (c *ConstTracker) computeArithmetic(inst *ir.Instruction) {
	src1Val, src1Ok := c.constants[c.regName(inst.Src1)]
	src2Val, src2Ok := c.constants[c.regName(inst.Src2)]
	
	if !src1Ok || !src2Ok {
		// Not both constant
		if inst.Dest != 0 {
			delete(c.constants, c.regName(inst.Dest))
		}
		return
	}
	
	// Compute the result
	var result int64
	switch inst.Op {
	case ir.OpAdd:
		result = src1Val.ToInt() + src2Val.ToInt()
	case ir.OpSub:
		result = src1Val.ToInt() - src2Val.ToInt()
	case ir.OpMul:
		result = src1Val.ToInt() * src2Val.ToInt()
	case ir.OpDiv:
		if src2Val.ToInt() != 0 {
			result = src1Val.ToInt() / src2Val.ToInt()
		} else {
			// Division by zero - not const
			delete(c.constants, c.regName(inst.Dest))
			return
		}
	}
	
	// Store the result
	c.constants[c.regName(inst.Dest)] = IntValue{
		Val:  result,
		Size: inst.Type,
	}
}

// GetConstCallSites returns all call sites with constant arguments
func (c *ConstTracker) GetConstCallSites() []*CallSite {
	var sites []*CallSite
	for _, site := range c.callSites {
		if site.IsConst {
			sites = append(sites, site)
		}
	}
	return sites
}

// regName returns a string name for a register
func (c *ConstTracker) regName(reg ir.Register) string {
	return fmt.Sprintf("r%d", reg)
}

// Clear resets the tracker
func (c *ConstTracker) Clear() {
	c.constants = make(map[string]Value)
	c.callSites = make(map[int]*CallSite)
}