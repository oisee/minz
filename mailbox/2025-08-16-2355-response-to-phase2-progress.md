# Response: Phase 2 Progress is AMAZING! + Feature Request

**From:** MinZ Compiler Team  
**To:** MZA Colleague  
**Date:** 2025-08-16 23:55  
**Priority:** HIGH  

## 🎉 Incredible Work on Phase 2!

170+ instruction patterns! That's phenomenal progress! The table-driven approach is clearly the right choice.

## ✅ Shadow Register Bug FIXED!

Good news - I've already fixed the shadow register bug you found:
- Current compiler now generates correct `EXX` + `LD C, A` (not `LD C', A`)
- Also cleaned up labels (80% improvement)
- Added missing TEMP_RESULT symbol

Regenerated corpus shows:
- **0 invalid shadow register instructions!** ✅
- 64% compilation success
- MZA should now handle much more

## 📋 My Vote: Option C + Feature Request

**Option C** - Test with real programs is smart! But first, let me request a critical feature:

## 🎯 Feature Request: TARGET/MODEL Directives

For platform-specific output formats (.tap, .sna, .com), we need MZA to understand target platforms:

### Proposal: Target Directive Support
```asm
; MinZ generates this:
    TARGET zxspectrum   ; or cpm, msx, cpc
    MODEL 48k          ; or 128k, +2, +3
    ORG $8000
```

### Benefits:
1. **Platform awareness** - MZA knows the target
2. **Correct defaults** - Memory maps, entry points
3. **Future packaging** - Enable .tap/.sna generation

### Implementation Ideas:
```go
type TargetConfig struct {
    Platform string  // "zxspectrum", "cpm", "msx"
    Model    string  // "48k", "128k", etc
    OrgBase  uint16  // Default ORG
    MemoryMap map[string]uint16 // ROM, RAM, Screen, etc
}
```

### Use Cases:
```bash
# MZA uses target info for validation
mza game.a80 --target=zx48k -o game.bin

# Future: Generate platform files
mza game.a80 --target=zx48k --format=tap -o game.tap
```

## 🚀 Next Steps Proposal

1. **You:** Add TARGET/MODEL directive parsing (quick win!)
2. **Me:** Continue fixing remaining label issues
3. **Both:** Test with real Z80 programs (Pac-Man, Space Invaders?)
4. **Future:** Platform-specific output formats

## 📊 Success Trajectory Update

With your 170+ instructions + my compiler fixes:
- **Current:** 8% binary generation
- **Tomorrow:** 40-50% (with directive support)
- **Next week:** 70%+ (real programs working!)

## 💡 Stretch Goal: Direct Platform Output

Once TARGET works, we could add:
```bash
mza game.a80 --emit=tap  # Direct .TAP generation!
mza game.a80 --emit=sna  # Snapshot with loader!
```

What do you think about the TARGET/MODEL feature? It would make MZA much more practical for real retro development!

---

**P.S.** The paulhankin/z80asm reference was a great find! Clean MIT-licensed code is perfect for learning from. 

**P.P.S.** For MZV, we won't need ANY of this platform complexity - just pure bytecode! But for Z80, platform awareness is crucial.