# PGO E2E Metadata Integration: Quick-Win/Mid-Win/Slow-Win Strategy

## Executive Summary

MinZ needs enhanced metadata propagation throughout the compilation pipeline to maximize PGO effectiveness. We already have basic blocks in MIR visualizer and TAS infrastructure - we just need to connect them!

## 🚀 Quick Wins (1-2 weeks)

### QW1: Add Source Location to MIR Instructions (3 days)
```go
// Extend ir.Instruction with metadata
type Instruction struct {
    Op           Opcode
    // ... existing fields ...
    
    // NEW: Metadata for PGO
    SourceLine   int      // Line number in original .minz file
    SourceFile   string   // File path
    BasicBlockID int      // Which basic block this belongs to
    ProfileHint  string   // Hints like "hot", "cold", "likely"
}
```

**Implementation:**
1. Parser already has line info - just propagate it
2. Semantic analyzer preserves it when generating MIR
3. Optimizer preserves it through transformations
4. Codegen emits as comments for debugging

**Impact:** Enables precise profile→source mapping immediately!

### QW2: Hook TAS Recording to Basic Block Tracking (2 days)
```go
// In tas_debugger.go
type TASDebugger struct {
    // ... existing fields ...
    
    // NEW: Profile collection
    blockExecutions map[int]uint64     // BasicBlockID -> count
    branchOutcomes  map[int]BranchStat // PC -> taken/not-taken
}

func (t *TASDebugger) OnPC(pc uint16) {
    // Map PC to BasicBlockID using debug info
    if blockID, ok := t.pcToBlock[pc]; ok {
        t.blockExecutions[blockID]++
    }
}
```

**Impact:** Zero-overhead profiling using existing TAS!

### QW3: Simple Hot/Cold Annotation Pass (3 days)
```go
// New optimizer pass
func AnnotateHotCold(mir *MIR, profile *TASProfile) {
    threshold := calculateThreshold(profile)
    
    for _, fn := range mir.Functions {
        for i, inst := range fn.Instructions {
            blockID := inst.BasicBlockID
            if profile.blockExecutions[blockID] > threshold {
                inst.ProfileHint = "hot"
            } else if profile.blockExecutions[blockID] == 0 {
                inst.ProfileHint = "cold"
            }
        }
    }
}
```

**Usage:**
```bash
# Collect profile
mze game.a80 --record game.tas

# Compile with profile
mz game.minz --pgo game.tas -o game_optimized.a80
```

**Impact:** 10-20% speedup just from better code layout!

### QW4: Platform-Aware Memory Layout (2 days)
```go
// In codegen/z80.go
func (g *Z80Generator) PlaceFunction(fn *Function, hint string) uint16 {
    switch g.Target {
    case "spectrum":
        if hint == "hot" {
            return g.AllocateUncontended() // 0x8000+
        }
        return g.AllocateContended() // 0x4000+
        
    case "cpm":
        if hint == "hot" && g.rstVectorsFree > 0 {
            g.rstVectorsFree--
            return uint16(g.rstVectorsFree * 8) // RST vector
        }
        return g.AllocateNormal()
    }
}
```

**Impact:** Spectrum games 15-30% faster by avoiding contention!

## 🎯 Mid Wins (1 month)

### MW1: Full Basic Block Analysis Infrastructure (1 week)
```go
// New package: pkg/analysis/cfg.go
type ControlFlowGraph struct {
    Blocks      []BasicBlock
    Edges       []Edge
    Dominators  map[int]int     // Block domination tree
    LoopHeaders map[int]bool    // Loop detection
}

type BasicBlock struct {
    ID          int
    Start       int      // First instruction index
    End         int      // Last instruction index
    Successors  []int    // Next blocks
    ProfileData BlockProfile
}

type BlockProfile struct {
    ExecutionCount uint64
    TotalCycles    uint64  // Z80 T-states spent here
    HeatScore      float64 // Normalized 0.0-1.0
}
```

**Integration with existing visualizer:**
```go
// Enhance existing visualizer
func (v *Visualizer) ExportCFG() *ControlFlowGraph {
    // Convert visualization data to CFG
    cfg := &ControlFlowGraph{
        Blocks: v.identifyBasicBlocks(),
    }
    cfg.ComputeDominators()
    cfg.DetectLoops()
    return cfg
}
```

