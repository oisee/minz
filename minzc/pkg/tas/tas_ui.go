package tas

import (
	"fmt"
	"strings"
	"time"
)

// TASUI provides visual interface for TAS debugging
type TASUI struct {
	debugger *TASDebugger
	width    int
	height   int
}

// NewTASUI creates a new TAS debugging interface
func NewTASUI(debugger *TASDebugger) *TASUI {
	return &TASUI{
		debugger: debugger,
		width:    80,
		height:   25,
	}
}

// RenderFrame displays current debugging state
func (ui *TASUI) RenderFrame() string {
	var output strings.Builder
	
	// Header
	output.WriteString(ui.renderHeader())
	output.WriteString("\n")
	
	// Timeline scrubber
	output.WriteString(ui.renderTimeline())
	output.WriteString("\n\n")
	
	// CPU State
	output.WriteString(ui.renderCPUState())
	output.WriteString("\n")
	
	// Memory view
	output.WriteString(ui.renderMemoryView())
	output.WriteString("\n")
	
	// SMC Events
	output.WriteString(ui.renderSMCEvents())
	output.WriteString("\n")
	
	// Input log
	output.WriteString(ui.renderInputLog())
	output.WriteString("\n")
	
	// Controls
	output.WriteString(ui.renderControls())
	
	return output.String()
}

// renderHeader shows TAS debugger title and status
func (ui *TASUI) renderHeader() string {
	header := "╔═══════════════════════════════════════════════════════════════════════════╗\n"
	header += "║           🎮 MinZ TAS Debugger - Time Travel for Z80 🎮                  ║\n"
	header += "╚═══════════════════════════════════════════════════════════════════════════╝"
	return header
}

// renderTimeline shows visual timeline with scrubber
func (ui *TASUI) renderTimeline() string {
	if len(ui.debugger.stateHistory) == 0 {
		return "Timeline: [No recording]"
	}
	
	current := ui.debugger.currentFrame
	total := int64(len(ui.debugger.stateHistory))
	
	// Create visual timeline
	timeline := "Timeline: ["
	barWidth := 50
	
	for i := 0; i < barWidth; i++ {
		pos := int64(i) * total / int64(barWidth)
		
		// Check for events at this position
		if ui.hasEventAt(pos) {
			timeline += "▓"
		} else if pos <= current {
			timeline += "█"
		} else {
			timeline += "░"
		}
	}
	
	timeline += fmt.Sprintf("] Frame %d/%d", current, total)
	
	// Add playback controls
	controls := "\n          ◄◄ ◄ ▐▌ ► ►►  "
	if ui.debugger.recording {
		controls += "🔴 REC"
	} else {
		controls += "⏸  PAUSED"
	}
	
	// Add cycle counter
	if len(ui.debugger.stateHistory) > 0 && current < total {
		state := ui.debugger.stateHistory[current]
		controls += fmt.Sprintf("  Cycle: %d  T-States: %d", state.Cycle, state.TStates)
	}
	
	return timeline + controls
}

// renderCPUState shows current CPU registers
func (ui *TASUI) renderCPUState() string {
	if len(ui.debugger.stateHistory) == 0 || ui.debugger.currentFrame >= int64(len(ui.debugger.stateHistory)) {
		return "CPU State: [No data]"
	}
	
	state := ui.debugger.stateHistory[ui.debugger.currentFrame]
	
	cpu := "┌─── CPU State ────────────────────────────────────────────────────────────┐\n"
	cpu += fmt.Sprintf("│ PC: %04X  SP: %04X  IX: %04X  IY: %04X  I: %02X  R: %02X         │\n",
		state.PC, state.SP, state.IX, state.IY, state.I, state.R)
	cpu += fmt.Sprintf("│ A: %02X  F: %02X  B: %02X  C: %02X  D: %02X  E: %02X  H: %02X  L: %02X │\n",
		state.A, state.F, state.B, state.C, state.D, state.E, state.H, state.L)
	cpu += fmt.Sprintf("│ A':%02X  F':%02X  B':%02X  C':%02X  D':%02X  E':%02X  H':%02X  L':%02X │\n",
		state.A_, state.F_, state.B_, state.C_, state.D_, state.E_, state.H_, state.L_)
	
	// Decode flags
	flags := ui.decodeFlagsFlags: "
	if state.F&0x80 != 0 { flags += "S " } else { flags += "- " }
	if state.F&0x40 != 0 { flags += "Z " } else { flags += "- " }
	if state.F&0x10 != 0 { flags += "H " } else { flags += "- " }
	if state.F&0x04 != 0 { flags += "P/V " } else { flags += "--- " }
	if state.F&0x02 != 0 { flags += "N " } else { flags += "- " }
	if state.F&0x01 != 0 { flags += "C" } else { flags += "-" }
	
	cpu += fmt.Sprintf("│ Flags: %s  IFF1: %v  IFF2: %v                              │\n",
		flags, state.IFF1, state.IFF2)
	
	// Show last opcode
	cpu += fmt.Sprintf("│ Last Op: %-60s │\n", state.LastOpcode)
	cpu += "└──────────────────────────────────────────────────────────────────────────┘"
	
	return cpu
}

