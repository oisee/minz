package interpreter

import (
	"fmt"
	"strings"
	"strconv"
	
	"github.com/oisee/minz/pkg/ir"
)

// MIRInterpreter executes MIR code at compile time for metaprogramming
type MIRInterpreter struct {
	registers  map[ir.Register]int64     // Virtual register values
	memory     map[int64]byte            // Simulated memory space
	stack      []int64                   // Execution stack
	callStack  []CallFrame               // Function call stack
	pc         int                       // Program counter
	flags      ProcessorFlags            // Z80-like flags
	functions  map[string]*ir.Function   // Available functions
	labels     map[string]int            // Label to instruction mapping
	
	// String management for metaprogramming
	strings    map[int64]string          // String heap
	nextStringID int64                   // String allocation counter
	
	// Metaprogramming output
	output     strings.Builder           // Generated MinZ code
	symbolGen  int                       // Symbol generator counter
	
	// Execution limits
	maxInstructions int64                // Prevent infinite loops
	instructionCount int64               // Current instruction count
}

// ProcessorFlags simulates Z80 flags for conditional operations
type ProcessorFlags struct {
	Zero    bool
	Carry   bool  
	Sign    bool
	Parity  bool
}

// CallFrame represents a function call context
type CallFrame struct {
	function    *ir.Function
	returnPC    int
	localRegs   map[ir.Register]int64
}

// NewMIRInterpreter creates a new MIR interpreter instance
func NewMIRInterpreter() *MIRInterpreter {
	return &MIRInterpreter{
		registers:       make(map[ir.Register]int64),
		memory:          make(map[int64]byte),
		stack:           make([]int64, 0, 256),
		callStack:       make([]CallFrame, 0, 32),
		functions:       make(map[string]*ir.Function),
		labels:          make(map[string]int),
		strings:         make(map[int64]string),
		nextStringID:    1000, // Start string IDs at 1000
		maxInstructions: 10000, // Reasonable limit for metaprogramming
	}
}

// Execute runs a MIR function with given arguments
func (interp *MIRInterpreter) Execute(function *ir.Function, args []int64) (int64, error) {
	// Reset interpreter state
	interp.pc = 0
	interp.instructionCount = 0
	interp.output.Reset()
	
	// Set up function context
	err := interp.setupFunction(function, args)
	if err != nil {
		return 0, err
	}
	
	// Build label map
	interp.buildLabelMap(function)
	
	// Execute instructions
	for interp.pc < len(function.Instructions) {
		// Check execution limits
		interp.instructionCount++
		if interp.instructionCount > interp.maxInstructions {
			return 0, fmt.Errorf("execution limit exceeded (%d instructions)", interp.maxInstructions)
		}
		
		inst := &function.Instructions[interp.pc]
		err := interp.executeInstruction(inst)
		if err != nil {
			return 0, fmt.Errorf("execution error at PC=%d: %v", interp.pc, err)
		}
		
		interp.pc++
	}
	
	// Return result (convention: return value in RegRet)
	return interp.registers[ir.RegRet], nil
}

// setupFunction initializes the function execution context
func (interp *MIRInterpreter) setupFunction(function *ir.Function, args []int64) error {
	// Clear registers
	for k := range interp.registers {
		delete(interp.registers, k)
	}
	
	// Set up parameters
	if len(args) > len(function.Params) {
		return fmt.Errorf("too many arguments: got %d, expected %d", len(args), len(function.Params))
	}
	
	// Load arguments into parameter registers
	for i, arg := range args {
		paramReg := ir.Register(i + 1) // Params start at register 1
		interp.registers[paramReg] = arg
	}
	
	return nil
}

// buildLabelMap creates mapping from labels to instruction indices
func (interp *MIRInterpreter) buildLabelMap(function *ir.Function) {
	// Clear existing labels
	for k := range interp.labels {
		delete(interp.labels, k)
	}
	
	// Map labels to instruction indices
	for i, inst := range function.Instructions {
		if inst.Op == ir.OpLabel && inst.Label != "" {
			interp.labels[inst.Label] = i
		}
	}
}

// executeInstruction executes a single MIR instruction
func (interp *MIRInterpreter) executeInstruction(inst *ir.Instruction) error {
	switch inst.Op {
	// Control flow
	case ir.OpNop:
		// Do nothing
		
	case ir.OpLabel:
		// Labels are handled by buildLabelMap
		
	case ir.OpJump:
		return interp.executeJump(inst.Label)
		
	case ir.OpJumpIf:
		if interp.flags.Zero {
			return interp.executeJump(inst.Label)
		}
		
	case ir.OpJumpIfNot:
		if !interp.flags.Zero {
			return interp.executeJump(inst.Label)
		}
		
	case ir.OpCall:
		return interp.executeCall(inst)
		
	case ir.OpReturn:
		val := interp.registers[inst.Src1]
		interp.registers[ir.RegRet] = val
		// For now, simple return - full call stack later
		interp.pc = len(interp.functions["main"].Instructions) // Force exit
		
	// Data movement
	case ir.OpLoadConst:
		interp.registers[inst.Dest] = inst.Imm
		
	case ir.OpMove:
		interp.registers[inst.Dest] = interp.registers[inst.Src1]
		
	// Arithmetic
	case ir.OpAdd:
		val1 := interp.registers[inst.Src1]
		val2 := interp.registers[inst.Src2]
		result := val1 + val2
		interp.registers[inst.Dest] = result
		interp.updateFlags(result)
		
	case ir.OpSub:
		val1 := interp.registers[inst.Src1]
		val2 := interp.registers[inst.Src2]
		result := val1 - val2
		interp.registers[inst.Dest] = result
		interp.updateFlags(result)
		
	case ir.OpMul:
		val1 := interp.registers[inst.Src1]
		val2 := interp.registers[inst.Src2]
		result := val1 * val2
		interp.registers[inst.Dest] = result
		interp.updateFlags(result)
		
	case ir.OpDiv:
		val1 := interp.registers[inst.Src1]
		val2 := interp.registers[inst.Src2]
		if val2 == 0 {
			return fmt.Errorf("division by zero")
		}
		result := val1 / val2
		interp.registers[inst.Dest] = result
		interp.updateFlags(result)
		
	// Comparison
	case ir.OpCmp:
		val1 := interp.registers[inst.Src1]
		val2 := interp.registers[inst.Src2]
		result := val1 - val2
		interp.updateFlags(result)
		
	default:
		return fmt.Errorf("unimplemented opcode: %v", inst.Op)
	}
	
	return nil
}

