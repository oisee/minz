// Package debugger provides debugging capabilities for MinZ programs
// running on the MZE Z80 emulator.
//
// This package serves as the core debugging logic that can be exposed
// via multiple protocols:
// - DAP (Debug Adapter Protocol) for VS Code integration
// - GDB RSP (Remote Serial Protocol) for universal tool compatibility
package debugger

import (
	"fmt"
	"sync"

	"github.com/minz/minzc/pkg/emulator"
)

// DebugState represents the current state of the debugger
type DebugState int

const (
	StateStopped DebugState = iota
	StateRunning
	StatePaused
	StateExited
)

func (s DebugState) String() string {
	switch s {
	case StateStopped:
		return "stopped"
	case StateRunning:
		return "running"
	case StatePaused:
		return "paused"
	case StateExited:
		return "exited"
	default:
		return "unknown"
	}
}

// Breakpoint represents a breakpoint in the program
type Breakpoint struct {
	ID         int
	Address    uint16
	Enabled    bool
	Condition  string
	HitCount   int
	Temporary  bool
	SourceInfo *SourceLocation
}

// SourceLocation maps an address to source code
type SourceLocation struct {
	File   string
	Line   int
	Column int
}

// StopReason indicates why the debugger stopped
type StopReason int

const (
	StopNone StopReason = iota
	StopBreakpoint
	StopStep
	StopPause
	StopException
	StopExit
)

func (r StopReason) String() string {
	switch r {
	case StopBreakpoint:
		return "breakpoint"
	case StopStep:
		return "step"
	case StopPause:
		return "pause"
	case StopException:
		return "exception"
	case StopExit:
		return "exit"
	default:
		return "none"
	}
}

// StopEvent is emitted when the debugger stops
type StopEvent struct {
	Reason     StopReason
	Breakpoint *Breakpoint
	Address    uint16
	Message    string
}

// SMCEvent represents a self-modifying code event
type SMCEvent struct {
	Address  uint16
	OldValue byte
	NewValue byte
	PC       uint16
}

// Debugger provides debugging capabilities for Z80 programs
type Debugger struct {
	mu sync.Mutex

	emu *emulator.RemogattoZ80

	state      DebugState
	stopReason StopReason

	breakpoints   map[uint16]*Breakpoint
	nextBreakID   int
	stepBreakAddr uint16

	sourceMap *SourceMap

	smcEvents  []SMCEvent
	smcEnabled bool
	smcHeatmap map[uint16]int

	onStop func(StopEvent)
	onSMC  func(SMCEvent)

	pauseRequested bool
}

// SourceMap maps addresses to source locations
type SourceMap struct {
	AddrToSource map[uint16]*SourceLocation
	SourceToAddr map[string]map[int]uint16
	Symbols      map[string]uint16
}

// NewDebugger creates a new debugger instance
func NewDebugger() *Debugger {
	d := &Debugger{
		breakpoints: make(map[uint16]*Breakpoint),
		smcHeatmap:  make(map[uint16]int),
		smcEnabled:  true,
		sourceMap: &SourceMap{
			AddrToSource: make(map[uint16]*SourceLocation),
			SourceToAddr: make(map[string]map[int]uint16),
			Symbols:      make(map[string]uint16),
		},
	}

	d.emu = emulator.NewRemogattoZ80()

	d.emu.SetSMCTracker(func(addr uint16, oldVal, newVal byte) {
		d.handleSMCEvent(addr, oldVal, newVal)
	})

	return d
}

func (d *Debugger) handleSMCEvent(addr uint16, oldVal, newVal byte) {
	if !d.smcEnabled {
		return
	}

	event := SMCEvent{
		Address:  addr,
		OldValue: oldVal,
		NewValue: newVal,
		PC:       d.emu.GetPC(),
	}

	d.mu.Lock()
	d.smcEvents = append(d.smcEvents, event)
	d.smcHeatmap[addr]++
	d.mu.Unlock()

	if d.onSMC != nil {
		d.onSMC(event)
	}
}

// LoadProgram loads a program into the emulator
func (d *Debugger) LoadProgram(binary []byte, loadAddr, startAddr uint16) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	err := d.emu.LoadMemory(loadAddr, binary)
	if err != nil {
		return err
	}

	d.emu.SetPC(startAddr)
	d.state = StateStopped
	return nil
}

// SetBreakpoint sets a breakpoint at the specified address
func (d *Debugger) SetBreakpoint(addr uint16) *Breakpoint {
	d.mu.Lock()
	defer d.mu.Unlock()

	if bp, exists := d.breakpoints[addr]; exists {
		return bp
	}

	bp := &Breakpoint{
		ID:      d.nextBreakID,
		Address: addr,
		Enabled: true,
	}
	d.nextBreakID++

	if loc, ok := d.sourceMap.AddrToSource[addr]; ok {
		bp.SourceInfo = loc
	}

	d.breakpoints[addr] = bp
	return bp
}

