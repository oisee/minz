// Package mirvm provides debugging capabilities for MIR code
package mirvm

import (
	"fmt"
	"sync"

	"github.com/minz/minzc/pkg/ir"
)

// DebugState represents the current state of the debugger
type DebugState int

const (
	MIRStateStopped DebugState = iota
	MIRStateRunning
	MIRStatePaused
	MIRStateExited
)

func (s DebugState) String() string {
	switch s {
	case MIRStateStopped:
		return "stopped"
	case MIRStateRunning:
		return "running"
	case MIRStatePaused:
		return "paused"
	case MIRStateExited:
		return "exited"
	default:
		return "unknown"
	}
}

// MIRBreakpoint represents a breakpoint in MIR code
type MIRBreakpoint struct {
	ID         int
	FuncName   string  // Function name
	InstIndex  int     // Instruction index within function
	Enabled    bool
	Condition  string
	HitCount   int
	Temporary  bool
	SourceInfo *MIRSourceLocation
}

// MIRSourceLocation maps an MIR instruction to source code
type MIRSourceLocation struct {
	File   string
	Line   int
	Column int
}

// MIRStopReason indicates why the debugger stopped
type MIRStopReason int

const (
	MIRStopNone MIRStopReason = iota
	MIRStopBreakpoint
	MIRStopStep
	MIRStopPause
	MIRStopException
	MIRStopExit
)

func (r MIRStopReason) String() string {
	switch r {
	case MIRStopBreakpoint:
		return "breakpoint"
	case MIRStopStep:
		return "step"
	case MIRStopPause:
		return "pause"
	case MIRStopException:
		return "exception"
	case MIRStopExit:
		return "exit"
	default:
		return "none"
	}
}

// MIRStopEvent is emitted when the debugger stops
type MIRStopEvent struct {
	Reason     MIRStopReason
	Breakpoint *MIRBreakpoint
	FuncName   string
	InstIndex  int
	Message    string
}

// MIRRegisters represents the MIR VM register state
type MIRRegisters struct {
	Values map[ir.Register]int64
	PC     int    // Current instruction index
	Func   string // Current function name
}

// MIRDebugger provides debugging capabilities for MIR programs
type MIRDebugger struct {
	mu sync.Mutex

	vm       *VM
	module   *ir.Module
	platform Platform

	state      DebugState
	stopReason MIRStopReason

	breakpoints     map[string]map[int]*MIRBreakpoint // func -> instIndex -> bp
	nextBreakID     int
	stepBreakFunc   string
	stepBreakInst   int

	sourceMap *MIRSourceMap

	onStop func(MIRStopEvent)

	pauseRequested bool
	exitCode       int
}

// MIRSourceMap maps MIR instructions to source locations
type MIRSourceMap struct {
	InstToSource map[string]map[int]*MIRSourceLocation // func -> instIndex -> source
	SourceToInst map[string]map[int][]MIRInstRef       // file -> line -> instructions
	Symbols      map[string]string                      // symbol -> function
}

// MIRInstRef references an MIR instruction
type MIRInstRef struct {
	FuncName  string
	InstIndex int
}

// NewMIRDebugger creates a new MIR debugger instance
func NewMIRDebugger(platform Platform) *MIRDebugger {
	if platform == nil {
		platform = NewHeadlessPlatform()
	}

	return &MIRDebugger{
		platform:    platform,
		breakpoints: make(map[string]map[int]*MIRBreakpoint),
		sourceMap: &MIRSourceMap{
			InstToSource: make(map[string]map[int]*MIRSourceLocation),
			SourceToInst: make(map[string]map[int][]MIRInstRef),
			Symbols:      make(map[string]string),
		},
		state: MIRStateStopped,
	}
}

// LoadModule loads an MIR module for debugging
func (d *MIRDebugger) LoadModule(module *ir.Module) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.module = module

	// Create VM with the platform
	config := Config{
		Platform:   d.platform,
		MemorySize: 64 * 1024, // 64KB default
	}

	var err error
	d.vm = New(config)
	err = d.vm.LoadModule(module)
	if err != nil {
		return fmt.Errorf("create VM: %w", err)
	}

	d.state = MIRStateStopped
	return nil
}

