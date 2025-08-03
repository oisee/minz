package tas

import (
	"fmt"
)

// Z80Emulator interface for Z80 emulation
type Z80Emulator interface {
	GetPC() uint16
	SetPC(uint16)
	GetSP() uint16
	SetSP(uint16)
	GetA() byte
	SetA(byte)
	GetB() byte
	SetB(byte)
	GetC() byte
	SetC(byte)
	GetD() byte
	SetD(byte)
	GetE() byte
	SetE(byte)
	GetF() byte
	SetF(byte)
	GetH() byte
	SetH(byte)
	GetL() byte
	SetL(byte)
	GetIX() uint16
	SetIX(uint16)
	GetIY() uint16
	SetIY(uint16)
	GetI() byte
	SetI(byte)
	GetR() byte
	SetR(byte)
	GetIFF1() bool
	SetIFF1(bool)
	GetIFF2() bool
	SetIFF2(bool)
	GetShadowA() byte
	SetShadowA(byte)
	GetShadowB() byte
	SetShadowB(byte)
	GetShadowC() byte
	SetShadowC(byte)
	GetShadowD() byte
	SetShadowD(byte)
	GetShadowE() byte
	SetShadowE(byte)
	GetShadowF() byte
	SetShadowF(byte)
	GetShadowH() byte
	SetShadowH(byte)
	GetShadowL() byte
	SetShadowL(byte)
	GetCycles() uint64
	SetCycles(uint64)
	GetTStates() uint64
	SetTStates(uint64)
	GetMemory() []byte
	SetMemory([]byte)
	ReadByte(uint16) byte
	WriteByte(uint16, byte)
	GetBorder() byte
	GetLastOpcode() string
	GetRegisters() *CPURegisters
}

// CPURegisters holds all CPU register values
type CPURegisters struct {
	PC, SP     uint16
	A, B, C, D, E, F, H, L byte
	A_, B_, C_, D_, E_, F_, H_, L_ byte  // Shadow registers
	IX, IY     uint16
	I, R       byte
	IFF1, IFF2 bool
}

// TASDebugger implements Tool-Assisted Speedrun inspired debugging for Z80
// Memory is cheap - we record EVERYTHING!
type TASDebugger struct {
	emulator     Z80Emulator
	stateHistory []StateSnapshot
	currentFrame int64
	recording    bool
	
	// Save states - like TAS emulators
	saveStates   map[string]*StateSnapshot
	
	// Input recording for perfect replay
	inputLog     []InputEvent
	inputIndex   int
	
	// SMC tracking with visual timeline
	smcEvents    []SMCEvent
	
	// Frame-perfect optimization hunting
	huntMode     bool
	huntGoal     OptimizationGoal
	bestPath     []StateSnapshot
	
	// Memory is cheap - record everything!
	maxHistory   int  // Default: 1,000,000 frames (~16 minutes at 60fps)
}

// StateSnapshot captures complete Z80 state at a single cycle
// This is only 65KB + registers - we can store thousands!
type StateSnapshot struct {
	Cycle       uint64
	Frame       uint64         // For 50Hz display
	
	// Complete CPU state
	PC          uint16
	SP          uint16
	A, B, C, D, E, F, H, L byte
	A_, B_, C_, D_, E_, F_, H_, L_ byte  // Shadow registers
	IX, IY      uint16
	I, R        byte
	IFF1, IFF2  bool
	
	// Complete memory - only 64KB!
	Memory      [65536]byte
	
	// Screen state for visual debugging
	Screen      [6912]byte  // ZX Spectrum screen + attributes
	Border      byte
	
	// Performance metrics
	TStates     uint64
	
	// Metadata for debugging
	LastOpcode  string
	StackTrace  []uint16
}

// InputEvent records user input with cycle-perfect timing
type InputEvent struct {
	Cycle    uint64
	Frame    uint64
	Port     uint16  // IN port
	Value    byte    // Key state
	Type     string  // "key", "joy", "mouse"
}

// SMCEvent tracks self-modifying code for visualization
type SMCEvent struct {
	Cycle       uint64
	PC          uint16      // Where SMC happened
	Address     uint16      // What was modified
	OldValue    byte        // Previous instruction byte
	NewValue    byte        // New instruction byte
	Reason      string      // "parameter_patch", "loop_optimization", etc.
}

// OptimizationGoal for frame-perfect hunting
type OptimizationGoal struct {
	TargetPC    uint16      // Reach this address
	TargetRegs  map[string]byte  // With these register values
	MaxCycles   uint64      // In minimum cycles
}

// NewTASDebugger creates a TAS-inspired debugger
func NewTASDebugger(emu Z80Emulator) *TASDebugger {
	return &TASDebugger{
		emulator:     emu,
		stateHistory: make([]StateSnapshot, 0, 1000000),
		saveStates:   make(map[string]*StateSnapshot),
		inputLog:     make([]InputEvent, 0, 10000),
		smcEvents:    make([]SMCEvent, 0, 1000),
		maxHistory:   1000000,  // ~16 minutes at 60fps
	}
}

