package tas

import (
	"fmt"
)

// RecordingStrategy defines how to handle different situations
type RecordingStrategy int

const (
	StrategyAutomatic RecordingStrategy = iota // Automatically choose best approach
	StrategyDeterministic                       // Always use deterministic replay
	StrategySnapshot                            // Always use snapshots
	StrategyHybrid                              // Mix based on context
	StrategyParanoid                            // Maximum reliability (more snapshots)
)

// HybridRecorder combines deterministic and snapshot approaches
type HybridRecorder struct {
	strategy RecordingStrategy
	
	// Core components
	cycleRecorder *CyclePerfectRecorder
	determinism   *DeterminismDetector
	snapshots     *SnapshotManager
	
	// Decision making
	lastSnapshot      int64
	snapshotInterval  int64
	ioEventCount      int
	uncertaintyLevel  float64
	
	// Configuration
	config RecorderConfig
	
	// Statistics
	stats RecorderStats
}

// RecorderConfig configures the hybrid recorder
type RecorderConfig struct {
	Strategy            RecordingStrategy
	SnapshotOnIO        bool    // Take snapshot on I/O events
	IOSnapshotThreshold int     // Number of I/O events before snapshot
	MaxDeterministic    int64   // Max cycles without snapshot
	MinSnapshotInterval int64   // Minimum cycles between snapshots
	UncertaintyTolerance float64 // When to force snapshot (0.0-1.0)
	ParanoidMode        bool    // Extra safety with more snapshots
}

// RecorderStats tracks recording statistics
type RecorderStats struct {
	TotalCycles          int64
	SnapshotCount        int
	EventCount           int
	DeterministicRatio   float64
	CompressionRatio     float64
	MemoryUsed           int64
	SnapshotMemory       int64
	EventMemory          int64
}

// SnapshotManager manages state snapshots
type SnapshotManager struct {
	snapshots        []TimedSnapshot
	keyframes        map[int64]*StateSnapshot
	maxSnapshots     int
	compressionLevel int
}

// TimedSnapshot is a snapshot with timing info
type TimedSnapshot struct {
	Snapshot     StateSnapshot
	Cycle        int64
	Reason       SnapshotReason
	Compressed   bool
	CompressedData []byte
}

// SnapshotReason explains why snapshot was taken
type SnapshotReason byte

const (
	ReasonInterval   SnapshotReason = iota // Regular interval
	ReasonIO                                // I/O event occurred
	ReasonUncertain                         // High uncertainty
	ReasonSMC                               // Self-modifying code
	ReasonUser                              // User requested
	ReasonKeyframe                          // Keyframe for seeking
	ReasonParanoid                          // Paranoid mode safety
)

// NewHybridRecorder creates a new hybrid recorder
func NewHybridRecorder(config RecorderConfig) *HybridRecorder {
	// Set defaults
	if config.MaxDeterministic == 0 {
		config.MaxDeterministic = 100000 // 100k cycles max without snapshot
	}
	if config.MinSnapshotInterval == 0 {
		config.MinSnapshotInterval = 1000 // At least 1000 cycles between snapshots
	}
	if config.IOSnapshotThreshold == 0 {
		config.IOSnapshotThreshold = 10 // Snapshot after 10 I/O events
	}
	if config.UncertaintyTolerance == 0 {
		config.UncertaintyTolerance = 0.3 // 30% uncertainty triggers snapshot
	}
	
	return &HybridRecorder{
		strategy:         config.Strategy,
		cycleRecorder:    NewCyclePerfectRecorder(),
		determinism:      NewDeterminismDetector(),
		snapshots:        NewSnapshotManager(),
		snapshotInterval: 10000, // Default 10k cycles
		config:          config,
	}
}