// executeJump performs a jump to a label
func (interp *MIRInterpreter) executeJump(label string) error {
	if label == "" {
		return fmt.Errorf("empty jump label")
	}
	
	targetPC, exists := interp.labels[label]
	if !exists {
		return fmt.Errorf("undefined label: %s", label)
	}
	
	interp.pc = targetPC - 1 // -1 because pc will be incremented
	return nil
}

// executeCall handles function calls (built-ins for now)
func (interp *MIRInterpreter) executeCall(inst *ir.Instruction) error {
	switch inst.Symbol {
	case "string_concat":
		return interp.builtinStringConcat(inst)
	case "string_format":
		return interp.builtinStringFormat(inst)
	case "to_string":
		return interp.builtinToString(inst)
	case "print_code":
		return interp.builtinPrintCode(inst)
	default:
		return fmt.Errorf("unknown function: %s", inst.Symbol)
	}
}

// updateFlags updates processor flags based on result
func (interp *MIRInterpreter) updateFlags(result int64) {
	interp.flags.Zero = (result == 0)
	interp.flags.Sign = (result < 0)
	interp.flags.Carry = false // Simplified for now
	interp.flags.Parity = false // Simplified for now
}

// String management functions

// storeString stores a string and returns its ID
func (interp *MIRInterpreter) storeString(s string) int64 {
	id := interp.nextStringID
	interp.nextStringID++
	interp.strings[id] = s
	return id
}

// getString retrieves a string by ID
func (interp *MIRInterpreter) getString(id int64) string {
	if str, exists := interp.strings[id]; exists {
		return str
	}
	return ""
}

// Built-in functions for metaprogramming

// builtinStringConcat concatenates two strings
func (interp *MIRInterpreter) builtinStringConcat(inst *ir.Instruction) error {
	str1ID := interp.registers[inst.Args[0]]
	str2ID := interp.registers[inst.Args[1]]
	
	str1 := interp.getString(str1ID)
	str2 := interp.getString(str2ID)
	
	result := str1 + str2
	resultID := interp.storeString(result)
	
	interp.registers[inst.Dest] = resultID
	return nil
}

// builtinStringFormat formats a string with arguments
func (interp *MIRInterpreter) builtinStringFormat(inst *ir.Instruction) error {
	formatID := interp.registers[inst.Args[0]]
	formatStr := interp.getString(formatID)
	
	// Simple template substitution - replace {0}, {1}, etc.
	result := formatStr
	for i := 1; i < len(inst.Args); i++ {
		placeholder := fmt.Sprintf("{%d}", i-1)
		value := fmt.Sprintf("%d", interp.registers[inst.Args[i]])
		result = strings.ReplaceAll(result, placeholder, value)
	}
	
	resultID := interp.storeString(result)
	interp.registers[inst.Dest] = resultID
	return nil
}

// builtinToString converts a number to string
func (interp *MIRInterpreter) builtinToString(inst *ir.Instruction) error {
	value := interp.registers[inst.Args[0]]
	str := strconv.FormatInt(value, 10)
	resultID := interp.storeString(str)
	interp.registers[inst.Dest] = resultID
	return nil
}

// builtinPrintCode adds code to the output
func (interp *MIRInterpreter) builtinPrintCode(inst *ir.Instruction) error {
	codeID := interp.registers[inst.Args[0]]
	code := interp.getString(codeID)
	interp.output.WriteString(code)
	return nil
}

// GetGeneratedCode returns the generated MinZ code
func (interp *MIRInterpreter) GetGeneratedCode() string {
	return interp.output.String()
}

// ExecuteMinzCode executes MinZ code string (simplified version)
func (interp *MIRInterpreter) ExecuteMinzCode(code string, args []int64) (string, error) {
	// For now, return the code as-is - full parsing integration comes later
	// This is a placeholder for the complete implementation
	
	// Simple template substitution
	result := code
	for i, arg := range args {
		placeholder := fmt.Sprintf("{%d}", i)
		value := fmt.Sprintf("%d", arg)
		result = strings.ReplaceAll(result, placeholder, value)
	}
	
	return result, nil
}