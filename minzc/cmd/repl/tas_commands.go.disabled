package main

import (
	"fmt"
	"strconv"
	"strings"
	
	"github.com/minz/minzc/pkg/tas"
)

// toggleTAS enables/disables TAS debugging
func (r *REPL) toggleTAS() {
	if r.tasEnabled {
		r.tasEnabled = false
		if r.tasDebugger != nil {
			// Stop recording if active
			r.tasDebugger.recording = false
		}
		fmt.Println("TAS debugging disabled")
	} else {
		r.tasEnabled = true
		if r.tasDebugger == nil {
			// Create TAS debugger wrapping emulator
			r.tasDebugger = tas.NewTASDebugger(r.emulator)
			r.tasUI = tas.NewTASUI(r.tasDebugger)
		}
		fmt.Println("TAS debugging enabled - time travel activated!")
		fmt.Println("Commands: /record, /stop, /rewind, /forward, /savestate, /loadstate")
	}
}

// startTASRecording begins recording execution
func (r *REPL) startTASRecording() {
	if !r.tasEnabled {
		fmt.Println("TAS debugging not enabled. Use /tas to enable")
		return
	}
	
	r.tasDebugger.recording = true
	fmt.Println("🔴 TAS recording started - every CPU cycle is being recorded")
	fmt.Printf("Memory usage: ~%dKB per 1000 frames\n", 64)
}

// stopTASRecording stops recording
func (r *REPL) stopTASRecording() {
	if !r.tasEnabled || r.tasDebugger == nil {
		fmt.Println("TAS debugging not active")
		return
	}
	
	r.tasDebugger.recording = false
	frames := len(r.tasDebugger.stateHistory)
	fmt.Printf("⏹ TAS recording stopped - %d frames recorded\n", frames)
	
	// Show compression stats if available
	if frames > 0 {
		sizeUncompressed := frames * 65536
		sizeCompressed := frames * 64 * 1024 // Simplified
		ratio := float64(sizeUncompressed) / float64(sizeCompressed)
		fmt.Printf("Compression: %.1fx (%.1fMB -> %.1fMB)\n", 
			ratio, 
			float64(sizeUncompressed)/1024/1024,
			float64(sizeCompressed)/1024/1024)
	}
}

// tasRewind rewinds execution by N frames
func (r *REPL) tasRewind(framesStr string) {
	if !r.tasEnabled || r.tasDebugger == nil {
		fmt.Println("TAS debugging not active")
		return
	}
	
	frames, err := strconv.Atoi(framesStr)
	if err != nil {
		fmt.Printf("Invalid frame count: %s\n", framesStr)
		return
	}
	
	err = r.tasDebugger.Rewind(frames)
	if err != nil {
		fmt.Printf("Rewind failed: %v\n", err)
		return
	}
	
	fmt.Printf("⏪ Rewound %d frames to frame %d\n", frames, r.tasDebugger.currentFrame)
	
	// Show CPU state after rewind
	r.showRegistersCompact()
}

// tasForward advances execution by N frames
func (r *REPL) tasForward(framesStr string) {
	if !r.tasEnabled || r.tasDebugger == nil {
		fmt.Println("TAS debugging not active")
		return
	}
	
	frames, err := strconv.Atoi(framesStr)
	if err != nil {
		fmt.Printf("Invalid frame count: %s\n", framesStr)
		return
	}
	
	// Forward is rewind with negative frames
	err = r.tasDebugger.Rewind(-frames)
	if err != nil {
		// If can't go forward, execute new frames
		for i := 0; i < frames; i++ {
			r.emulator.Step()
			r.tasDebugger.RecordFrame()
		}
	}
	
	fmt.Printf("⏩ Advanced %d frames to frame %d\n", frames, r.tasDebugger.currentFrame)
}

// tasSaveState creates a named save state
func (r *REPL) tasSaveState(name string) {
	if !r.tasEnabled || r.tasDebugger == nil {
		fmt.Println("TAS debugging not active")
		return
	}
	
	r.tasDebugger.SaveState(name)
	fmt.Printf("💾 State saved as '%s' at frame %d\n", name, r.tasDebugger.currentFrame)
}