### MW2: Profile-Guided Register Allocation (1 week)
```go
// Enhanced register allocator
type PGORegisterAllocator struct {
    cfg     *ControlFlowGraph
    profile *ProfileData
}

func (ra *PGORegisterAllocator) AllocateRegisters(fn *Function) {
    // Compute register pressure for each block
    for _, block := range ra.cfg.Blocks {
        pressure := ra.computePressure(block)
        heat := ra.profile.GetHeat(block.ID)
        
        // Hot blocks get priority for register allocation
        if heat > 0.8 {
            ra.allocateAggressively(block) // Keep everything in registers
        } else if heat < 0.2 {
            ra.allocateMinimally(block)    // Spill to memory freely
        }
    }
}
```

### MW3: Edge Profiling for Branch Optimization (1 week)
```go
// Track edge transitions, not just blocks
type EdgeProfile struct {
    From  int     // Source block
    To    int     // Target block
    Count uint64  // How often taken
    Prob  float64 // Probability (0.0-1.0)
}

func OptimizeBranches(cfg *ControlFlowGraph, profile *ProfileData) {
    for _, block := range cfg.Blocks {
        if block.HasConditionalBranch() {
            edges := profile.GetEdges(block.ID)
            
            // Sort edges by probability
            mostLikely := edges.MostProbable()
            
            // Invert branch if necessary to make likely path fall-through
            if mostLikely.To != block.ID + 1 {
                block.InvertBranch()
                block.SwapSuccessors()
            }
        }
    }
}
```

### MW4: DJNZ Pattern Recognition with Profile (1 week)
```go
// Automatically convert hot loops to DJNZ
type DJNZOptimizer struct {
    profile *ProfileData
}

func (opt *DJNZOptimizer) FindDJNZCandidates(cfg *ControlFlowGraph) []DJNZCandidate {
    candidates := []DJNZCandidate{}
    
    for blockID, isLoop := range cfg.LoopHeaders {
        if !isLoop {
            continue
        }
        
        heat := opt.profile.GetHeat(blockID)
        if heat > 0.6 { // Hot loop
            if pattern := opt.matchDJNZPattern(cfg, blockID); pattern != nil {
                candidates = append(candidates, pattern)
            }
        }
    }
    
    return candidates
}
```

## 🚀 Slow Wins (2-3 months) - Revolutionary Features

### SW1: Whole-Program Profile Database (3 weeks)
```go
// Persistent profile storage
type ProfileDatabase struct {
    Version     int
    Programs    map[string]*ProgramProfile
    SharedStats *GlobalStatistics
}

type ProgramProfile struct {
    Source      string           // MinZ source hash
    Target      string           // Platform (spectrum/cpm/etc)
    Runs        []RunProfile     // Multiple execution profiles
    Aggregate   AggregateProfile // Combined statistics
}

// Profile sharing via cloud
func (db *ProfileDatabase) Upload(url string) error {
    // Share profiles with community
    return http.Post(url, "application/protobuf", db.Serialize())
}

func (db *ProfileDatabase) Download(program string) (*ProgramProfile, error) {
    // Get optimal profile from community
    return http.Get(fmt.Sprintf("%s/profiles/%s", url, program))
}
```

**Usage:**
```bash
# Download optimal profile for Pac-Man
mz pacman.minz --pgo-download -o pacman.a80

# Share your profile
mze pacman.a80 --record --pgo-upload
```

### SW2: Auto-Tuning PGO with Multiple Profiles (4 weeks)
```go
// Machine learning-inspired profile combination
type AutoTuner struct {
    profiles []*ProfileData
    weights  []float64
}

func (at *AutoTuner) CombineProfiles() *ProfileData {
    combined := &ProfileData{}
    
    // Weight profiles by execution characteristics
    for i, profile := range at.profiles {
        // Profiles from longer runs get more weight
        // Profiles with more coverage get more weight
        weight := at.calculateWeight(profile)
        combined.Merge(profile, weight)
    }
    
    return combined
}

func (at *AutoTuner) OptimizeForScenario(scenario string) {
    switch scenario {
    case "game":
        // Prioritize frame rate consistency
        at.weights = at.optimizeForLatency()
    case "demo":
        // Prioritize peak performance
        at.weights = at.optimizeForThroughput()
    case "tool":
        // Prioritize code size
        at.weights = at.optimizeForSize()
    }
}
```