// RecordCycle records a single CPU cycle with smart decision making
func (r *HybridRecorder) RecordCycle(cycle int64, state *StateSnapshot, event *CycleEvent) {
	r.stats.TotalCycles = cycle
	
	// Always record the event
	if event != nil {
		r.cycleRecorder.RecordInstruction(
			event.Data.(InstructionData).Opcode,
			event.Data.(InstructionData).PC,
			event.TStates,
		)
		r.determinism.ProcessEvent(*event)
		r.stats.EventCount++
	}
	
	// Decide whether to take a snapshot
	shouldSnapshot := r.shouldTakeSnapshot(cycle, event)
	
	if shouldSnapshot {
		reason := r.determineSnapshotReason(cycle, event)
		r.takeSnapshot(state, cycle, reason)
	}
}

// shouldTakeSnapshot decides if a snapshot is needed
func (r *HybridRecorder) shouldTakeSnapshot(cycle int64, event *CycleEvent) bool {
	// Check minimum interval
	if cycle - r.lastSnapshot < r.config.MinSnapshotInterval {
		return false
	}
	
	switch r.config.Strategy {
	case StrategySnapshot:
		// Always snapshot at intervals
		return cycle - r.lastSnapshot >= r.snapshotInterval
		
	case StrategyDeterministic:
		// Only snapshot on I/O or uncertainty
		if event != nil {
			switch event.Type {
			case EventIORead, EventIOWrite:
				if r.config.SnapshotOnIO {
					return true
				}
				r.ioEventCount++
				return r.ioEventCount >= r.config.IOSnapshotThreshold
				
			case EventInterrupt:
				return true // Always snapshot on interrupts
				
			case EventSMC:
				// SMC might need snapshot for safety
				return r.uncertaintyLevel > 0.5
			}
		}
		
		// Check if we've gone too long without snapshot
		return cycle - r.lastSnapshot >= r.config.MaxDeterministic
		
	case StrategyHybrid, StrategyAutomatic:
		// Smart decision based on context
		if event != nil {
			switch event.Type {
			case EventIORead, EventIOWrite:
				r.ioEventCount++
				// Snapshot if I/O is frequent
				if r.config.SnapshotOnIO || r.ioEventCount >= r.config.IOSnapshotThreshold {
					return true
				}
				
			case EventInterrupt:
				return true
				
			case EventSMC:
				// Calculate uncertainty based on SMC frequency
				r.uncertaintyLevel += 0.1
				if r.uncertaintyLevel > r.config.UncertaintyTolerance {
					return true
				}
			}
		}
		
		// Regular interval check
		if cycle - r.lastSnapshot >= r.snapshotInterval {
			// Adjust interval based on determinism
			stats := r.determinism.GetStatistics()
			if stats.Ratio > 0.8 {
				// Very deterministic - increase interval
				r.snapshotInterval = min(r.snapshotInterval * 2, r.config.MaxDeterministic)
			} else if stats.Ratio < 0.3 {
				// Not deterministic - decrease interval
				r.snapshotInterval = max(r.snapshotInterval / 2, r.config.MinSnapshotInterval)
			}
			return true
		}
		
	case StrategyParanoid:
		// Take lots of snapshots for maximum safety
		if event != nil {
			switch event.Type {
			case EventIORead, EventIOWrite, EventInterrupt, EventSMC:
				return true // Snapshot on any non-deterministic event
			}
		}
		// Also snapshot frequently
		return cycle - r.lastSnapshot >= r.config.MinSnapshotInterval * 5
	}
	
	return false
}

// determineSnapshotReason figures out why we're taking a snapshot
func (r *HybridRecorder) determineSnapshotReason(cycle int64, event *CycleEvent) SnapshotReason {
	if r.config.ParanoidMode {
		return ReasonParanoid
	}
	
	if event != nil {
		switch event.Type {
		case EventIORead, EventIOWrite:
			return ReasonIO
		case EventInterrupt:
			return ReasonIO // Interrupts are like I/O
		case EventSMC:
			return ReasonSMC
		}
	}
	
	if r.uncertaintyLevel > r.config.UncertaintyTolerance {
		return ReasonUncertain
	}
	
	if cycle - r.lastSnapshot >= r.snapshotInterval {
		// Check if this is a keyframe
		if cycle % (r.snapshotInterval * 10) == 0 {
			return ReasonKeyframe
		}
		return ReasonInterval
	}
	
	return ReasonUser
}

