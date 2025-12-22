# MinZ Code Generation Improvements

*Based on analysis in docs/260_MinZ_Code_Generation_Analysis.md*

## Priority 1: Critical (30%+ improvement expected)

### [ ] Dead Code Elimination
- Remove unreachable code after inlined functions
- Pattern: RET followed by more code before next label
- Location: `minzc/pkg/codegen/z80_backend.go`
- Estimated impact: 10-15% size reduction

### [ ] Register Allocation Rewrite
- Current: Hierarchical with aggressive spilling
- Target: Proper liveness analysis + graph coloring
- Files: `minzc/pkg/codegen/register_allocator.go`
- Estimated impact: 20-30% size reduction

### [ ] Fix Duplicate Register Moves
```asm
; Current (bad):
    LD D, H
    LD E, L
    ; Could optimize: LD H,D / LD L,E
    LD H, D
    LD L, E

; Should be: nothing (registers already correct)
```

## Priority 2: Medium (10-20% improvement)

### [ ] Comparison Simplification
```asm
; Current (12 instructions for a <= b):
    LD E, B
    LD D, 0
    OR A
    SBC HL, DE
    JP M, le_true
    JP Z, le_true
    LD HL, 0
    JP le_done
le_true:
    LD HL, 1
le_done:

; Optimal (3 instructions):
    CP B
    JR C, less_than
    JR Z, equal
```

### [ ] EXX Batching
- Group shadow register operations
- Single EXX pair for multiple accesses
- Pattern: `EXX; LD x,y; EXX` -> accumulate and batch

### [ ] Jump Threading
```asm
; Current:
    JP NZ, label_a
    JP label_b
label_a:
    JP end

; Optimal:
    JP Z, label_b
label_a:
    ; fall through to end
```

## Priority 3: Polish (5-10% improvement)

### [ ] Use INC/DEC for ±1
```asm
; Current:
    LD A, 1
    ADD A, B

; Optimal:
    INC B
    LD A, B
```

### [ ] Constant Folding
```asm
; Current:
    LD A, 2
    LD B, 3
    ADD A, B

; Optimal:
    LD A, 5
```

### [ ] Loop Optimization
- Detect for-loops with constant bounds
- Use DJNZ where possible (8-bit counter)
- Unroll small loops

## Metrics to Track

| Metric | Current | Target |
|--------|---------|--------|
| fibonacci.a80 lines | 117 | 50 |
| simple_add.a80 lines | 83 | 30 |
| Average lines/function | 50 | 25 |
| Peephole patterns | 35 | 60 |

## Testing Strategy

1. Create `minzc/tests/codegen_quality_test.go`
2. Assert maximum line counts for reference programs
3. Count specific patterns (EXX pairs, dead RETs, etc.)
4. Compare T-state estimates

## References

- docs/260_MinZ_Code_Generation_Analysis.md
- docs/149_World_Class_Multi_Level_Optimization_Guide.md
- Z80 User Manual (Zilog)
- SDCC codegen for comparison
