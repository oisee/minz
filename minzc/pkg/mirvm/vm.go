// Package mirvm implements a virtual machine for executing MinZ MIR code
package mirvm

import (
	"fmt"
	"io"
	"strings"

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
	Platform     Platform          // Hardware platform (nil = headless)
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

	// Metaprogramming support
	emittedCode   []string // Captured @emit output
	stringPool    map[int64]string // String literals

	// Platform abstraction (v0.18.0)
	platform      Platform
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
	vm := &VM{
		config:      config,
		memory:      make([]byte, config.MemorySize),
		funcIndex:   make(map[string]*ir.Function),
		sp:          config.StackSize, // Stack grows down
		fp:          config.StackSize,
		emittedCode: make([]string, 0),
		stringPool:  make(map[int64]string),
	}

	// Initialize platform (default to headless if not provided)
	if config.Platform != nil {
		vm.platform = config.Platform
	} else {
		vm.platform = NewHeadlessPlatform()
	}

	return vm
}

// LoadModule loads a MIR module into the VM
func (vm *VM) LoadModule(module *ir.Module) error {
	vm.module = module

	// Build function index and resolve labels for each function
	for _, fn := range module.Functions {
		vm.funcIndex[fn.Name] = fn

		// Build label-to-index map for this function
		labels := make(map[string]int)
		for i, inst := range fn.Instructions {
			if inst.Op == ir.OpLabel && inst.Label != "" {
				labels[inst.Label] = i
			}
		}

		// Resolve jump targets from labels to instruction indices
		for i := range fn.Instructions {
			inst := &fn.Instructions[i]
			if inst.Label != "" && inst.Target == 0 {
				switch inst.Op {
				case ir.OpJump, ir.OpJumpIf, ir.OpJumpIfNot,
					ir.OpJmp, ir.OpJmpIf, ir.OpJmpIfNot, ir.OpDJNZ:
					if target, ok := labels[inst.Label]; ok {
						inst.Target = target
					}
				}
			}
		}
	}

	// Find main function (could be "main" or "module.main")
	mainFunc, ok := vm.funcIndex["main"]
	if !ok {
		// Try to find a function ending in ".main"
		for name, fn := range vm.funcIndex {
			if strings.HasSuffix(name, ".main") {
				mainFunc = fn
				ok = true
				break
			}
		}
	}
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

	case ir.OpLoadConst:
		// Same as LoadImm but uses Imm field instead of Value
		vm.registers[inst.Dest] = inst.Imm

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

	case ir.OpLoadVar:
		// Load from named variable - look up variable's register in current function's Locals
		varReg := vm.findVarRegister(inst.Symbol)
		if varReg < 0 {
			return false, fmt.Errorf("undefined variable: %s", inst.Symbol)
		}
		vm.registers[inst.Dest] = vm.registers[varReg]

	case ir.OpStoreVar:
		// Store to named variable - look up variable's register in current function's Locals
		varReg := vm.findVarRegister(inst.Symbol)
		if varReg < 0 {
			return false, fmt.Errorf("undefined variable: %s", inst.Symbol)
		}
		vm.registers[varReg] = vm.registers[inst.Src1]

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

	case ir.OpInc:
		vm.registers[inst.Dest] = vm.registers[inst.Src1] + 1

	case ir.OpDec:
		vm.registers[inst.Dest] = vm.registers[inst.Src1] - 1

	case ir.OpDJNZ:
		// Decrement and jump if not zero (Z80-style loop optimization)
		vm.registers[inst.Src1]--
		if vm.registers[inst.Src1] != 0 {
			vm.pc = inst.Target
			return false, nil
		}

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

	case ir.OpLt:
		if vm.registers[inst.Src1] < vm.registers[inst.Src2] {
			vm.registers[inst.Dest] = 1
		} else {
			vm.registers[inst.Dest] = 0
		}

	case ir.OpGt:
		if vm.registers[inst.Src1] > vm.registers[inst.Src2] {
			vm.registers[inst.Dest] = 1
		} else {
			vm.registers[inst.Dest] = 0
		}

	case ir.OpEq:
		if vm.registers[inst.Src1] == vm.registers[inst.Src2] {
			vm.registers[inst.Dest] = 1
		} else {
			vm.registers[inst.Dest] = 0
		}

	case ir.OpNe:
		if vm.registers[inst.Src1] != vm.registers[inst.Src2] {
			vm.registers[inst.Dest] = 1
		} else {
			vm.registers[inst.Dest] = 0
		}

	case ir.OpLe:
		if vm.registers[inst.Src1] <= vm.registers[inst.Src2] {
			vm.registers[inst.Dest] = 1
		} else {
			vm.registers[inst.Dest] = 0
		}

	case ir.OpGe:
		if vm.registers[inst.Src1] >= vm.registers[inst.Src2] {
			vm.registers[inst.Dest] = 1
		} else {
			vm.registers[inst.Dest] = 0
		}

	case ir.OpLabel:
		// Labels are just markers, no operation needed
		// (Jump targets are resolved to instruction indices)

	case ir.OpJmp, ir.OpJump:
		vm.pc = inst.Target
		return false, nil

	case ir.OpJmpIf, ir.OpJumpIf:
		if vm.registers[inst.Src1] != 0 {
			vm.pc = inst.Target
			return false, nil
		}

	case ir.OpJmpIfNot, ir.OpJumpIfNot:
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
		
	case ir.OpEmit:
		// @emit instruction for metaprogramming
		return false, vm.handleEmit(inst)
		
	case ir.OpLoadString:
		// Load string literal into register
		vm.registers[inst.Dest] = int64(inst.StringID)
		if inst.StringValue != "" {
			vm.stringPool[int64(inst.StringID)] = inst.StringValue
		}

	// Platform I/O operations (v0.18.0)
	case ir.OpPortIn:
		// r0 = port_in(port) - read from I/O port
		port := uint16(inst.Imm)
		if inst.Src1 != 0 {
			port = uint16(vm.registers[inst.Src1])
		}
		vm.registers[inst.Dest] = int64(vm.platform.PortIn(port))

	case ir.OpPortOut:
		// port_out(port, r0) - write to I/O port
		port := uint16(inst.Imm)
		if inst.Src1 != 0 {
			port = uint16(vm.registers[inst.Src1])
		}
		value := byte(vm.registers[inst.Src2])
		vm.platform.PortOut(port, value)

	case ir.OpSyscall:
		// syscall(id, args...) - platform system call
		syscallID := int(inst.Imm)
		result, err := vm.handleSyscall(syscallID, inst)
		if err != nil {
			return false, err
		}
		vm.registers[inst.Dest] = result

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

// handleEmit processes @emit instructions for metaprogramming
func (vm *VM) handleEmit(inst ir.Instruction) error {
	// Get the string to emit
	var emitStr string
	
	if inst.StringValue != "" {
		// Direct string value
		emitStr = inst.StringValue
	} else if inst.Src1 != 0 {
		// String from register
		stringID := vm.registers[inst.Src1]
		if str, ok := vm.stringPool[stringID]; ok {
			emitStr = str
		} else {
			return fmt.Errorf("invalid string ID: %d", stringID)
		}
	} else {
		return fmt.Errorf("@emit requires either StringValue or Src1")
	}
	
	// Add to emitted code
	vm.emittedCode = append(vm.emittedCode, emitStr)
	
	if vm.config.Debug {
		fmt.Fprintf(vm.config.OutputStream, "DEBUG: @emit: %s\n", emitStr)
	}
	
	return nil
}

// GetEmittedCode returns all code emitted by @emit instructions
func (vm *VM) GetEmittedCode() []string {
	return vm.emittedCode
}

// ClearEmittedCode clears the emitted code buffer
func (vm *VM) ClearEmittedCode() {
	vm.emittedCode = make([]string, 0)
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

// handleSyscall handles platform system calls
func (vm *VM) handleSyscall(id int, inst ir.Instruction) (int64, error) {
	switch id {
	case 0: // exit(code)
		code := int(vm.registers[inst.Src1])
		vm.platform.Exit(code)
		return 0, nil

	case 1: // write_char(char)
		ch := byte(vm.registers[inst.Src1])
		vm.platform.WriteChar(ch)
		return 1, nil

	case 2: // read_char() -> char
		ch, ok := vm.platform.ReadChar()
		if !ok {
			return -1, nil // EOF
		}
		return int64(ch), nil

	case 3: // port_out(port, value)
		port := uint16(vm.registers[inst.Src1])
		value := byte(vm.registers[inst.Src2])
		vm.platform.PortOut(port, value)
		return 0, nil

	case 4: // port_in(port) -> value
		port := uint16(vm.registers[inst.Src1])
		value := vm.platform.PortIn(port)
		return int64(value), nil

	case 10: // set_pixel(x, y, color) - if display available
		if vm.platform.HasDisplay() {
			x := int(vm.registers[inst.Src1])
			y := int(vm.registers[inst.Src2])
			color := uint32(vm.registers[0]) // color in r0
			vm.platform.Display().SetPixel(x, y, color)
		}
		return 0, nil

	case 11: // get_pixel(x, y) -> color
		if vm.platform.HasDisplay() {
			x := int(vm.registers[inst.Src1])
			y := int(vm.registers[inst.Src2])
			color := vm.platform.Display().GetPixel(x, y)
			return int64(color), nil
		}
		return 0, nil

	case 12: // clear_screen(color)
		if vm.platform.HasDisplay() {
			color := uint32(vm.registers[inst.Src1])
			vm.platform.Display().Clear(color)
		}
		return 0, nil

	case 13: // get_display_width() -> width
		if vm.platform.HasDisplay() {
			return int64(vm.platform.Display().Width()), nil
		}
		return 0, nil

	case 14: // get_display_height() -> height
		if vm.platform.HasDisplay() {
			return int64(vm.platform.Display().Height()), nil
		}
		return 0, nil

	default:
		return 0, fmt.Errorf("unknown syscall: %d", id)
	}
}

// GetPlatform returns the current platform
func (vm *VM) GetPlatform() Platform {
	return vm.platform
}

// SetPlatform sets the platform (for runtime platform switching)
func (vm *VM) SetPlatform(p Platform) {
	vm.platform = p
}

// findVarRegister looks up a variable name in the current function's Locals
// and returns the register allocated for it, or -1 if not found
func (vm *VM) findVarRegister(name string) int {
	if vm.currentFunc == nil {
		return -1
	}
	for _, local := range vm.currentFunc.Locals {
		if local.Name == name {
			return int(local.Reg)
		}
	}
	// Also check parameters
	for _, param := range vm.currentFunc.Params {
		if param.Name == name {
			return int(param.Reg)
		}
	}
	return -1
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

// =============================================================================
// Debugging Support (v0.18.0 - DAP integration)
// =============================================================================

// exited tracks if the program has exited
var vmExited bool
var vmExitCode int

// Step executes a single instruction (for debugging)
func (vm *VM) Step() error {
	if vm.currentFunc == nil {
		return fmt.Errorf("no function loaded")
	}

	// Check if we're past the end of the current function
	if vm.pc >= len(vm.currentFunc.Instructions) {
		// Check if we need to return from a function
		if len(vm.callStack) > 0 {
			if err := vm.returnFromFunction(); err != nil {
				return err
			}
			return nil
		}
		// Otherwise, we've completed execution
		vmExited = true
		vmExitCode = int(vm.registers[0]) // r0 is typically return value
		return nil
	}

	_, err := vm.executeInstruction()
	if err != nil {
		return err
	}

	vm.instructionCount++
	return nil
}

// HasExited returns true if the program has finished execution
func (vm *VM) HasExited() bool {
	return vmExited
}

// ExitCode returns the program exit code
func (vm *VM) ExitCode() int {
	return vmExitCode
}

// GetCurrentLocation returns the current function name and instruction index
func (vm *VM) GetCurrentLocation() (string, int) {
	if vm.currentFunc == nil {
		return "", 0
	}
	return vm.currentFunc.Name, vm.pc
}

// GetRegisters returns a copy of all register values
func (vm *VM) GetRegisters() map[ir.Register]int64 {
	regs := make(map[ir.Register]int64)
	for i := 0; i < 256; i++ {
		if vm.registers[i] != 0 {
			regs[ir.Register(i)] = vm.registers[i]
		}
	}
	return regs
}

// ReadMemory reads a slice of memory
func (vm *VM) ReadMemory(addr uint32, size int) []byte {
	if int(addr)+size > len(vm.memory) {
		// Return what we can
		if int(addr) >= len(vm.memory) {
			return make([]byte, size)
		}
		size = len(vm.memory) - int(addr)
	}

	result := make([]byte, size)
	copy(result, vm.memory[addr:int(addr)+size])
	return result
}

// WriteMemory writes data to memory
func (vm *VM) WriteMemory(addr uint32, data []byte) {
	if int(addr) >= len(vm.memory) {
		return
	}

	end := int(addr) + len(data)
	if end > len(vm.memory) {
		end = len(vm.memory)
	}

	copy(vm.memory[addr:end], data[:end-int(addr)])
}

// GetCallStack returns the current call stack
func (vm *VM) GetCallStack() []MIRStackFrame {
	stack := make([]MIRStackFrame, len(vm.callStack))
	for i, frame := range vm.callStack {
		stack[i] = MIRStackFrame{
			FuncName:   frame.Function.Name,
			InstIndex:  frame.ReturnPC,
			ReturnAddr: frame.ReturnPC,
		}
	}

	// Add current frame
	if vm.currentFunc != nil {
		stack = append(stack, MIRStackFrame{
			FuncName:  vm.currentFunc.Name,
			InstIndex: vm.pc,
		})
	}

	return stack
}

// Reset resets the VM state for a new execution
func (vm *VM) Reset() {
	vmExited = false
	vmExitCode = 0
	vm.pc = 0
	vm.sp = vm.config.StackSize
	vm.fp = vm.config.StackSize
	vm.callStack = nil
	vm.instructionCount = 0
	vm.breakHit = false
	vm.stepMode = false

	// Clear registers
	for i := range vm.registers {
		vm.registers[i] = 0
	}

	// Clear memory
	for i := range vm.memory {
		vm.memory[i] = 0
	}
}