// takeSnapshot captures current state
func (r *HybridRecorder) takeSnapshot(state *StateSnapshot, cycle int64, reason SnapshotReason) {
	snapshot := TimedSnapshot{
		Snapshot: *state,
		Cycle:    cycle,
		Reason:   reason,
	}
	
	// Compress if not a keyframe
	if reason != ReasonKeyframe && reason != ReasonIO {
		snapshot.Compressed = true
		snapshot.CompressedData = compressSnapshot(state)
		r.stats.SnapshotMemory += int64(len(snapshot.CompressedData))
	} else {
		r.stats.SnapshotMemory += 65536 + 32 // Full snapshot size
	}
	
	r.snapshots.Add(snapshot)
	r.lastSnapshot = cycle
	r.ioEventCount = 0
	r.uncertaintyLevel *= 0.5 // Reduce uncertainty after snapshot
	
	r.stats.SnapshotCount++
	
	// Log snapshot decision
	if debugMode {
		fmt.Printf("📸 Snapshot at cycle %d (reason: %s)\n", cycle, reasonString(reason))
	}
}

// GetReplayStrategy returns the best replay strategy for a section
func (r *HybridRecorder) GetReplayStrategy(startCycle, endCycle int64) ReplayStrategy {
	// Find snapshots in range
	snapshots := r.snapshots.GetRange(startCycle, endCycle)
	
	// Get determinism info
	stats := r.determinism.GetStatistics()
	
	// Decide replay strategy
	if len(snapshots) == 0 {
		// No snapshots - must use deterministic replay
		return ReplayStrategy{
			Type:        ReplayDeterministic,
			StartCycle:  startCycle,
			EndCycle:    endCycle,
			Confidence:  stats.Ratio,
		}
	}
	
	// Find closest snapshot
	closest := r.snapshots.GetClosest(startCycle)
	distance := startCycle - closest.Cycle
	
	if distance < 1000 {
		// Snapshot is very close - use it
		return ReplayStrategy{
			Type:           ReplayFromSnapshot,
			SnapshotCycle:  closest.Cycle,
			StartCycle:     startCycle,
			EndCycle:       endCycle,
			Confidence:     0.99,
		}
	}
	
	// Check determinism between snapshot and target
	sectionStats := r.determinism.GetSectionStats(closest.Cycle, startCycle)
	
	if sectionStats.Ratio > 0.9 {
		// Highly deterministic - safe to replay from snapshot
		return ReplayStrategy{
			Type:           ReplayFromSnapshot,
			SnapshotCycle:  closest.Cycle,
			StartCycle:     startCycle,
			EndCycle:       endCycle,
			Confidence:     sectionStats.Ratio,
		}
	}
	
	// Mixed approach - use snapshot but verify
	return ReplayStrategy{
		Type:           ReplayHybrid,
		SnapshotCycle:  closest.Cycle,
		StartCycle:     startCycle,
		EndCycle:       endCycle,
		Confidence:     sectionStats.Ratio,
		VerifyPoints:   r.getVerificationPoints(closest.Cycle, startCycle),
	}
}

// ReplayStrategy describes how to replay a section
type ReplayStrategy struct {
	Type           ReplayType
	SnapshotCycle  int64
	StartCycle     int64
	EndCycle       int64
	Confidence     float64
	VerifyPoints   []int64 // Cycles to verify during replay
}

type ReplayType byte

const (
	ReplayDeterministic ReplayType = iota // Pure event-based replay
	ReplayFromSnapshot                     // Start from snapshot
	ReplayHybrid                           // Mix of both
	ReplayVerified                         // With verification
)