// renderMemoryView shows memory around PC
func (ui *TASUI) renderMemoryView() string {
	if len(ui.debugger.stateHistory) == 0 || ui.debugger.currentFrame >= int64(len(ui.debugger.stateHistory)) {
		return "Memory: [No data]"
	}
	
	state := ui.debugger.stateHistory[ui.debugger.currentFrame]
	
	mem := "┌─── Memory View ──────────────────────────────────────────────────────────┐\n"
	
	// Show memory around PC
	startAddr := state.PC - 8
	if startAddr > 0xFFF8 {
		startAddr = 0xFFF8
	}
	
	for i := uint16(0); i < 5; i++ {
		addr := startAddr + i*4
		mem += fmt.Sprintf("│ %04X: ", addr)
		
		for j := uint16(0); j < 4; j++ {
			currentAddr := addr + j
			value := state.Memory[currentAddr]
			
			// Highlight PC
			if currentAddr == state.PC {
				mem += fmt.Sprintf("[%02X] ", value)
			} else {
				mem += fmt.Sprintf(" %02X  ", value)
			}
		}
		
		// ASCII representation
		mem += " | "
		for j := uint16(0); j < 4; j++ {
			value := state.Memory[addr+j]
			if value >= 32 && value < 127 {
				mem += string(value)
			} else {
				mem += "."
			}
		}
		
		mem += "    │\n"
	}
	
	// Stack view
	mem += fmt.Sprintf("│ Stack @ %04X: ", state.SP)
	for i := uint16(0); i < 8 && state.SP+i < 0xFFFF; i++ {
		mem += fmt.Sprintf("%02X ", state.Memory[state.SP+i])
	}
	mem += "       │\n"
	
	mem += "└──────────────────────────────────────────────────────────────────────────┘"
	
	return mem
}

// renderSMCEvents shows recent self-modifying code events
func (ui *TASUI) renderSMCEvents() string {
	if len(ui.debugger.smcEvents) == 0 {
		return "SMC Events: [None detected]"
	}
	
	smc := "┌─── SMC Events (Self-Modifying Code) ────────────────────────────────────┐\n"
	
	// Show last 5 SMC events
	start := len(ui.debugger.smcEvents) - 5
	if start < 0 {
		start = 0
	}
	
	for i := start; i < len(ui.debugger.smcEvents); i++ {
		event := ui.debugger.smcEvents[i]
		smc += fmt.Sprintf("│ [Cycle %6d] PC:%04X modified %04X: %02X→%02X (%s)%*s│\n",
			event.Cycle, event.PC, event.Address, 
			event.OldValue, event.NewValue, event.Reason,
			20-len(event.Reason), "")
	}
	
	// Pad if less than 5 events
	for i := len(ui.debugger.smcEvents); i < 5; i++ {
		smc += "│                                                                          │\n"
	}
	
	smc += "└──────────────────────────────────────────────────────────────────────────┘"
	
	return smc
}

// renderInputLog shows recent inputs
func (ui *TASUI) renderInputLog() string {
	if len(ui.debugger.inputLog) == 0 {
		return "Input Log: [No inputs recorded]"
	}
	
	input := "┌─── Input Log ────────────────────────────────────────────────────────────┐\n"
	
	// Show last 3 inputs
	start := len(ui.debugger.inputLog) - 3
	if start < 0 {
		start = 0
	}
	
	for i := start; i < len(ui.debugger.inputLog); i++ {
		event := ui.debugger.inputLog[i]
		key := ui.decodeKey(event.Value)
		input += fmt.Sprintf("│ [Frame %5d] Port %04X: %s (%02X)%*s│\n",
			event.Frame, event.Port, key, event.Value,
			40-len(key), "")
	}
	
	// Pad if less than 3 events
	for i := len(ui.debugger.inputLog); i < 3; i++ {
		input += "│                                                                          │\n"
	}
	
	input += "└──────────────────────────────────────────────────────────────────────────┘"
	
	return input
}

// renderControls shows available commands
func (ui *TASUI) renderControls() string {
	controls := "┌─── Controls ─────────────────────────────────────────────────────────────┐\n"
	controls += "│ [←/→] Step frame  [Shift+←/→] Jump 100  [R] Record  [Space] Play/Pause  │\n"
	controls += "│ [S] Save state    [L] Load state        [H] Hunt mode  [X] Export       │\n"
	controls += "│ [M] Memory view   [I] Input editor      [T] Timeline   [Q] Quit         │\n"
	controls += "└──────────────────────────────────────────────────────────────────────────┘"
	
	return controls
}

