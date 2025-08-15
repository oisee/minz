// Package mirvm implements a virtual machine for executing MinZ MIR code
package mirvm

import (
	"fmt"
	"io"

	"github.com/minz/minzc/pkg/ir"
)

// Config holds VM configuration
type Config struct {
	MemorySize   int
	StackSize    int
	Debug        bool
	Trace        bool
	MaxSteps     int
	Verbose      bool
	OutputStream io.Writer
	Breakpoints  map[string][]int // function -> instruction indices
}

// Statistics tracks execution statistics
type Statistics struct {
	InstructionsExecuted int
	FunctionsCalled      int
	MaxStackDepth        int
	MemoryUsed           int
}

// VM is the MIR virtual machine
type VM struct {
	config Config
	stats  Statistics
	
	// Memory and registers
	memory    []byte
	registers [256]int64 // Virtual registers
	pc        int         // Program counter (instruction index)
	sp        int         // Stack pointer
	fp        int         // Frame pointer
	
	// Module and execution state
	module        *ir.Module
	currentFunc   *ir.Function
	funcIndex     map[string]*ir.Function
	callStack     []CallFrame
	
	// Debug state
	breakHit      bool
	stepMode      bool
	instructionCount int
}

// CallFrame represents a function call frame
type CallFrame struct {
	Function     *ir.Function
	ReturnPC     int
	FramePointer int
	LocalBase    int // Base register for locals
}

// New creates a new VM instance
func New(config Config) *VM {
	return &VM{
		config:    config,
		memory:    make([]byte, config.MemorySize),
		funcIndex: make(map[string]*ir.Function),
		sp:        config.StackSize, // Stack grows down
		fp:        config.StackSize,
	}
}

// LoadModule loads a MIR module into the VM
func (vm *VM) LoadModule(module *ir.Module) error {
	vm.module = module
	
	// Build function index
	for _, fn := range module.Functions {
		vm.funcIndex[fn.Name] = fn
	}
	
	// Find main function
	mainFunc, ok := vm.funcIndex["main"]
	if !ok {
		return fmt.Errorf("no main function found")
	}
	
	vm.currentFunc = mainFunc
	vm.pc = 0
	
	// Initialize global variables
	for i := range module.Globals {
		if err := vm.initGlobal(&module.Globals[i]); err != nil {
			return fmt.Errorf("failed to initialize global %s: %v", module.Globals[i].Name, err)
		}
	}
	
	return nil
}

// Run executes the loaded program
func (vm *VM) Run() (int, error) {
	if vm.currentFunc == nil {
		return 1, fmt.Errorf("no function loaded")
	}
	
	// Main execution loop
	for vm.instructionCount < vm.config.MaxSteps {
		// Check breakpoints
		if vm.checkBreakpoint() {
			if err := vm.handleBreakpoint(); err != nil {
				return 1, err
			}
		}
		
		// Execute next instruction
		done, err := vm.executeInstruction()
		if err != nil {
			return 1, fmt.Errorf("runtime error at %s:%d: %v", 
				vm.currentFunc.Name, vm.pc, err)
		}
		
		if done {
			// Program completed successfully
			return 0, nil
		}
		
		vm.instructionCount++
		vm.stats.InstructionsExecuted++
		
		// Update max stack depth
		stackDepth := (vm.config.StackSize - vm.sp) / 8
		if stackDepth > vm.stats.MaxStackDepth {
			vm.stats.MaxStackDepth = stackDepth
		}
	}
	
	return 1, fmt.Errorf("execution limit exceeded (%d instructions)", vm.config.MaxSteps)
}