// getVerificationPoints returns cycles where state should be verified
func (r *HybridRecorder) getVerificationPoints(start, end int64) []int64 {
	points := []int64{}
	
	// Add I/O event cycles as verification points
	for _, event := range r.cycleRecorder.events {
		if event.Cycle >= start && event.Cycle <= end {
			if event.Type == EventIORead || event.Type == EventIOWrite {
				points = append(points, event.Cycle)
			}
		}
	}
	
	return points
}

// GetStatistics returns recording statistics
func (r *HybridRecorder) GetStatistics() RecorderStats {
	r.stats.DeterministicRatio = r.determinism.GetStatistics().Ratio
	r.stats.EventMemory = int64(r.stats.EventCount * 8)
	r.stats.MemoryUsed = r.stats.SnapshotMemory + r.stats.EventMemory
	
	// Calculate compression ratio
	uncompressed := r.stats.TotalCycles * 67
	r.stats.CompressionRatio = float64(uncompressed) / float64(r.stats.MemoryUsed)
	
	return r.stats
}

// PrintReport prints a detailed recording report
func (r *HybridRecorder) PrintReport() {
	stats := r.GetStatistics()
	
	fmt.Println("\n📊 HYBRID RECORDING REPORT")
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Printf("Strategy:            %s\n", strategyString(r.config.Strategy))
	fmt.Printf("Total Cycles:        %d\n", stats.TotalCycles)
	fmt.Printf("Snapshots Taken:     %d\n", stats.SnapshotCount)
	fmt.Printf("Events Recorded:     %d\n", stats.EventCount)
	fmt.Printf("Deterministic Ratio: %.1f%%\n", stats.DeterministicRatio*100)
	
	fmt.Println("\n💾 MEMORY USAGE:")
	fmt.Println("───────────────────────────────────────────────────────")
	fmt.Printf("Snapshots:           %.1f MB\n", float64(stats.SnapshotMemory)/1024/1024)
	fmt.Printf("Events:              %.1f MB\n", float64(stats.EventMemory)/1024/1024)
	fmt.Printf("Total:               %.1f MB\n", float64(stats.MemoryUsed)/1024/1024)
	fmt.Printf("Compression Ratio:   %.0fx\n", stats.CompressionRatio)
	
	// Strategy effectiveness
	fmt.Println("\n🎯 STRATEGY EFFECTIVENESS:")
	fmt.Println("───────────────────────────────────────────────────────")
	
	avgSnapshotInterval := float64(stats.TotalCycles) / float64(max(stats.SnapshotCount, 1))
	fmt.Printf("Avg Snapshot Interval: %.0f cycles\n", avgSnapshotInterval)
	
	if r.config.Strategy == StrategyAutomatic || r.config.Strategy == StrategyHybrid {
		fmt.Printf("Current Interval:      %d cycles\n", r.snapshotInterval)
		fmt.Printf("Uncertainty Level:     %.1f%%\n", r.uncertaintyLevel*100)
		
		if stats.DeterministicRatio > 0.8 {
			fmt.Println("✅ Strategy working well - high determinism detected")
		} else if stats.DeterministicRatio < 0.3 {
			fmt.Println("⚠️  Consider more snapshots - low determinism")
		}
	}
	
	// Recommendations
	fmt.Println("\n💡 RECOMMENDATIONS:")
	fmt.Println("───────────────────────────────────────────────────────")
	
	if stats.SnapshotCount > stats.TotalCycles/1000 {
		fmt.Println("• Too many snapshots - increase MinSnapshotInterval")
	}
	if stats.DeterministicRatio > 0.9 && r.config.Strategy != StrategyDeterministic {
		fmt.Println("• High determinism - consider StrategyDeterministic")
	}
	if stats.DeterministicRatio < 0.5 && r.config.Strategy != StrategySnapshot {
		fmt.Println("• Low determinism - consider StrategySnapshot")
	}
	if r.ioEventCount > 100 {
		fmt.Println("• High I/O activity - enable SnapshotOnIO")
	}
}

