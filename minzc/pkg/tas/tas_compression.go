package tas

import (
	"fmt"
)

// TASCompression implements smart compression for deterministic execution
type TASCompression struct {
	debugger *TASDebugger
	
	// Determinism tracking
	isDeterministic   bool
	lastInputCycle    uint64
	lastRandCycle     uint64
	lastIOCycle       uint64
	
	// Compression strategy
	keyframeInterval  int     // Save full state every N frames
	deltaFrames       []DeltaFrame  // Only changes between frames
	keyframes         map[int64]*StateSnapshot
}

// DeltaFrame stores only changes from previous frame
type DeltaFrame struct {
	Cycle    uint64
	Changes  []StateChange
}

// StateChange represents a single change in state
type StateChange struct {
	Type     ChangeType
	Address  uint16  // For memory changes
	OldValue byte
	NewValue byte
	Register string  // For register changes
}

type ChangeType int

const (
	ChangeMemory ChangeType = iota
	ChangeRegister
	ChangePC
	ChangeSP
	ChangeFlags
)

// NewTASCompression creates smart compression system
func NewTASCompression(debugger *TASDebugger) *TASCompression {
	return &TASCompression{
		debugger:         debugger,
		isDeterministic:  true,
		keyframeInterval: 1000,  // Save full state every 1000 frames
		deltaFrames:      make([]DeltaFrame, 0, 10000),
		keyframes:        make(map[int64]*StateSnapshot),
	}
}

// AnalyzeDeterminism checks if execution is deterministic
func (c *TASCompression) AnalyzeDeterminism(state StateSnapshot) {
	// Check for non-deterministic operations
	
	// 1. IN/OUT instructions (ports)
	opcode := state.Memory[state.PC]
	if c.isIOInstruction(opcode) {
		c.isDeterministic = false
		c.lastIOCycle = state.Cycle
	}
	
	// 2. R register access (used for refresh, sometimes as random)
	if c.isRRegisterAccess(opcode) {
		c.isDeterministic = false
		c.lastRandCycle = state.Cycle
	}
	
	// 3. Interrupt handling (timing-dependent)
	if state.IFF1 || state.IFF2 {
		// Interrupts can make execution non-deterministic
		c.isDeterministic = false
	}
}

// CompressFrame intelligently compresses based on determinism
func (c *TASCompression) CompressFrame(current, previous StateSnapshot) {
	c.AnalyzeDeterminism(current)
	
	// If deterministic and not a keyframe, only store deltas
	if c.isDeterministic && current.Frame % uint64(c.keyframeInterval) != 0 {
		delta := c.computeDelta(current, previous)
		c.deltaFrames = append(c.deltaFrames, delta)
	} else {
		// Store full keyframe
		c.keyframes[int64(current.Frame)] = &current
	}
}

// computeDelta calculates changes between states
func (c *TASCompression) computeDelta(current, previous StateSnapshot) DeltaFrame {
	delta := DeltaFrame{
		Cycle:   current.Cycle,
		Changes: make([]StateChange, 0),
	}
	
	// Check PC change
	if current.PC != previous.PC {
		delta.Changes = append(delta.Changes, StateChange{
			Type:     ChangePC,
			OldValue: byte(previous.PC & 0xFF),
			NewValue: byte(current.PC & 0xFF),
		})
	}
	
	// Check register changes (optimize: only check modified ones)
	if current.A != previous.A {
		delta.Changes = append(delta.Changes, StateChange{
			Type:     ChangeRegister,
			Register: "A",
			OldValue: previous.A,
			NewValue: current.A,
		})
	}
	
	// Memory changes (smart: only track modified areas)
	// In real implementation, track write operations
	for addr := uint16(0); addr < 100; addr++ { // Simplified - check hot memory areas
		if current.Memory[addr] != previous.Memory[addr] {
			delta.Changes = append(delta.Changes, StateChange{
				Type:     ChangeMemory,
				Address:  addr,
				OldValue: previous.Memory[addr],
				NewValue: current.Memory[addr],
			})
		}
	}
	
	return delta
}

// ReconstructState rebuilds state from keyframe + deltas
func (c *TASCompression) ReconstructState(targetFrame int64) (*StateSnapshot, error) {
	// Find nearest keyframe before target
	nearestKeyframe := int64(0)
	for frame := range c.keyframes {
		if frame <= targetFrame && frame > nearestKeyframe {
			nearestKeyframe = frame
		}
	}
	
	if nearestKeyframe == 0 {
		return nil, fmt.Errorf("no keyframe found before frame %d", targetFrame)
	}
	
	// Start from keyframe
	state := *c.keyframes[nearestKeyframe]
	
	// Apply deltas up to target frame
	for _, delta := range c.deltaFrames {
		if delta.Cycle > state.Cycle && int64(delta.Cycle) <= uint64(targetFrame) {
			c.applyDelta(&state, delta)
		}
	}
	
	return &state, nil
}

// applyDelta applies changes to state
func (c *TASCompression) applyDelta(state *StateSnapshot, delta DeltaFrame) {
	for _, change := range delta.Changes {
		switch change.Type {
		case ChangePC:
			state.PC = uint16(change.NewValue) // Simplified
		case ChangeRegister:
			switch change.Register {
			case "A":
				state.A = change.NewValue
			case "B":
				state.B = change.NewValue
			// ... other registers
			}
		case ChangeMemory:
			state.Memory[change.Address] = change.NewValue
		}
	}
}

