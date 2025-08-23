package mir

import (
	"fmt"
	"github.com/minz/minzc/pkg/ir"
)

// Interpreter executes MIR code at compile time
type Interpreter struct {
	// Memory for compile-time execution
	memory    map[uint16]byte
	registers map[ir.Register]int64
	
	// Stack for function calls
	callStack []CallFrame
	
	// Program counter
	pc int
	
	// Flags
	zeroFlag  bool
	carryFlag bool
	signFlag  bool
	
	// Functions available for execution
	functions map[string]*ir.Function
	
	// Current function being executed
	currentFunc *ir.Function
	
	// Maximum iterations to prevent infinite loops
	maxIterations int
	iterCount     int
	
	// Output accumulator for @emit
	output []string
}

// CallFrame represents a function call frame
type CallFrame struct {
	Function  *ir.Function
	PC        int
	Registers map[ir.Register]int64
	ReturnReg ir.Register
}

// NewInterpreter creates a new MIR interpreter
func NewInterpreter() *Interpreter {
	return &Interpreter{
		memory:        make(map[uint16]byte),
		registers:     make(map[ir.Register]int64),
		callStack:     make([]CallFrame, 0),
		functions:     make(map[string]*ir.Function),
		maxIterations: 100000, // Prevent infinite loops at compile time
		output:        make([]string, 0),
	}
}

// AddFunction registers a function for compile-time execution
func (i *Interpreter) AddFunction(fn *ir.Function) {
	i.functions[fn.Name] = fn
}

// Execute runs a function at compile time and returns the result
func (i *Interpreter) Execute(funcName string, args []int64) (int64, error) {
	fn, exists := i.functions[funcName]
	if !exists {
		return 0, fmt.Errorf("function %s not found", funcName)
	}
	
	// Initialize execution
	i.currentFunc = fn
	i.pc = 0
	i.iterCount = 0
	i.registers = make(map[ir.Register]int64)
	
	// Load arguments into parameter registers
	for idx, arg := range args {
		if idx < len(fn.Params) {
			i.registers[fn.Params[idx].Reg] = arg
		}
	}
	
	// Execute instructions
	for i.pc < len(fn.Instructions) {
		if i.iterCount >= i.maxIterations {
			return 0, fmt.Errorf("compile-time execution exceeded maximum iterations")
		}
		i.iterCount++
		
		inst := &fn.Instructions[i.pc]
		if err := i.executeInstruction(inst); err != nil {
			return 0, err
		}
		
		// Check for return
		if inst.Op == ir.OpReturn {
			if inst.Src1 != 0 {
				return i.registers[inst.Src1], nil
			}
			return 0, nil
		}
		
		i.pc++
	}
	
	return 0, nil
}