// SetBreakpoint sets a breakpoint at the specified location
func (d *MIRDebugger) SetBreakpoint(funcName string, instIndex int) *MIRBreakpoint {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.breakpoints[funcName] == nil {
		d.breakpoints[funcName] = make(map[int]*MIRBreakpoint)
	}

	if bp, exists := d.breakpoints[funcName][instIndex]; exists {
		return bp
	}

	bp := &MIRBreakpoint{
		ID:        d.nextBreakID,
		FuncName:  funcName,
		InstIndex: instIndex,
		Enabled:   true,
	}
	d.nextBreakID++

	// Check for source mapping
	if funcMap, ok := d.sourceMap.InstToSource[funcName]; ok {
		if loc, ok := funcMap[instIndex]; ok {
			bp.SourceInfo = loc
		}
	}

	d.breakpoints[funcName][instIndex] = bp
	return bp
}

// ClearBreakpoint removes a breakpoint
func (d *MIRDebugger) ClearBreakpoint(funcName string, instIndex int) bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	if funcBps, exists := d.breakpoints[funcName]; exists {
		if _, exists := funcBps[instIndex]; exists {
			delete(funcBps, instIndex)
			return true
		}
	}
	return false
}

// Continue resumes execution
func (d *MIRDebugger) Continue() error {
	d.mu.Lock()
	if d.state != MIRStatePaused && d.state != MIRStateStopped {
		d.mu.Unlock()
		return fmt.Errorf("cannot continue: state is %s", d.state)
	}
	d.state = MIRStateRunning
	d.pauseRequested = false
	d.mu.Unlock()

	return d.run()
}

// Step executes a single MIR instruction
func (d *MIRDebugger) Step() error {
	d.mu.Lock()
	if d.state != MIRStatePaused && d.state != MIRStateStopped {
		d.mu.Unlock()
		return fmt.Errorf("cannot step: state is %s", d.state)
	}
	d.mu.Unlock()

	// Execute one instruction
	err := d.vm.Step()

	if err != nil {
		d.mu.Lock()
		d.state = MIRStateExited
		d.stopReason = MIRStopException
		d.mu.Unlock()
		d.notifyStop(MIRStopEvent{
			Reason:  MIRStopException,
			Message: err.Error(),
		})
		return nil
	}

	if d.vm.HasExited() {
		d.mu.Lock()
		d.state = MIRStateExited
		d.stopReason = MIRStopExit
		d.exitCode = d.vm.ExitCode()
		d.mu.Unlock()
		d.notifyStop(MIRStopEvent{
			Reason: MIRStopExit,
		})
		return nil
	}

	d.mu.Lock()
	d.state = MIRStatePaused
	d.stopReason = MIRStopStep
	d.mu.Unlock()

	funcName, instIndex := d.vm.GetCurrentLocation()
	d.notifyStop(MIRStopEvent{
		Reason:    MIRStopStep,
		FuncName:  funcName,
		InstIndex: instIndex,
	})
	return nil
}

// Pause requests the debugger to pause
func (d *MIRDebugger) Pause() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.pauseRequested = true
}

func (d *MIRDebugger) run() error {
	for {
		d.mu.Lock()
		if d.pauseRequested {
			d.state = MIRStatePaused
			d.stopReason = MIRStopPause
			d.pauseRequested = false
			d.mu.Unlock()

			funcName, instIndex := d.vm.GetCurrentLocation()
			d.notifyStop(MIRStopEvent{
				Reason:    MIRStopPause,
				FuncName:  funcName,
				InstIndex: instIndex,
			})
			return nil
		}

		funcName, instIndex := d.vm.GetCurrentLocation()

		// Check for breakpoint
		if funcBps, exists := d.breakpoints[funcName]; exists {
			if bp, exists := funcBps[instIndex]; exists && bp.Enabled {
				bp.HitCount++
				d.state = MIRStatePaused
				d.stopReason = MIRStopBreakpoint
				d.mu.Unlock()

				d.notifyStop(MIRStopEvent{
					Reason:     MIRStopBreakpoint,
					Breakpoint: bp,
					FuncName:   funcName,
					InstIndex:  instIndex,
				})

				if bp.Temporary {
					delete(funcBps, instIndex)
				}
				return nil
			}
		}
		d.mu.Unlock()

		// Execute one instruction
		err := d.vm.Step()

		if err != nil {
			d.mu.Lock()
			d.state = MIRStateExited
			d.stopReason = MIRStopException
			d.mu.Unlock()
			d.notifyStop(MIRStopEvent{
				Reason:  MIRStopException,
				Message: err.Error(),
			})
			return nil
		}

		if d.vm.HasExited() {
			d.mu.Lock()
			d.state = MIRStateExited
			d.stopReason = MIRStopExit
			d.exitCode = d.vm.ExitCode()
			d.mu.Unlock()
			d.notifyStop(MIRStopEvent{
				Reason: MIRStopExit,
			})
			return nil
		}
	}
}

