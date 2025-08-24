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
	
	// PGO Profile collection (Quick Win #2)
	blockExecutions map[uint16]uint64  // PC -> execution count
	branchOutcomes  map[uint16]bool    // PC -> last branch taken?
	
	// Frame-perfect optimization hunting
	huntMode     bool
	huntGoal     OptimizationGoal
	bestPath     []StateSnapshot
	
	// Memory is cheap - record everything!
	maxHistory   int  // Default: 1,000,000 frames (~16 minutes at 60fps)
	
	// Hybrid recording with intelligent strategy
	hybridRecorder   *HybridRecorder
	recordingMode    RecordingStrategy
	cyclePerfect     *CyclePerfectRecorder
	determinism      *DeterminismDetector
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
	// Default to hybrid strategy for best balance
	config := RecorderConfig{
		Strategy:            StrategyHybrid,
		SnapshotOnIO:        true,
		IOSnapshotThreshold: 10,
		MaxDeterministic:    100000,
		MinSnapshotInterval: 1000,
		UncertaintyTolerance: 0.3,
		ParanoidMode:        false,
	}
	
	return &TASDebugger{
		emulator:       emu,
		stateHistory:   make([]StateSnapshot, 0, 1000000),
		saveStates:     make(map[string]*StateSnapshot),
		inputLog:       make([]InputEvent, 0, 10000),
		smcEvents:      make([]SMCEvent, 0, 1000),
		blockExecutions: make(map[uint16]uint64),  // PGO: track execution counts
		branchOutcomes:  make(map[uint16]bool),    // PGO: track branch outcomes
		maxHistory:     1000000,  // ~16 minutes at 60fps
		hybridRecorder: NewHybridRecorder(config),
		recordingMode:  StrategyHybrid,
		cyclePerfect:   NewCyclePerfectRecorder(),
		determinism:    NewDeterminismDetector(),
	}
}

// SetRecordingStrategy changes the recording strategy
func (t *TASDebugger) SetRecordingStrategy(strategy RecordingStrategy) {
	t.recordingMode = strategy
	t.hybridRecorder.strategy = strategy
	
	strategyNames := map[RecordingStrategy]string{
		StrategyAutomatic:     "Automatic",
		StrategyDeterministic: "Deterministic",
		StrategySnapshot:      "Snapshot",
		StrategyHybrid:        "Hybrid",
		StrategyParanoid:      "Paranoid",
	}
	
	fmt.Printf("📼 Recording strategy changed to: %s\n", strategyNames[strategy])
}

// RecordFrame captures current state - called every instruction or frame
func (t *TASDebugger) RecordFrame() {
	if !t.recording {
		return
	}
	
	snapshot := t.captureState()
	cycle := int64(t.emulator.GetCycles())
	
	// Create cycle event for the current instruction
	var event *CycleEvent
	if pc := t.emulator.GetPC(); pc != 0 {
		// Get last opcode (simplified - would need actual opcode)
		event = &CycleEvent{
			Cycle:   cycle,
			Type:    EventInstruction,
			TStates: 4, // Would need actual timing
			Data: InstructionData{
				Opcode: 0, // Would need actual opcode
				PC:     pc,
			},
		}
		
		// Feed to determinism detector
		t.determinism.ProcessEvent(*event)
		
		// Record with cycle-perfect timing
		t.cyclePerfect.RecordInstruction(0, pc, 4)
	}
	
	// Use hybrid recorder to decide on snapshots
	t.hybridRecorder.RecordCycle(cycle, &snapshot, event)
	
	// Still maintain state history for compatibility
	// but with smarter management
	if len(t.stateHistory) >= t.maxHistory {
		// Use hybrid recorder's decision on what to keep
		t.compressWithStrategy()
	}
	
	t.stateHistory = append(t.stateHistory, snapshot)
	t.currentFrame++
}

// compressWithStrategy uses recording strategy to compress history
func (t *TASDebugger) compressWithStrategy() {
	stats := t.hybridRecorder.GetStatistics()
	
	if stats.DeterministicRatio > 0.8 {
		// High determinism - keep only keyframes and events
		t.keepOnlyKeyframes()
	} else {
		// Low determinism - traditional compression
		t.compressOldHistory()
	}
}

// keepOnlyKeyframes retains only important snapshots
func (t *TASDebugger) keepOnlyKeyframes() {
	newHistory := make([]StateSnapshot, 0, len(t.stateHistory)/10)
	
	// Keep every 10,000th frame and I/O frames
	for i, snap := range t.stateHistory {
		if i%10000 == 0 || t.hasIOEvent(snap.Cycle) {
			newHistory = append(newHistory, snap)
		}
	}
	
	fmt.Printf("🗜️ Compressed history: %d → %d frames (%.1fx compression)\n",
		len(t.stateHistory), len(newHistory),
		float64(len(t.stateHistory))/float64(len(newHistory)))
	
	t.stateHistory = newHistory
}