// GetCompressionStats returns compression efficiency
func (c *TASCompression) GetCompressionStats() CompressionStats {
	totalFrames := len(c.deltaFrames) + len(c.keyframes)
	keyframeSize := len(c.keyframes) * 65536  // Full state
	
	deltaSize := 0
	for _, delta := range c.deltaFrames {
		deltaSize += len(delta.Changes) * 4  // Approximate bytes per change
	}
	
	uncompressedSize := totalFrames * 65536
	compressedSize := keyframeSize + deltaSize
	
	return CompressionStats{
		TotalFrames:      totalFrames,
		Keyframes:        len(c.keyframes),
		DeltaFrames:      len(c.deltaFrames),
		UncompressedSize: uncompressedSize,
		CompressedSize:   compressedSize,
		CompressionRatio: float64(uncompressedSize) / float64(compressedSize),
		IsDeterministic:  c.isDeterministic,
	}
}

// CompressionStats shows compression efficiency
type CompressionStats struct {
	TotalFrames      int
	Keyframes        int
	DeltaFrames      int
	UncompressedSize int
	CompressedSize   int
	CompressionRatio float64
	IsDeterministic  bool
}

// isIOInstruction checks if opcode is IN/OUT
func (c *TASCompression) isIOInstruction(opcode byte) bool {
	// Z80 IN/OUT opcodes
	return opcode == 0xDB || // IN A,(n)
		   opcode == 0xD3 || // OUT (n),A
		   opcode == 0xED    // Extended opcodes (IN r,(C), OUT (C),r)
}

// isRRegisterAccess checks if R register is accessed
func (c *TASCompression) isRRegisterAccess(opcode byte) bool {
	// LD A,R (ED 5F) or LD R,A (ED 4F)
	// Need to check two-byte opcodes
	return false // Simplified - would need to check next byte
}

// SmartRewind uses compression knowledge for fast rewind
func (c *TASCompression) SmartRewind(targetFrame int64) error {
	if c.isDeterministic {
		// Can jump directly to keyframe + replay deltas
		state, err := c.ReconstructState(targetFrame)
		if err != nil {
			return err
		}
		c.debugger.restoreState(state)
		return nil
	} else {
		// Need to replay from last non-deterministic point
		// Use regular frame-by-frame replay
		return c.debugger.Rewind(int(c.debugger.currentFrame - targetFrame))
	}
}

// PredictiveRecord only saves when necessary
func (c *TASCompression) PredictiveRecord(state StateSnapshot) {
	// During deterministic execution, save less frequently
	if c.isDeterministic {
		// Only save on:
		// 1. Keyframe intervals
		// 2. Before non-deterministic operations
		// 3. At branch points (conditional jumps)
		
		opcode := state.Memory[state.PC]
		
		// Is this a branch point?
		if c.isBranchInstruction(opcode) {
			// Save state before branch for quick rewind
			c.keyframes[int64(state.Frame)] = &state
		} else if state.Frame % uint64(c.keyframeInterval) == 0 {
			// Regular keyframe
			c.keyframes[int64(state.Frame)] = &state
		}
		// Otherwise, deltas are enough!
	} else {
		// Non-deterministic - save every frame
		c.debugger.stateHistory = append(c.debugger.stateHistory, state)
	}
}

// isBranchInstruction checks for conditional jumps
func (c *TASCompression) isBranchInstruction(opcode byte) bool {
	// Z80 conditional jumps
	return (opcode >= 0x20 && opcode <= 0x38 && opcode%8 == 0) || // JR cc,e
		   (opcode >= 0xC2 && opcode <= 0xDA && (opcode-0xC2)%8 < 4) || // JP cc,nn
		   (opcode >= 0xC4 && opcode <= 0xDC && (opcode-0xC4)%8 < 4)    // CALL cc,nn
}

// RenderCompressionStats shows compression efficiency
func (stats CompressionStats) Render() string {
	output := "╔═══════════════════════════════════════════════════════════════════╗\n"
	output += "║                 TAS COMPRESSION STATISTICS                       ║\n"
	output += "╠═══════════════════════════════════════════════════════════════════╣\n"
	
	if stats.IsDeterministic {
		output += "║ Execution Mode: DETERMINISTIC ✅                                 ║\n"
	} else {
		output += "║ Execution Mode: NON-DETERMINISTIC ⚠️                             ║\n"
	}
	
	output += fmt.Sprintf("║ Total Frames:    %-10d                                      ║\n", stats.TotalFrames)
	output += fmt.Sprintf("║ Keyframes:       %-10d (every %d frames)                  ║\n", 
		stats.Keyframes, stats.TotalFrames/stats.Keyframes)
	output += fmt.Sprintf("║ Delta Frames:    %-10d                                      ║\n", stats.DeltaFrames)
	output += "║                                                                   ║\n"
	
	// Size comparison
	output += fmt.Sprintf("║ Uncompressed:    %10d bytes (%.1f MB)                    ║\n", 
		stats.UncompressedSize, float64(stats.UncompressedSize)/1024/1024)
	output += fmt.Sprintf("║ Compressed:      %10d bytes (%.1f MB)                    ║\n",
		stats.CompressedSize, float64(stats.CompressedSize)/1024/1024)
	output += fmt.Sprintf("║ Compression:     %.1fx (%.1f%% space saved)                      ║\n",
		stats.CompressionRatio, (1.0 - 1.0/stats.CompressionRatio) * 100)
	
	// Smart suggestions
	output += "║                                                                   ║\n"
	if stats.IsDeterministic {
		output += "║ 💡 TIP: Deterministic execution detected!                        ║\n"
		output += "║    Only keyframes needed - massive space savings!                ║\n"
		output += "║    Time travel is instant via delta replay!                      ║\n"
	} else {
		output += "║ ⚠️  WARNING: Non-deterministic operations detected!              ║\n"
		output += "║    Full frame recording required for accuracy                    ║\n"
		output += "║    Consider isolating I/O for better compression                 ║\n"
	}
	
	output += "╚═══════════════════════════════════════════════════════════════════╝"
	
	return output
}