func (d *MIRDebugger) notifyStop(event MIRStopEvent) {
	if d.onStop != nil {
		d.onStop(event)
	}
}

// GetState returns the current debugger state
func (d *MIRDebugger) GetState() DebugState {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.state
}

// GetRegisters returns the current MIR register values
func (d *MIRDebugger) GetRegisters() MIRRegisters {
	funcName, instIndex := d.vm.GetCurrentLocation()

	return MIRRegisters{
		Values: d.vm.GetRegisters(),
		PC:     instIndex,
		Func:   funcName,
	}
}

// GetCurrentLocation returns the current function and instruction index
func (d *MIRDebugger) GetCurrentLocation() (string, int) {
	return d.vm.GetCurrentLocation()
}

// ReadMemory reads memory from the VM
func (d *MIRDebugger) ReadMemory(addr uint32, size int) []byte {
	return d.vm.ReadMemory(addr, size)
}

// WriteMemory writes data to VM memory
func (d *MIRDebugger) WriteMemory(addr uint32, data []byte) {
	d.vm.WriteMemory(addr, data)
}

// GetModule returns the loaded MIR module
func (d *MIRDebugger) GetModule() *ir.Module {
	return d.module
}

// GetExitCode returns the exit code if the program has exited
func (d *MIRDebugger) GetExitCode() int {
	return d.exitCode
}

// OnStop sets the callback for stop events
func (d *MIRDebugger) OnStop(callback func(MIRStopEvent)) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.onStop = callback
}

// AddSourceMapping adds a source location mapping for an MIR instruction
func (d *MIRDebugger) AddSourceMapping(funcName string, instIndex int, file string, line, col int) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Add instruction -> source mapping
	if d.sourceMap.InstToSource[funcName] == nil {
		d.sourceMap.InstToSource[funcName] = make(map[int]*MIRSourceLocation)
	}
	d.sourceMap.InstToSource[funcName][instIndex] = &MIRSourceLocation{
		File:   file,
		Line:   line,
		Column: col,
	}

	// Add source -> instruction mapping
	if d.sourceMap.SourceToInst[file] == nil {
		d.sourceMap.SourceToInst[file] = make(map[int][]MIRInstRef)
	}
	d.sourceMap.SourceToInst[file][line] = append(
		d.sourceMap.SourceToInst[file][line],
		MIRInstRef{FuncName: funcName, InstIndex: instIndex},
	)
}

// GetSourceLocation returns the source location for an MIR instruction
func (d *MIRDebugger) GetSourceLocation(funcName string, instIndex int) *MIRSourceLocation {
	d.mu.Lock()
	defer d.mu.Unlock()

	if funcMap, ok := d.sourceMap.InstToSource[funcName]; ok {
		if loc, ok := funcMap[instIndex]; ok {
			return loc
		}
	}
	return nil
}

// GetInstructionsForLine returns all MIR instructions for a source line
func (d *MIRDebugger) GetInstructionsForLine(file string, line int) []MIRInstRef {
	d.mu.Lock()
	defer d.mu.Unlock()

	if fileMap, ok := d.sourceMap.SourceToInst[file]; ok {
		if refs, ok := fileMap[line]; ok {
			result := make([]MIRInstRef, len(refs))
			copy(result, refs)
			return result
		}
	}
	return nil
}

// GetCallStack returns the current call stack
func (d *MIRDebugger) GetCallStack() []MIRStackFrame {
	return d.vm.GetCallStack()
}

// MIRStackFrame represents a stack frame in the MIR VM
type MIRStackFrame struct {
	FuncName    string
	InstIndex   int
	ReturnAddr  int
	SourceInfo  *MIRSourceLocation
}