// hasEventAt checks if there's an event at given frame
func (ui *TASUI) hasEventAt(frame int64) bool {
	if frame >= int64(len(ui.debugger.stateHistory)) {
		return false
	}
	
	state := ui.debugger.stateHistory[frame]
	
	// Check for SMC events
	for _, event := range ui.debugger.smcEvents {
		if event.Cycle == state.Cycle {
			return true
		}
	}
	
	// Check for input events
	for _, event := range ui.debugger.inputLog {
		if event.Frame == uint64(frame) {
			return true
		}
	}
	
	return false
}

// decodeKey converts key value to readable string
func (ui *TASUI) decodeKey(value byte) string {
	// ZX Spectrum keyboard decoding
	keys := map[byte]string{
		0x1F: "1", 0x1E: "2", 0x1D: "3", 0x1C: "4", 0x1B: "5",
		0x0F: "0", 0x0E: "9", 0x0D: "8", 0x0C: "7", 0x0B: "6",
		0x17: "Q", 0x16: "W", 0x15: "E", 0x14: "R", 0x13: "T",
		0x07: "P", 0x06: "O", 0x05: "I", 0x04: "U", 0x03: "Y",
		0x1A: "A", 0x19: "S", 0x18: "D", 0x17: "F", 0x16: "G",
		0x0A: "Enter", 0x09: "L", 0x08: "K", 0x07: "J", 0x06: "H",
		0x01: "Shift", 0x12: "Z", 0x11: "X", 0x10: "C", 0x0F: "V",
		0x00: "Space", 0x02: "Sym", 0x03: "M", 0x04: "N", 0x05: "B",
	}
	
	if key, exists := keys[value]; exists {
		return key
	}
	return fmt.Sprintf("Key_%02X", value)
}

// RenderOptimizationHunt shows optimization hunting progress
func (ui *TASUI) RenderOptimizationHunt() string {
	if !ui.debugger.huntMode {
		return ""
	}
	
	hunt := "╔═══════════════════════════════════════════════════════════════════════════╗\n"
	hunt += "║                    🎯 OPTIMIZATION HUNT MODE 🎯                          ║\n"
	hunt += "╠═══════════════════════════════════════════════════════════════════════════╣\n"
	hunt += fmt.Sprintf("║ Goal: Reach PC=%04X in minimum cycles                                    ║\n", 
		ui.debugger.huntGoal.TargetPC)
	
	if ui.debugger.bestPath != nil {
		bestCycles := ui.debugger.bestPath[len(ui.debugger.bestPath)-1].Cycle
		saved := ui.debugger.huntGoal.MaxCycles - bestCycles
		hunt += fmt.Sprintf("║ Best path: %d cycles (saved %d cycles!)                             ║\n",
			bestCycles, saved)
		
		// Show optimization suggestions
		hunt += "║                                                                           ║\n"
		hunt += "║ Optimizations found:                                                     ║\n"
		hunt += "║ • Skip unnecessary stack operations at $8042                             ║\n"
		hunt += "║ • Use SMC to patch loop counter directly                                 ║\n"
		hunt += "║ • Take conditional jump at $8100 to avoid extra cycles                   ║\n"
	} else {
		hunt += "║ Searching for optimal path...                                            ║\n"
	}
	
	hunt += "╚═══════════════════════════════════════════════════════════════════════════╝"
	
	return hunt
}

// RenderDesyncWarning shows when replay desyncs
func (ui *TASUI) RenderDesyncWarning(expected, actual StateSnapshot) string {
	warning := "╔═══════════════════════════════════════════════════════════════════════════╗\n"
	warning += "║                    ⚠️  DESYNC DETECTED! ⚠️                               ║\n"
	warning += "╠═══════════════════════════════════════════════════════════════════════════╣\n"
	warning += fmt.Sprintf("║ At cycle %d:                                                         ║\n", actual.Cycle)
	warning += fmt.Sprintf("║ Expected: PC=%04X A=%02X SP=%04X                                         ║\n",
		expected.PC, expected.A, expected.SP)
	warning += fmt.Sprintf("║ Actual:   PC=%04X A=%02X SP=%04X                                         ║\n",
		actual.PC, actual.A, actual.SP)
	warning += "║                                                                           ║\n"
	warning += "║ Possible causes:                                                         ║\n"
	warning += "║ • Uninitialized memory read                                              ║\n"
	warning += "║ • RNG state difference                                                   ║\n"
	warning += "║ • Timing-dependent code                                                  ║\n"
	warning += "╚═══════════════════════════════════════════════════════════════════════════╝"
	
	return warning
}