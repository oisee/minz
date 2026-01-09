# MZV: Self-Modifying Code Visualization Tool

## Executive Summary
**MZV** will be a groundbreaking visualization tool for True Self-Modifying Code (TSMC) on Z80, showing in real-time how code rewrites itself for optimization. This tool will make the invisible visible, turning SMC from a black art into an observable science.

## Vision
"See your code evolve in real-time as it optimizes itself"

## Core Concept
MZV tracks and visualizes:
1. **Code mutations** - When and how instructions change
2. **Performance gains** - Cycle counts before/after modification
3. **Memory patterns** - Heat maps of self-modification activity
4. **Execution flow** - How control flow changes with SMC

## Architecture Design

### Components

#### 1. SMC Tracker (Backend)
```go
type SMCEvent struct {
    Address   uint16
    OldValue  byte
    NewValue  byte
    Cycle     int
    PC        uint16  // Where the modification came from
    Type      SMCType // Opcode change, operand patch, etc.
}

type SMCTracker struct {
    events    []SMCEvent
    heatMap   map[uint16]int  // Frequency of modifications
    timeline  []SMCSnapshot   // Code state over time
}
```

#### 2. Visualization Engine
- **Text Mode** (MVP): ASCII representation in terminal
- **HTML Mode**: Interactive web view with animations
- **Live Mode**: Real-time updates during execution

### Display Modes

#### Mode 1: Diff View
Shows before/after code changes:
```
Address | Before          | After           | Gain
--------|-----------------|-----------------|------
$8000   | JP NZ, $8010   | JR NZ, $10     | -1 byte, -3 cycles
$8003   | LD A, 00       | LD A, 42       | Constant folded
$8005   | CALL $9000     | LD HL, result  | Inlined: -17 cycles
```

#### Mode 2: Timeline View
Shows evolution over execution:
```
Cycle 0:    [JP NZ, $8010] [LD A, 00] [CALL $9000]
            ↓ SMC @ cycle 1000
Cycle 1000: [JR NZ, $10  ] [LD A, 00] [CALL $9000]
            ↓ SMC @ cycle 2000
Cycle 2000: [JR NZ, $10  ] [LD A, 42] [CALL $9000]
            ↓ SMC @ cycle 3000
Cycle 3000: [JR NZ, $10  ] [LD A, 42] [LD HL, res]
```

#### Mode 3: Heat Map
Shows modification frequency:
```
$8000: ████████████ (12 modifications)
$8001: ██           (2 modifications)
$8002: █████        (5 modifications)
$8003: ███████████  (11 modifications)
```

#### Mode 4: Performance Dashboard
```
=== SMC Performance Impact ===
Total modifications: 47
Code size reduction: 23 bytes (15%)
Cycle reduction: 3,421 cycles (34%)
Hottest address: $8003 (11 mods)

Top optimizations:
1. CALL elimination: 1,200 cycles saved
2. Jump shortening: 800 cycles saved
3. Constant folding: 600 cycles saved
```

## Implementation Plan

### Phase 1: Core Tracking (Week 1)
1. Integrate SMC tracker into RemogattoZ80
2. Hook memory write detection
3. Build event collection system
4. Create SMCEvent data structure

### Phase 2: Text Visualization (Week 2)
1. Implement diff view
2. Add timeline display
3. Create heat map renderer
4. Build performance calculator

### Phase 3: Interactive Mode (Week 3)
1. HTML/JavaScript frontend
2. WebSocket for live updates
3. Playback controls (pause, step, rewind)
4. Code annotation support

### Phase 4: Advanced Features (Week 4)
1. Pattern recognition (common SMC idioms)
2. Optimization suggestions
3. Export to video/GIF
4. Integration with MinZ compiler

## Use Cases

### 1. TSMC Development
- See exactly how your SMC patterns perform
- Identify unnecessary modifications
- Optimize modification sequences

### 2. Education
- Learn how SMC works visually
- Understand performance implications
- See classic SMC techniques in action

### 3. Debugging
- Track down SMC bugs
- Verify modification sequences
- Ensure atomic updates

### 4. Research
- Analyze SMC patterns in existing code
- Discover new optimization techniques
- Measure SMC effectiveness

## Technical Integration

### With MZE
```go
// Run with SMC tracking
mze program.a80 --track-smc --visualize
```

### With MinZ Compiler
```go
// Compile with SMC instrumentation
mz program.minz -o program.a80 --instrument-smc
```

### Standalone
```go
// Analyze existing binary
mzv analyze program.a80 --smc-patterns
```

## Example Visualizations

### Simple Operand Patching
```
Before: LD A, $00  ; Placeholder
After:  LD A, $42  ; Patched value
Impact: Constant propagated at runtime
```

### Jump Table Optimization
```
Before: JP (HL)    ; Indirect jump
After:  JP $8050   ; Direct jump
Impact: -6 cycles per iteration
```

### Call Inlining
```
Before: CALL PrintChar
After:  OUT ($01), A
Impact: -17 cycles, -2 bytes stack
```

## Success Metrics
- Visualize 100% of SMC events
- < 5% performance overhead when tracking
- Identify optimization opportunities
- Support for all TSMC patterns

## Future Extensions
1. **3D Visualization**: Memory space as 3D grid
2. **AI Analysis**: Pattern recognition and suggestions
3. **Collaborative**: Share SMC patterns online
4. **Competition Mode**: SMC golf challenges

## Deliverables
1. `cmd/mzv/main.go` - CLI tool
2. `pkg/emulator/smc_tracker.go` - Tracking system
3. `pkg/visualization/` - Rendering engines
4. `web/` - HTML/JS frontend
5. Documentation and examples

## Revolutionary Impact
MZV will transform SMC from an arcane technique into a visible, measurable, optimizable practice. For the first time, developers will SEE their code evolve and optimize itself in real-time, making the Z80's most powerful feature accessible to everyone.

---

"With MZV, self-modifying code is no longer invisible magic—it's observable science."