// executeInstruction executes a single MIR instruction
func (vm *VM) executeInstruction() (bool, error) {
	if vm.pc >= len(vm.currentFunc.Instructions) {
		// End of function
		if len(vm.callStack) == 0 {
			// Main function ended - program complete
			return true, nil
		}
		
		// Return from function
		return false, vm.returnFromFunction()
	}
	
	inst := vm.currentFunc.Instructions[vm.pc]
	
	if vm.config.Trace {
		vm.traceInstruction(inst)
	}
	
	// Execute based on opcode
	switch inst.Op {
	case ir.OpNop:
		// No operation
		
	case ir.OpLoadImm:
		vm.registers[inst.Dest] = int64(inst.Value)
		
	case ir.OpLoadReg:
		vm.registers[inst.Dest] = vm.registers[inst.Src1]
		
	case ir.OpLoadMem:
		addr := vm.registers[inst.Src1]
		if inst.Offset != 0 {
			addr += int64(inst.Offset)
		}
		value := vm.readMemory(int(addr), inst.Size)
		vm.registers[inst.Dest] = value
		
	case ir.OpStoreMem:
		addr := vm.registers[inst.Dest]
		if inst.Offset != 0 {
			addr += int64(inst.Offset)
		}
		value := vm.registers[inst.Src1]
		vm.writeMemory(int(addr), value, inst.Size)
		
	case ir.OpAdd:
		vm.registers[inst.Dest] = vm.registers[inst.Src1] + vm.registers[inst.Src2]
		
	case ir.OpSub:
		vm.registers[inst.Dest] = vm.registers[inst.Src1] - vm.registers[inst.Src2]
		
	case ir.OpMul:
		vm.registers[inst.Dest] = vm.registers[inst.Src1] * vm.registers[inst.Src2]
		
	case ir.OpDiv:
		if vm.registers[inst.Src2] == 0 {
			return false, fmt.Errorf("division by zero")
		}
		vm.registers[inst.Dest] = vm.registers[inst.Src1] / vm.registers[inst.Src2]
		
	case ir.OpMod:
		if vm.registers[inst.Src2] == 0 {
			return false, fmt.Errorf("modulo by zero")
		}
		vm.registers[inst.Dest] = vm.registers[inst.Src1] % vm.registers[inst.Src2]
		
	case ir.OpAnd:
		vm.registers[inst.Dest] = vm.registers[inst.Src1] & vm.registers[inst.Src2]
		
	case ir.OpOr:
		vm.registers[inst.Dest] = vm.registers[inst.Src1] | vm.registers[inst.Src2]
		
	case ir.OpXor:
		vm.registers[inst.Dest] = vm.registers[inst.Src1] ^ vm.registers[inst.Src2]
		
	case ir.OpShl:
		vm.registers[inst.Dest] = vm.registers[inst.Src1] << uint(vm.registers[inst.Src2])
		
	case ir.OpShr:
		vm.registers[inst.Dest] = vm.registers[inst.Src1] >> uint(vm.registers[inst.Src2])
		
	case ir.OpNot:
		vm.registers[inst.Dest] = ^vm.registers[inst.Src1]
		
	case ir.OpNeg:
		vm.registers[inst.Dest] = -vm.registers[inst.Src1]
		
	case ir.OpCmp:
		// Set flags based on comparison
		a := vm.registers[inst.Src1]
		b := vm.registers[inst.Src2]
		if a == b {
			vm.registers[255] = 0 // Equal
		} else if a < b {
			vm.registers[255] = -1 // Less than
		} else {
			vm.registers[255] = 1 // Greater than
		}
		
	case ir.OpJmp:
		vm.pc = inst.Target
		return false, nil
		
	case ir.OpJmpIf:
		if vm.registers[inst.Src1] != 0 {
			vm.pc = inst.Target
			return false, nil
		}
		
	case ir.OpJmpIfNot:
		if vm.registers[inst.Src1] == 0 {
			vm.pc = inst.Target
			return false, nil
		}
		
	case ir.OpCall:
		return false, vm.callFunction(inst.FuncName)
		
	case ir.OpReturn:
		if len(vm.callStack) == 0 {
			// Returning from main
			return true, nil
		}
		return false, vm.returnFromFunction()
		
	case ir.OpPush:
		vm.sp -= 8
		vm.writeMemory(vm.sp, vm.registers[inst.Src1], 8)
		
	case ir.OpPop:
		value := vm.readMemory(vm.sp, 8)
		vm.registers[inst.Dest] = value
		vm.sp += 8
		
	case ir.OpPrint:
		// Built-in print function
		value := vm.registers[inst.Src1]
		fmt.Fprintf(vm.config.OutputStream, "%d", value)
		
	case ir.OpPrintChar:
		// Print as character
		value := vm.registers[inst.Src1]
		fmt.Fprintf(vm.config.OutputStream, "%c", byte(value))
		
	case ir.OpHalt:
		// Stop execution
		return true, nil
		
	default:
		return false, fmt.Errorf("unknown opcode: %v", inst.Op)
	}
	
	vm.pc++
	return false, nil
}

// callFunction calls a function
func (vm *VM) callFunction(name string) error {
	fn, ok := vm.funcIndex[name]
	if !ok {
		// Check for built-in functions
		if vm.handleBuiltin(name) {
			return nil
		}
		return fmt.Errorf("undefined function: %s", name)
	}
	
	// Save current state
	frame := CallFrame{
		Function:     vm.currentFunc,
		ReturnPC:     vm.pc + 1,
		FramePointer: vm.fp,
		LocalBase:    0, // TODO: Calculate local base
	}
	vm.callStack = append(vm.callStack, frame)
	
	// Set up new function
	vm.currentFunc = fn
	vm.pc = 0
	vm.fp = vm.sp
	
	vm.stats.FunctionsCalled++
	
	return nil
}

// returnFromFunction returns from the current function
func (vm *VM) returnFromFunction() error {
	if len(vm.callStack) == 0 {
		return fmt.Errorf("call stack underflow")
	}
	
	// Restore previous frame
	frame := vm.callStack[len(vm.callStack)-1]
	vm.callStack = vm.callStack[:len(vm.callStack)-1]
	
	vm.currentFunc = frame.Function
	vm.pc = frame.ReturnPC
	vm.fp = frame.FramePointer
	
	return nil
}