// hasIOEvent checks if an I/O event occurred at this cycle
func (t *TASDebugger) hasIOEvent(cycle uint64) bool {
	// Check both input log and cycle recorder
	for _, event := range t.inputLog {
		if event.Cycle == cycle {
			return true
		}
	}
	
	// Check cycle-perfect recorder
	for _, event := range t.cyclePerfect.events {
		if uint64(event.Cycle) == cycle && 
		   (event.Type == EventIORead || event.Type == EventIOWrite) {
			return true
		}
	}
	
	return false
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
	// Use the TAS file format
	tasFile := CreateReplay(t)
	
	// Determine format from extension
	format := uint8(TASFormatJSON)
	if len(filename) > 5 {
		switch filename[len(filename)-5:] {
		case ".tasb":
			format = TASFormatBinary
		case ".tasc", "as.gz":
			format = TASFormatCompressed
		}
	}
	
	return tasFile.SaveToFile(filename, format)
}

// ImportReplay loads a recording for playback
func (t *TASDebugger) ImportReplay(filename string) error {
	tasFile, err := LoadFromFile(filename)
	if err != nil {
		return err
	}
	
	// Clear and load new data
	t.stateHistory = tasFile.States
	t.inputLog = tasFile.Events.Inputs
	t.smcEvents = tasFile.Events.SMCEvents
	
	if len(t.stateHistory) > 0 {
		t.currentFrame = int64(t.stateHistory[len(t.stateHistory)-1].Frame)
	}
	
	return nil
}

// GetRecordingStats returns comprehensive recording statistics
func (t *TASDebugger) GetRecordingStats() string {
	hybridStats := t.hybridRecorder.GetStatistics()
	detStats := t.determinism.GetStatistics()
	
	report := fmt.Sprintf(`
📊 TAS RECORDING STATISTICS
═══════════════════════════════════════════════════════════════
Recording Strategy:     %s
Total Frames:          %d
Total Cycles:          %d
State History Size:    %d snapshots
Input Events:          %d
SMC Events:            %d

💾 MEMORY USAGE:
─────────────────────────────────────────────────────────────
State Snapshots:       %.1f MB
Event Recording:       %.1f MB
Total Memory:          %.1f MB
Compression Ratio:     %.0fx

🎯 DETERMINISM ANALYSIS:
─────────────────────────────────────────────────────────────
Deterministic Ratio:   %.1f%%
Deterministic Cycles:  %d
Section Count:         %d
Average Section:       %.0f cycles
Compression Potential: %.0fx

🚀 OPTIMIZATION OPPORTUNITIES:
─────────────────────────────────────────────────────────────`,
		strategyString(t.recordingMode),
		t.currentFrame,
		hybridStats.TotalCycles,
		len(t.stateHistory),
		len(t.inputLog),
		len(t.smcEvents),
		float64(len(t.stateHistory)*65536)/1024/1024,
		float64(hybridStats.EventMemory)/1024/1024,
		float64(hybridStats.MemoryUsed)/1024/1024,
		hybridStats.CompressionRatio,
		detStats.Ratio*100,
		detStats.DeterministicCycles,
		detStats.SectionCount,
		detStats.AverageLength,
		detStats.CompressionPotential)
	
	// Add recommendations
	if detStats.Ratio > 0.8 {
		report += "\n✅ High determinism - Perfect for event-only recording"
	} else if detStats.Ratio > 0.5 {
		report += "\n⚠️  Medium determinism - Hybrid approach recommended"
	} else {
		report += "\n❌ Low determinism - Frequent snapshots needed"
	}
	
	if len(t.smcEvents) > 100 {
		report += "\n⚡ High SMC activity - Consider SMC-aware compression"
	}
	
	if len(t.inputLog) > 1000 {
		report += "\n🎮 High input activity - Interactive program detected"
	}
	
	return report
}

// PrintDetailedReport prints comprehensive debugging report
func (t *TASDebugger) PrintDetailedReport() {
	fmt.Println(t.GetRecordingStats())
	fmt.Println()
	t.hybridRecorder.PrintReport()
	fmt.Println()
	t.determinism.PrintReport()
	
	// Add cycle-perfect timing analysis
	fmt.Println("\n⏱️ CYCLE-PERFECT TIMING ANALYSIS:")
	fmt.Println("─────────────────────────────────────────────────────────────")
	compressionRatio := t.cyclePerfect.GetCompressionRatio()
	fmt.Printf("Compression Ratio:     %.0fx\n", compressionRatio)
	fmt.Printf("Events Recorded:       %d\n", len(t.cyclePerfect.events))
	
	if t.cyclePerfect.inDeterministic {
		fmt.Printf("Currently in deterministic section since cycle %d\n", 
			t.cyclePerfect.deterministicStart)
	}
}

// PGO: Enable profile collection (Quick Win #2)
func (t *TASDebugger) EnablePGO() {
	// Hook PC changes for basic block counting
	if emulator, ok := t.emulator.(interface{ SetPCHook(func(uint16)) }); ok {
		emulator.SetPCHook(func(pc uint16) {
			t.blockExecutions[pc]++
		})
	}
}

// PGO: Get profile data for compilation
func (t *TASDebugger) GetProfileData() map[string]interface{} {
	// Calculate execution threshold for hot/cold classification
	total := uint64(0)
	for _, count := range t.blockExecutions {
		total += count
	}
	threshold := total / 10  // Top 10% is "hot"
	
	profile := make(map[string]interface{})
	profile["executions"] = t.blockExecutions
	profile["branches"] = t.branchOutcomes
	profile["hot_threshold"] = threshold
	profile["smc_events"] = t.smcEvents
	
	return profile
}