// RecordFrame captures current state - called every instruction or frame
func (t *TASDebugger) RecordFrame() {
	if !t.recording {
		return
	}
	
	snapshot := t.captureState()
	
	// Ring buffer - overwrite old history if we exceed max
	if len(t.stateHistory) >= t.maxHistory {
		// Compress old history to save space
		t.compressOldHistory()
		t.stateHistory = t.stateHistory[100000:]  // Keep most recent
	}
	
	t.stateHistory = append(t.stateHistory, snapshot)
	t.currentFrame++
}

// captureState creates a complete snapshot of Z80 state
func (t *TASDebugger) captureState() StateSnapshot {
	snap := StateSnapshot{
		Cycle: t.emulator.GetCycles(),
		Frame: uint64(t.currentFrame),
	}
	
	// Capture CPU registers
	regs := t.emulator.GetRegisters()
	snap.PC = regs.PC
	snap.SP = regs.SP
	snap.A = regs.A
	snap.B = regs.B
	snap.C = regs.C
	snap.D = regs.D
	snap.E = regs.E
	snap.F = regs.F
	snap.H = regs.H
	snap.L = regs.L
	
	// Shadow registers
	snap.A_ = regs.A_
	snap.B_ = regs.B_
	snap.C_ = regs.C_
	snap.D_ = regs.D_
	snap.E_ = regs.E_
	snap.F_ = regs.F_
	snap.H_ = regs.H_
	snap.L_ = regs.L_
	
	// Index registers
	snap.IX = regs.IX
	snap.IY = regs.IY
	snap.I = regs.I
	snap.R = regs.R
	
	// Interrupt state
	snap.IFF1 = regs.IFF1
	snap.IFF2 = regs.IFF2
	
	// Copy entire memory - it's only 64KB!
	copy(snap.Memory[:], t.emulator.GetMemory())
	
	// ZX Spectrum screen (if applicable)
	copy(snap.Screen[:], t.emulator.GetMemory()[0x4000:0x5B00])
	snap.Border = t.emulator.GetBorder()
	
	// Performance data
	snap.TStates = t.emulator.GetTStates()
	
	// Debug info
	snap.LastOpcode = t.emulator.GetLastOpcode()
	snap.StackTrace = t.getStackTrace()
	
	return snap
}

// Rewind goes back in time to a previous state
func (t *TASDebugger) Rewind(frames int) error {
	targetFrame := t.currentFrame - int64(frames)
	if targetFrame < 0 {
		targetFrame = 0
	}
	
	if targetFrame >= int64(len(t.stateHistory)) {
		return fmt.Errorf("cannot rewind to frame %d (only %d frames recorded)", 
			targetFrame, len(t.stateHistory))
	}
	
	// Restore the state
	t.restoreState(&t.stateHistory[targetFrame])
	t.currentFrame = targetFrame
	
	return nil
}

// SaveState creates a named save state (like TAS save slots)
func (t *TASDebugger) SaveState(name string) {
	snapshot := t.captureState()
	t.saveStates[name] = &snapshot
	fmt.Printf("State saved: '%s' at frame %d\n", name, t.currentFrame)
}

// LoadState restores a named save state
func (t *TASDebugger) LoadState(name string) error {
	state, exists := t.saveStates[name]
	if !exists {
		return fmt.Errorf("save state '%s' not found", name)
	}
	
	t.restoreState(state)
	fmt.Printf("State loaded: '%s' (frame %d)\n", name, state.Frame)
	return nil
}

// restoreState sets emulator to a previous state
func (t *TASDebugger) restoreState(state *StateSnapshot) {
	// Restore CPU registers
	t.emulator.SetPC(state.PC)
	t.emulator.SetSP(state.SP)
	t.emulator.SetA(state.A)
	t.emulator.SetB(state.B)
	t.emulator.SetC(state.C)
	t.emulator.SetD(state.D)
	t.emulator.SetE(state.E)
	t.emulator.SetF(state.F)
	t.emulator.SetH(state.H)
	t.emulator.SetL(state.L)
	
	// Restore shadow registers
	t.emulator.SetShadowA(state.A_)
	t.emulator.SetShadowB(state.B_)
	t.emulator.SetShadowC(state.C_)
	t.emulator.SetShadowD(state.D_)
	t.emulator.SetShadowE(state.E_)
	t.emulator.SetShadowF(state.F_)
	t.emulator.SetShadowH(state.H_)
	t.emulator.SetShadowL(state.L_)
	
	// Restore index registers
	t.emulator.SetIX(state.IX)
	t.emulator.SetIY(state.IY)
	t.emulator.SetI(state.I)
	t.emulator.SetR(state.R)
	
	// Restore interrupt state
	t.emulator.SetIFF1(state.IFF1)
	t.emulator.SetIFF2(state.IFF2)
	
	// Restore complete memory
	t.emulator.SetMemory(state.Memory[:])
	
	// Restore timing
	t.emulator.SetCycles(state.Cycle)
	t.emulator.SetTStates(state.TStates)
}

// RecordInput logs user input for perfect replay
func (t *TASDebugger) RecordInput(port uint16, value byte) {
	if !t.recording {
		return
	}
	
	event := InputEvent{
		Cycle: t.emulator.GetCycles(),
		Frame: uint64(t.currentFrame),
		Port:  port,
		Value: value,
		Type:  "key",
	}
	
	t.inputLog = append(t.inputLog, event)
}