// NewSnapshotManager creates a new snapshot manager
func NewSnapshotManager() *SnapshotManager {
	return &SnapshotManager{
		snapshots:    make([]TimedSnapshot, 0),
		keyframes:    make(map[int64]*StateSnapshot),
		maxSnapshots: 10000,
	}
}

// Add adds a snapshot
func (s *SnapshotManager) Add(snapshot TimedSnapshot) {
	s.snapshots = append(s.snapshots, snapshot)
	
	// Add to keyframes if it's a keyframe
	if snapshot.Reason == ReasonKeyframe {
		s.keyframes[snapshot.Cycle] = &snapshot.Snapshot
	}
	
	// Limit total snapshots
	if len(s.snapshots) > s.maxSnapshots {
		// Remove oldest non-keyframe snapshots
		newSnapshots := make([]TimedSnapshot, 0, s.maxSnapshots)
		for _, snap := range s.snapshots {
			if snap.Reason == ReasonKeyframe || 
			   len(newSnapshots) < s.maxSnapshots/2 {
				newSnapshots = append(newSnapshots, snap)
			}
		}
		s.snapshots = newSnapshots
	}
}

// GetClosest finds the closest snapshot before a cycle
func (s *SnapshotManager) GetClosest(cycle int64) *TimedSnapshot {
	var closest *TimedSnapshot
	for i := range s.snapshots {
		if s.snapshots[i].Cycle <= cycle {
			if closest == nil || s.snapshots[i].Cycle > closest.Cycle {
				closest = &s.snapshots[i]
			}
		}
	}
	return closest
}

// GetRange returns snapshots in a cycle range
func (s *SnapshotManager) GetRange(start, end int64) []TimedSnapshot {
	result := []TimedSnapshot{}
	for _, snap := range s.snapshots {
		if snap.Cycle >= start && snap.Cycle <= end {
			result = append(result, snap)
		}
	}
	return result
}

// GetSectionStats gets statistics for a specific section
func (d *DeterminismDetector) GetSectionStats(start, end int64) DeterminismStats {
	deterministicCycles := int64(0)
	
	for _, section := range d.sections {
		// Check overlap
		overlapStart := max(section.StartCycle, start)
		overlapEnd := min(section.EndCycle, end)
		
		if overlapStart < overlapEnd {
			deterministicCycles += overlapEnd - overlapStart
		}
	}
	
	totalCycles := end - start
	ratio := float64(deterministicCycles) / float64(totalCycles)
	
	return DeterminismStats{
		TotalCycles:         totalCycles,
		DeterministicCycles: deterministicCycles,
		Ratio:              ratio,
	}
}

// Helper functions

func reasonString(reason SnapshotReason) string {
	switch reason {
	case ReasonInterval:
		return "Interval"
	case ReasonIO:
		return "I/O Event"
	case ReasonUncertain:
		return "Uncertainty"
	case ReasonSMC:
		return "SMC"
	case ReasonUser:
		return "User"
	case ReasonKeyframe:
		return "Keyframe"
	case ReasonParanoid:
		return "Paranoid"
	default:
		return "Unknown"
	}
}

func strategyString(strategy RecordingStrategy) string {
	switch strategy {
	case StrategyAutomatic:
		return "Automatic"
	case StrategyDeterministic:
		return "Deterministic"
	case StrategySnapshot:
		return "Snapshot"
	case StrategyHybrid:
		return "Hybrid"
	case StrategyParanoid:
		return "Paranoid"
	default:
		return "Unknown"
	}
}

func compressSnapshot(state *StateSnapshot) []byte {
	// Simple RLE compression for memory
	// In practice, use zlib or lz4
	return []byte{} // Placeholder
}

func min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

var debugMode = false // Set to true for debug output