// executeInstruction executes a single MIR instruction
func (i *Interpreter) executeInstruction(inst *ir.Instruction) error {
	switch inst.Op {
	case ir.OpNop:
		// No operation
		
	case ir.OpLoadConst:
		i.registers[inst.Dest] = inst.Imm
		
	case ir.OpMove:
		i.registers[inst.Dest] = i.registers[inst.Src1]
		
	case ir.OpAdd:
		result := i.registers[inst.Src1] + i.registers[inst.Src2]
		i.registers[inst.Dest] = result
		i.updateFlags(result)
		
	case ir.OpSub:
		result := i.registers[inst.Src1] - i.registers[inst.Src2]
		i.registers[inst.Dest] = result
		i.updateFlags(result)
		
	case ir.OpMul:
		result := i.registers[inst.Src1] * i.registers[inst.Src2]
		i.registers[inst.Dest] = result
		i.updateFlags(result)
		
	case ir.OpDiv:
		if i.registers[inst.Src2] == 0 {
			return fmt.Errorf("division by zero at compile time")
		}
		result := i.registers[inst.Src1] / i.registers[inst.Src2]
		i.registers[inst.Dest] = result
		i.updateFlags(result)
		
	case ir.OpMod:
		if i.registers[inst.Src2] == 0 {
			return fmt.Errorf("modulo by zero at compile time")
		}
		result := i.registers[inst.Src1] % i.registers[inst.Src2]
		i.registers[inst.Dest] = result
		i.updateFlags(result)
		
	case ir.OpAnd:
		result := i.registers[inst.Src1] & i.registers[inst.Src2]
		i.registers[inst.Dest] = result
		i.updateFlags(result)
		
	case ir.OpOr:
		result := i.registers[inst.Src1] | i.registers[inst.Src2]
		i.registers[inst.Dest] = result
		i.updateFlags(result)
		
	case ir.OpXor:
		result := i.registers[inst.Src1] ^ i.registers[inst.Src2]
		i.registers[inst.Dest] = result
		i.updateFlags(result)
		
	case ir.OpNot:
		result := ^i.registers[inst.Src1]
		i.registers[inst.Dest] = result
		i.updateFlags(result)
		
	case ir.OpShl:
		result := i.registers[inst.Src1] << uint(i.registers[inst.Src2])
		i.registers[inst.Dest] = result
		i.updateFlags(result)
		
	case ir.OpShr:
		result := i.registers[inst.Src1] >> uint(i.registers[inst.Src2])
		i.registers[inst.Dest] = result
		i.updateFlags(result)
		
	case ir.OpEq:
		if i.registers[inst.Src1] == i.registers[inst.Src2] {
			i.registers[inst.Dest] = 1
		} else {
			i.registers[inst.Dest] = 0
		}
		
	case ir.OpNe:
		if i.registers[inst.Src1] != i.registers[inst.Src2] {
			i.registers[inst.Dest] = 1
		} else {
			i.registers[inst.Dest] = 0
		}
		
	case ir.OpLt:
		if i.registers[inst.Src1] < i.registers[inst.Src2] {
			i.registers[inst.Dest] = 1
		} else {
			i.registers[inst.Dest] = 0
		}
		
	case ir.OpLe:
		if i.registers[inst.Src1] <= i.registers[inst.Src2] {
			i.registers[inst.Dest] = 1
		} else {
			i.registers[inst.Dest] = 0
		}
		
	case ir.OpGt:
		if i.registers[inst.Src1] > i.registers[inst.Src2] {
			i.registers[inst.Dest] = 1
		} else {
			i.registers[inst.Dest] = 0
		}
		
	case ir.OpGe:
		if i.registers[inst.Src1] >= i.registers[inst.Src2] {
			i.registers[inst.Dest] = 1
		} else {
			i.registers[inst.Dest] = 0
		}
		
	case ir.OpJump:
		// Find label
		targetPC := i.findLabel(inst.Symbol)
		if targetPC < 0 {
			return fmt.Errorf("label %s not found", inst.Symbol)
		}
		i.pc = targetPC - 1 // -1 because pc++ happens after
		
	case ir.OpJumpIf:
		if i.registers[inst.Src1] != 0 {
			targetPC := i.findLabel(inst.Symbol)
			if targetPC < 0 {
				return fmt.Errorf("label %s not found", inst.Symbol)
			}
			i.pc = targetPC - 1
		}
		
	case ir.OpJumpIfNot:
		if i.registers[inst.Src1] == 0 {
			targetPC := i.findLabel(inst.Symbol)
			if targetPC < 0 {
				return fmt.Errorf("label %s not found", inst.Symbol)
			}
			i.pc = targetPC - 1
		}
		
	case ir.OpCall:
		// Save current state
		frame := CallFrame{
			Function:  i.currentFunc,
			PC:        i.pc,
			Registers: make(map[ir.Register]int64),
			ReturnReg: inst.Dest,
		}
		// Copy registers
		for r, v := range i.registers {
			frame.Registers[r] = v
		}
		i.callStack = append(i.callStack, frame)
		
		// Call function
		fn, exists := i.functions[inst.Symbol]
		if !exists {
			// Built-in function or external - can't execute at compile time
			return fmt.Errorf("cannot execute external function %s at compile time", inst.Symbol)
		}
		
		i.currentFunc = fn
		i.pc = -1 // Will be incremented to 0
		
	case ir.OpReturn:
		if len(i.callStack) > 0 {
			// Restore previous frame
			frame := i.callStack[len(i.callStack)-1]
			i.callStack = i.callStack[:len(i.callStack)-1]
			
			// Save return value if any
			var returnValue int64
			if inst.Src1 != 0 {
				returnValue = i.registers[inst.Src1]
			}
			
			// Restore state
			i.currentFunc = frame.Function
			i.pc = frame.PC
			i.registers = frame.Registers
			
			// Store return value
			if frame.ReturnReg != 0 {
				i.registers[frame.ReturnReg] = returnValue
			}
		}
		
	case ir.OpLabel:
		// Labels are no-ops during execution
		
	case ir.OpEmit:
		// Special operation for @minz blocks - accumulate output
		if inst.Symbol != "" {
			i.output = append(i.output, inst.Symbol)
		}
		
	default:
		// Unsupported operation for compile-time execution
		return fmt.Errorf("unsupported operation %v for compile-time execution", inst.Op)
	}
	
	return nil
}

// updateFlags updates CPU flags based on result
func (i *Interpreter) updateFlags(result int64) {
	i.zeroFlag = (result == 0)
	i.signFlag = (result < 0)
	// Carry flag would need more context
}

// findLabel finds the instruction index for a label
func (i *Interpreter) findLabel(label string) int {
	for idx, inst := range i.currentFunc.Instructions {
		if inst.Op == ir.OpLabel && inst.Symbol == label {
			return idx
		}
	}
	return -1
}

// GetOutput returns accumulated output from @emit operations
func (i *Interpreter) GetOutput() []string {
	return i.output
}

// Reset clears the interpreter state
func (i *Interpreter) Reset() {
	i.memory = make(map[uint16]byte)
	i.registers = make(map[ir.Register]int64)
	i.callStack = make([]CallFrame, 0)
	i.pc = 0
	i.iterCount = 0
	i.output = make([]string, 0)
}

// ExecuteMinzBlock executes a @minz[[[]]] block and returns generated code
func (i *Interpreter) ExecuteMinzBlock(code string, args []interface{}) (string, error) {
	// This would parse the MinZ code into MIR and execute it
	// For now, return a placeholder
	// TODO: Implement MinZ -> MIR compilation for compile-time blocks
	
	i.Reset()
	
	// The actual implementation would:
	// 1. Parse the MinZ code
	// 2. Convert to MIR
	// 3. Execute the MIR
	// 4. Return accumulated @emit output
	
	return "", fmt.Errorf("@minz block execution not yet implemented")
}

// ExecuteCTIE executes a function at compile time for CTIE optimization
func (i *Interpreter) ExecuteCTIE(fn *ir.Function, args []int64) (int64, bool, error) {
	// Check if all arguments are constants
	for range args {
		// In real implementation, would check if arg is compile-time constant
		// For now, assume all integer literals are constants
	}
	
	// Try to execute the function
	result, err := i.Execute(fn.Name, args)
	if err != nil {
		// Can't execute at compile time - fall back to runtime
		return 0, false, nil
	}
	
	// Successfully executed at compile time
	return result, true, nil
}