// ClearBreakpoint removes a breakpoint
func (d *Debugger) ClearBreakpoint(addr uint16) bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	if _, exists := d.breakpoints[addr]; exists {
		delete(d.breakpoints, addr)
		return true
	}
	return false
}

// Continue resumes execution
func (d *Debugger) Continue() error {
	d.mu.Lock()
	if d.state != StatePaused && d.state != StateStopped {
		d.mu.Unlock()
		return fmt.Errorf("cannot continue: state is %s", d.state)
	}
	d.state = StateRunning
	d.pauseRequested = false
	d.mu.Unlock()

	return d.run()
}

// Step executes a single instruction
func (d *Debugger) Step() error {
	d.mu.Lock()
	if d.state != StatePaused && d.state != StateStopped {
		d.mu.Unlock()
		return fmt.Errorf("cannot step: state is %s", d.state)
	}
	d.mu.Unlock()

	d.emu.Step()

	if d.emu.IsHalted() {
		d.mu.Lock()
		d.state = StateExited
		d.stopReason = StopExit
		d.mu.Unlock()
		d.notifyStop(StopEvent{Reason: StopExit, Address: d.emu.GetPC()})
		return nil
	}

	d.mu.Lock()
	d.state = StatePaused
	d.stopReason = StopStep
	d.mu.Unlock()

	d.notifyStop(StopEvent{Reason: StopStep, Address: d.emu.GetPC()})
	return nil
}

// Pause requests the debugger to pause
func (d *Debugger) Pause() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.pauseRequested = true
}

func (d *Debugger) run() error {
	for {
		d.mu.Lock()
		if d.pauseRequested {
			d.state = StatePaused
			d.stopReason = StopPause
			d.pauseRequested = false
			d.mu.Unlock()
			d.notifyStop(StopEvent{Reason: StopPause, Address: d.emu.GetPC()})
			return nil
		}

		pc := d.emu.GetPC()

		if bp, exists := d.breakpoints[pc]; exists && bp.Enabled {
			bp.HitCount++
			d.state = StatePaused
			d.stopReason = StopBreakpoint
			d.mu.Unlock()
			d.notifyStop(StopEvent{
				Reason:     StopBreakpoint,
				Breakpoint: bp,
				Address:    pc,
			})

			if bp.Temporary {
				delete(d.breakpoints, pc)
			}
			return nil
		}
		d.mu.Unlock()

		d.emu.Step()

		if d.emu.IsHalted() {
			d.mu.Lock()
			d.state = StateExited
			d.stopReason = StopExit
			d.mu.Unlock()
			d.notifyStop(StopEvent{Reason: StopExit, Address: d.emu.GetPC()})
			return nil
		}
	}
}

func (d *Debugger) notifyStop(event StopEvent) {
	if d.onStop != nil {
		d.onStop(event)
	}
}

// GetState returns the current debugger state
func (d *Debugger) GetState() DebugState {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.state
}

// GetRegisters returns the current CPU registers
func (d *Debugger) GetRegisters() emulator.Registers {
	return d.emu.GetRegisters()
}

// GetPC returns the current program counter
func (d *Debugger) GetPC() uint16 {
	return d.emu.GetPC()
}

// ReadMemory reads memory from the emulator
func (d *Debugger) ReadMemory(addr uint16, size int) []byte {
	data := make([]byte, size)
	for i := 0; i < size; i++ {
		data[i] = d.emu.GetMemory(addr + uint16(i))
	}
	return data
}

// WriteMemory writes data to memory
func (d *Debugger) WriteMemory(addr uint16, data []byte) {
	for i, b := range data {
		d.emu.SetMemory(addr+uint16(i), b)
	}
}

// GetCycles returns the total cycles executed
func (d *Debugger) GetCycles() int {
	return d.emu.GetCycles()
}

// GetSMCEvents returns all SMC events
func (d *Debugger) GetSMCEvents() []SMCEvent {
	d.mu.Lock()
	defer d.mu.Unlock()
	events := make([]SMCEvent, len(d.smcEvents))
	copy(events, d.smcEvents)
	return events
}

// GetSMCHeatmap returns the SMC modification count per address
func (d *Debugger) GetSMCHeatmap() map[uint16]int {
	d.mu.Lock()
	defer d.mu.Unlock()
	heatmap := make(map[uint16]int)
	for k, v := range d.smcHeatmap {
		heatmap[k] = v
	}
	return heatmap
}

// OnStop sets the callback for stop events
func (d *Debugger) OnStop(callback func(StopEvent)) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.onStop = callback
}

// OnSMC sets the callback for SMC events
func (d *Debugger) OnSMC(callback func(SMCEvent)) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.onSMC = callback
}