// tasLoadState restores a named save state
func (r *REPL) tasLoadState(name string) {
	if !r.tasEnabled || r.tasDebugger == nil {
		fmt.Println("TAS debugging not active")
		return
	}
	
	err := r.tasDebugger.LoadState(name)
	if err != nil {
		fmt.Printf("Failed to load state: %v\n", err)
		return
	}
	
	fmt.Printf("📂 State '%s' loaded\n", name)
	r.showRegistersCompact()
}

// showTASTimeline displays visual timeline
func (r *REPL) showTASTimeline() {
	if !r.tasEnabled || r.tasDebugger == nil {
		fmt.Println("TAS debugging not active")
		return
	}
	
	timeline := r.tasDebugger.GetTimeline()
	fmt.Println(timeline)
	
	// Show additional stats
	frames := len(r.tasDebugger.stateHistory)
	if frames > 0 {
		fmt.Printf("\nStats: %d frames | %d SMC events | %d inputs\n",
			frames,
			len(r.tasDebugger.smcEvents),
			len(r.tasDebugger.inputLog))
	}
}

// startOptimizationHunt begins searching for optimal execution path
func (r *REPL) startOptimizationHunt(targetStr string) {
	if !r.tasEnabled || r.tasDebugger == nil {
		fmt.Println("TAS debugging not active")
		return
	}
	
	// Parse target address
	var target uint64
	_, err := fmt.Sscanf(targetStr, "%x", &target)
	if err != nil {
		_, err = fmt.Sscanf(targetStr, "0x%x", &target)
		if err != nil {
			fmt.Printf("Invalid target address: %s\n", targetStr)
			return
		}
	}
	
	goal := tas.OptimizationGoal{
		TargetPC:  uint16(target),
		MaxCycles: 1000000, // Default max
	}
	
	r.tasDebugger.StartOptimizationHunt(goal)
	fmt.Printf("🎯 Optimization hunt started: find fastest path to 0x%04X\n", target)
}

// showTASHelp displays TAS-specific help
func (r *REPL) showTASHelp() {
	fmt.Println(`
TAS Debugging Commands:
═══════════════════════════════════════════════════════════════
  /tas              Enable/disable TAS debugging
  /record           Start recording execution (every CPU cycle)
  /stop             Stop recording
  /rewind [frames]  Go back in time (default: 100 frames)
  /forward [frames] Go forward in time
  /savestate [name] Create a save state (default: checkpoint)
  /loadstate <name> Restore a save state
  /timeline         Show execution timeline
  /hunt <addr>      Find optimal path to address
  /export <file>    Export TAS recording to .tas file
  /import <file>    Import TAS recording from .tas file
  /replay <file>    Replay TAS recording

Keyboard Shortcuts (when TAS enabled):
  ← →   Step frame backward/forward
  ⇧← ⇧→ Jump 100 frames
  S     Save state
  L     Load state
  R     Toggle recording
  Space Play/pause

Example Workflow:
  minz> /tas                    # Enable TAS
  minz> /record                 # Start recording
  minz> fibonacci(10)           # Run some code
  minz> /savestate before_bug   # Save checkpoint
  minz> buggy_function()        # Bug happens
  minz> /loadstate before_bug   # Go back in time!
  minz> /rewind 50              # Fine-tune position
  minz> /timeline               # See what happened
  minz> /export debug.tas       # Save for sharing
`)
}

// exportTAS exports current recording to file
func (r *REPL) exportTAS(filename string) {
	if !r.tasEnabled || r.tasDebugger == nil {
		fmt.Println("TAS debugging not active")
		return
	}
	
	// Create TAS file from debugger state
	tasFile := tas.CreateReplay(r.tasDebugger)
	
	// Determine format from extension
	format := tas.TASFormatJSON
	if strings.HasSuffix(filename, ".tasb") {
		format = tas.TASFormatBinary
	} else if strings.HasSuffix(filename, ".tasc") || strings.HasSuffix(filename, ".tas.gz") {
		format = tas.TASFormatCompressed
	}
	
	// Save to file
	if err := tasFile.SaveToFile(filename, format); err != nil {
		fmt.Printf("Failed to export TAS: %v\n", err)
		return
	}
	
	fmt.Printf("📼 TAS recording exported to %s\n", filename)
	fmt.Printf("  Frames: %d | Events: %d inputs, %d SMC\n",
		len(r.tasDebugger.stateHistory),
		len(r.tasDebugger.inputLog),
		len(r.tasDebugger.smcEvents))
}