// handleBuiltin handles built-in functions
func (vm *VM) handleBuiltin(name string) bool {
	switch name {
	case "print_u8":
		value := vm.registers[0] // Assuming first argument in r0
		fmt.Fprintf(vm.config.OutputStream, "%d", byte(value))
		return true
		
	case "print_u16":
		value := vm.registers[0]
		fmt.Fprintf(vm.config.OutputStream, "%d", uint16(value))
		return true
		
	case "print_char":
		value := vm.registers[0]
		fmt.Fprintf(vm.config.OutputStream, "%c", byte(value))
		return true
		
	case "memcpy":
		// dst in r0, src in r1, size in r2
		dst := int(vm.registers[0])
		src := int(vm.registers[1])
		size := int(vm.registers[2])
		copy(vm.memory[dst:dst+size], vm.memory[src:src+size])
		return true
		
	case "memset":
		// dst in r0, value in r1, size in r2
		dst := int(vm.registers[0])
		value := byte(vm.registers[1])
		size := int(vm.registers[2])
		for i := 0; i < size; i++ {
			vm.memory[dst+i] = value
		}
		return true
	}
	
	return false
}

// Memory access functions
func (vm *VM) readMemory(addr int, size int) int64 {
	if addr < 0 || addr+size > len(vm.memory) {
		// Memory access error - return 0
		return 0
	}
	
	var value int64
	for i := 0; i < size; i++ {
		value |= int64(vm.memory[addr+i]) << (i * 8)
	}
	return value
}

func (vm *VM) writeMemory(addr int, value int64, size int) {
	if addr < 0 || addr+size > len(vm.memory) {
		// Memory access error - ignore
		return
	}
	
	for i := 0; i < size; i++ {
		vm.memory[addr+i] = byte(value >> (i * 8))
	}
}

// initGlobal initializes a global variable
func (vm *VM) initGlobal(global *ir.Global) error {
	// Allocate space for global
	// For simplicity, we'll use fixed addresses starting at 0x1000
	// Note: addr is calculated but not used yet - this is a placeholder
	// addr := 0x1000 + len(vm.module.Globals)*8
	
	// Store initial value if any
	if global.Init != nil {
		// TODO: Handle initialization
	}
	
	return nil
}

// Debug functions
func (vm *VM) checkBreakpoint() bool {
	if vm.config.Breakpoints == nil {
		return false
	}
	
	breakpoints, ok := vm.config.Breakpoints[vm.currentFunc.Name]
	if !ok {
		return false
	}
	
	for _, bp := range breakpoints {
		if bp == vm.pc {
			return true
		}
	}
	
	return false
}

func (vm *VM) handleBreakpoint() error {
	fmt.Fprintf(vm.config.OutputStream, "\nBreakpoint hit at %s:%d\n", 
		vm.currentFunc.Name, vm.pc)
	
	// Print current instruction
	if vm.pc < len(vm.currentFunc.Instructions) {
		inst := vm.currentFunc.Instructions[vm.pc]
		fmt.Fprintf(vm.config.OutputStream, "  Next: %s\n", formatInstruction(inst))
	}
	
	// TODO: Interactive debugger
	vm.stepMode = true
	
	return nil
}

func (vm *VM) traceInstruction(inst ir.Instruction) {
	fmt.Fprintf(vm.config.OutputStream, "[%s:%d] %s\n", 
		vm.currentFunc.Name, vm.pc, formatInstruction(inst))
}

func formatInstruction(inst ir.Instruction) string {
	switch inst.Op {
	case ir.OpLoadImm:
		return fmt.Sprintf("r%d = %d", inst.Dest, inst.Value)
	case ir.OpLoadReg:
		return fmt.Sprintf("r%d = r%d", inst.Dest, inst.Src1)
	case ir.OpAdd:
		return fmt.Sprintf("r%d = r%d + r%d", inst.Dest, inst.Src1, inst.Src2)
	case ir.OpSub:
		return fmt.Sprintf("r%d = r%d - r%d", inst.Dest, inst.Src1, inst.Src2)
	case ir.OpCall:
		return fmt.Sprintf("call %s", inst.FuncName)
	case ir.OpReturn:
		return "return"
	case ir.OpJmp:
		return fmt.Sprintf("jmp %d", inst.Target)
	default:
		return inst.Op.String()
	}
}

// GetMemoryDump returns a dump of VM memory
func (vm *VM) GetMemoryDump() []byte {
	dump := make([]byte, len(vm.memory))
	copy(dump, vm.memory)
	return dump
}

// GetStatistics returns execution statistics
func (vm *VM) GetStatistics() Statistics {
	vm.stats.MemoryUsed = vm.config.StackSize - vm.sp
	return vm.stats
}