// ReplayInput feeds recorded input back during replay
func (t *TASDebugger) ReplayInput() (uint16, byte, bool) {
	if t.inputIndex >= len(t.inputLog) {
		return 0, 0, false
	}
	
	event := t.inputLog[t.inputIndex]
	if event.Cycle <= t.emulator.GetCycles() {
		t.inputIndex++
		return event.Port, event.Value, true
	}
	
	return 0, 0, false
}

// TrackSMC records self-modifying code events
func (t *TASDebugger) TrackSMC(pc, address uint16, oldVal, newVal byte) {
	event := SMCEvent{
		Cycle:    t.emulator.GetCycles(),
		PC:       pc,
		Address:  address,
		OldValue: oldVal,
		NewValue: newVal,
		Reason:   t.analyzeSMCReason(pc, address),
	}
	
	t.smcEvents = append(t.smcEvents, event)
}

// analyzeSMCReason tries to determine why SMC happened
func (t *TASDebugger) analyzeSMCReason(pc, address uint16) string {
	// Simple heuristics for now
	if address >= pc-10 && address <= pc+10 {
		return "parameter_patch"
	}
	if address >= 0x8000 && address < 0x9000 {
		return "loop_optimization"
	}
	return "dynamic_code"
}

// StartOptimizationHunt begins searching for optimal execution path
func (t *TASDebugger) StartOptimizationHunt(goal OptimizationGoal) {
	t.huntMode = true
	t.huntGoal = goal
	t.bestPath = nil
	fmt.Printf("🎯 Optimization hunt started: reach PC=%04X in minimum cycles\n", goal.TargetPC)
}

// CheckOptimizationGoal sees if we found a better path
func (t *TASDebugger) CheckOptimizationGoal() {
	if !t.huntMode {
		return
	}
	
	if t.emulator.GetPC() == t.huntGoal.TargetPC {
		cycles := t.emulator.GetCycles()
		
		// Check if this is better than our best
		if t.bestPath == nil || cycles < t.bestPath[len(t.bestPath)-1].Cycle {
			t.bestPath = make([]StateSnapshot, len(t.stateHistory))
			copy(t.bestPath, t.stateHistory)
			fmt.Printf("🏆 New best path found: %d cycles (saved %d)\n", 
				cycles, t.huntGoal.MaxCycles - cycles)
		}
	}
}

// GetTimeline returns visual representation of execution
func (t *TASDebugger) GetTimeline() string {
	if len(t.stateHistory) == 0 {
		return "No history recorded"
	}
	
	// Create ASCII timeline
	timeline := "Timeline: ["
	
	// Sample points from history
	samples := 50
	step := len(t.stateHistory) / samples
	if step < 1 {
		step = 1
	}
	
	for i := 0; i < len(t.stateHistory); i += step {
		state := t.stateHistory[i]
		
		// Mark special events
		if t.hasSMCEvent(state.Cycle) {
			timeline += "S"
		} else if t.hasInputEvent(state.Cycle) {
			timeline += "I"
		} else {
			timeline += "="
		}
	}
	
	timeline += fmt.Sprintf("] Frame %d/%d", t.currentFrame, len(t.stateHistory))
	return timeline
}

// hasSMCEvent checks if SMC happened at this cycle
func (t *TASDebugger) hasSMCEvent(cycle uint64) bool {
	for _, event := range t.smcEvents {
		if event.Cycle == cycle {
			return true
		}
	}
	return false
}

// hasInputEvent checks if input happened at this cycle
func (t *TASDebugger) hasInputEvent(cycle uint64) bool {
	for _, event := range t.inputLog {
		if event.Cycle == cycle {
			return true
		}
	}
	return false
}

// getStackTrace extracts current call stack
func (t *TASDebugger) getStackTrace() []uint16 {
	trace := make([]uint16, 0, 16)
	sp := t.emulator.GetSP()
	
	for i := 0; i < 16 && sp < 0xFFFF; i++ {
		// Read potential return address from stack
		low := t.emulator.ReadByte(sp)
		high := t.emulator.ReadByte(sp + 1)
		addr := uint16(high)<<8 | uint16(low)
		
		// Simple heuristic: likely a code address
		if addr >= 0x4000 && addr < 0xC000 {
			trace = append(trace, addr)
		}
		
		sp += 2
	}
	
	return trace
}

// compressOldHistory saves old frames in compressed format
func (t *TASDebugger) compressOldHistory() {
	// TODO: Implement compression of old frames
	// Could use gzip or delta compression
	fmt.Println("Compressing old history to save memory...")
}

// ExportReplay saves the recording for sharing (like TAS movies)
func (t *TASDebugger) ExportReplay(filename string) error {
	// TODO: Save state history and input log to file
	// Format could be similar to .fm2 or .bk2 TAS formats
	return nil
}

// ImportReplay loads a recording for playback
func (t *TASDebugger) ImportReplay(filename string) error {
	// TODO: Load state history and input log from file
	return nil
}