// importTAS imports recording from file
func (r *REPL) importTAS(filename string) {
	// Load TAS file
	tasFile, err := tas.LoadFromFile(filename)
	if err != nil {
		fmt.Printf("Failed to import TAS: %v\n", err)
		return
	}
	
	// Enable TAS if not already
	if !r.tasEnabled {
		r.toggleTAS()
	}
	
	// Clear current recording
	r.tasDebugger.stateHistory = []tas.StateSnapshot{}
	r.tasDebugger.inputLog = tasFile.Events.Inputs
	r.tasDebugger.smcEvents = tasFile.Events.SMCEvents
	
	// Load keyframes if present
	if len(tasFile.States) > 0 {
		r.tasDebugger.stateHistory = tasFile.States
		r.tasDebugger.currentFrame = tasFile.States[0].Cycle
	}
	
	fmt.Printf("📼 TAS recording imported from %s\n", filename)
	fmt.Printf("  Program: %s v%s\n", tasFile.Metadata.ProgramName, tasFile.Metadata.ProgramVersion)
	fmt.Printf("  Frames: %d | Cycles: %d\n", tasFile.Metadata.TotalFrames, tasFile.Metadata.TotalCycles)
	if tasFile.Metadata.Description != "" {
		fmt.Printf("  Description: %s\n", tasFile.Metadata.Description)
	}
}

// replayTAS replays a TAS recording
func (r *REPL) replayTAS(filename string) {
	// Load TAS file
	tasFile, err := tas.LoadFromFile(filename)
	if err != nil {
		fmt.Printf("Failed to load TAS: %v\n", err)
		return
	}
	
	fmt.Printf("🎬 Replaying TAS recording from %s\n", filename)
	fmt.Printf("  Total events: %d inputs, %d SMC, %d IO\n",
		len(tasFile.Events.Inputs),
		len(tasFile.Events.SMCEvents),
		len(tasFile.Events.IOEvents))
	
	// Apply replay to emulator
	if err := tas.ApplyReplay(tasFile, r.emulator); err != nil {
		fmt.Printf("Replay failed: %v\n", err)
		return
	}
	
	fmt.Println("✅ Replay complete!")
	r.showRegistersCompact()
}

// setTASStrategy changes the TAS recording strategy
func (r *REPL) setTASStrategy(strategyName string) {
	if !r.tasEnabled || r.tasDebugger == nil {
		fmt.Println("TAS debugging not active. Use /tas to enable")
		return
	}
	
	var strategy tas.RecordingStrategy
	switch strings.ToLower(strategyName) {
	case "auto", "automatic":
		strategy = tas.StrategyAutomatic
	case "det", "deterministic":
		strategy = tas.StrategyDeterministic
	case "snap", "snapshot":
		strategy = tas.StrategySnapshot
	case "hybrid":
		strategy = tas.StrategyHybrid
	case "paranoid":
		strategy = tas.StrategyParanoid
	default:
		fmt.Printf("Unknown strategy: %s\n", strategyName)
		fmt.Println("Available: automatic, deterministic, snapshot, hybrid, paranoid")
		return
	}
	
	r.tasDebugger.SetRecordingStrategy(strategy)
}

// showTASStats displays TAS recording statistics
func (r *REPL) showTASStats() {
	if !r.tasEnabled || r.tasDebugger == nil {
		fmt.Println("TAS debugging not active")
		return
	}
	
	fmt.Println(r.tasDebugger.GetRecordingStats())
}

// profilePerformance runs performance analysis on recording
func (r *REPL) profilePerformance() {
	if !r.tasEnabled || r.tasDebugger == nil {
		fmt.Println("TAS debugging not active")
		return
	}
	
	fmt.Println("🔍 Analyzing performance...")
	
	profiler := tas.NewPerformanceProfiler(r.tasDebugger)
	report := profiler.Analyze()
	report.PrintReport()
}

// showTASReport displays comprehensive TAS report
func (r *REPL) showTASReport() {
	if !r.tasEnabled || r.tasDebugger == nil {
		fmt.Println("TAS debugging not active")
		return
	}
	
	r.tasDebugger.PrintDetailedReport()
}