### SW3: Profile-Guided Self-Modifying Code Generation (4 weeks)
```go
// Revolutionary: Automatic SMC from profiles
type SMCGenerator struct {
    profile  *ProfileData
    analysis *StaticAnalysis
}

func (gen *SMCGenerator) GenerateSMC(fn *Function) []SMCPatch {
    patches := []SMCPatch{}
    
    for _, inst := range fn.Instructions {
        if inst.ProfileHint != "hot" {
            continue
        }
        
        // Find constant values from profile
        if values := gen.profile.GetConstantValues(inst); values != nil {
            if values.IsAlwaysSame() {
                // Generate SMC patch
                patch := SMCPatch{
                    Location: inst.Address,
                    Value:    values.Constant(),
                    Original: inst.Generate(),
                }
                patches = append(patches, patch)
            }
        }
    }
    
    return patches
}

// Runtime patcher
func GeneratePatcher(patches []SMCPatch) string {
    code := "smc_init:\n"
    for _, patch := range patches {
        code += fmt.Sprintf("    LD A, %d\n", patch.Value)
        code += fmt.Sprintf("    LD (%d), A\n", patch.Location)
    }
    code += "    RET\n"
    return code
}
```

### SW4: Interactive Profile Exploration (4 weeks)
```go
// Visual profiler integrated with mzv
type InteractiveProfiler struct {
    visualizer *MIRVisualizer
    profile    *ProfileData
    emulator   *Z80Emulator
}

func (ip *InteractiveProfiler) Visualize() {
    // Generate heat map overlay
    ip.visualizer.AddHeatMap(ip.profile)
    
    // Show in browser with D3.js
    ip.ServeHTTP(":8080")
}

// Real-time profile viewing
func (ip *InteractiveProfiler) LiveProfile() {
    go func() {
        for {
            snapshot := ip.emulator.GetState()
            ip.profile.Update(snapshot)
            ip.broadcastUpdate() // WebSocket to browser
            time.Sleep(100 * time.Millisecond)
        }
    }()
}
```

**Browser interface:**
```javascript
// Live heat map of execution
class ProfileViewer {
    updateHeatMap(data) {
        // D3.js visualization
        this.svg.selectAll('.basic-block')
            .data(data.blocks)
            .style('fill', d => this.heatColor(d.heat))
            .attr('stroke-width', d => d.executing ? 3 : 1);
    }
    
    showRecommendations(profile) {
        // AI-powered optimization suggestions
        const suggestions = analyzePGOPotential(profile);
        this.showPanel(suggestions);
    }
}
```

## 📊 Metadata Flow Architecture

```
Source (.minz) → [Parser + Line Info] → AST with locations
                                              ↓
                            [Semantic Analyzer + Preserve Metadata] → MIR with source mapping
                                              ↓
                                    [CFG Builder] → Basic Blocks with IDs
                                              ↓
                            [Optimizer + Profile] → Annotated MIR with hints
                                              ↓
                            [Code Generator + Layout] → Assembly with profile-guided placement
                                              ↓
                                    [Assembler] → Binary with debug symbols
                                              ↓
                            [Emulator + TAS] → Profile collection with mapping
                                              ↓
                                    [Profile DB] → Persistent optimization data
```

## Expected Results by Timeline

### After Quick Wins (2 weeks)
- ✅ Basic PGO working end-to-end
- ✅ 10-20% performance improvement
- ✅ Platform-specific optimizations
- ✅ Profile collection via TAS

### After Mid Wins (6 weeks)
- ✅ 30-50% performance improvement
- ✅ Automatic DJNZ conversion
- ✅ Smart register allocation
- ✅ Branch prediction optimization

### After Slow Wins (3 months)
- ✅ 50-200% performance improvement
- ✅ Community profile sharing
- ✅ Automatic SMC generation
- ✅ Interactive optimization exploration
- ✅ World-class PGO system

## Conclusion

MinZ is perfectly positioned for PGO success:
1. **TAS = Profiler** (already built!)
2. **MIR has basic blocks** (just need IDs)
3. **Visualizer exists** (add heat maps)
4. **Platform targets defined** (optimize per-platform)

The Quick Wins alone will deliver impressive results. The full vision will make MinZ the first compiler with true PGO for vintage platforms!

---

*"From accidental profiler to intentional revolution!"*