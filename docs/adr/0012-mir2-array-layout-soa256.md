# ADR-0012: MIR2 Array Layout Strategy — AoS / SoA / SoA256

## Status
Accepted

## Context

Z80 has no multiply instruction. Random array access (`base + index * stride`)
requires a software multiply loop — expensive. But sequential access via pointer
bumping is nearly free:

| Operation        | Instruction     | T-states |
|------------------|-----------------|----------|
| next u8 element  | INC HL          | 6T       |
| next u16 element | INC HL; INC HL  | 12T      |
| next stride-N    | ADD HL, BC      | 11T+setup|
| random index u8  | LD L, i         | 7T (SoA256 only) |
| random index u16 | — (needs mul)   | ~50-100T |

### The H=field / L=index insight

For a page-aligned Structure of Arrays layout (`LayoutSoA256`), if all column
arrays for one struct are allocated in consecutive 256-byte pages starting at
`$xx00`:

```
$C000: field_x[0..N-1]   ; page $C0
$C100: field_y[0..N-1]   ; page $C1
$C200: field_z[0..N-1]   ; page $C2
```

Then:
- **H** = column selector (field index) — `INC H` to switch field (4T!)
- **L** = row selector (element index) — `INC L` to advance, `LD L, i` for random access
- **B** is FREE for DJNZ — no pointer pair needed for three simultaneous columns

This gives O(1) random access without multiply, and cross-field switching in 4T.
This is the only case on Z80 where random struct field access is free.

### Register availability comparison

| Layout   | Pointers needed | B free? | Notes |
|----------|-----------------|---------|-------|
| AoS      | 1 × HL          | yes     | stride ≥ struct size |
| SoA      | N × (HL/DE/BC…) | **no** if N≥3 | BC conflicts with DJNZ counter |
| SoA256   | 1 × HL          | **yes** | H=field, L=index |

## Decision

Add three layout variants to `ArrayTy` in `pkg/mir2/types.go`:

```go
type ArrayLayout uint8
const (
    LayoutAoS    ArrayLayout = iota  // default: row-major, C-style
    LayoutSoA                        // columnar: one array per struct field
    LayoutSoA256                     // page-aligned SoA: H=field, L=index
)
```

`ArrayTy` gains `Layout ArrayLayout` and `Align int` fields.

### Lowering strategy

**LayoutAoS** (default): no change. `[N]Struct` stays as-is in memory.

**LayoutSoA**: lowering pass before codegen splits `[N]Struct{f0,f1,f2}` into
N separate `[N]Ti` global arrays (one per field). Field access becomes pointer
arithmetic on the individual column array.

**LayoutSoA256**: same as SoA, but allocator places column arrays on consecutive
256-byte page boundaries. Codegen uses `INC H` for column switch and `INC L` /
`LD L, i` for element access.

### Codegen for SoA256

```asm
; Iterating with SoA256, three fields simultaneously:
    LD H, $C0       ; base page (compile-time constant)
    LD L, 0         ; element index = 0
    LD B, N         ; DJNZ counter — B is FREE because no pointer pair needed
.loop:
    LD A, (HL)      ; read field_x[L]
    INC H           ; switch to field_y page
    ADD A, (HL)     ; + field_y[L]
    INC H           ; switch to field_z page
    ADD A, (HL)     ; + field_z[L]
    LD H, $C0       ; restore base page
    ; ... process A ...
    INC L           ; next element (wraps at 256 automatically)
    DJNZ .loop
```

## Constraints and verification

The verifier must enforce:

| Constraint | Reason |
|------------|--------|
| `LayoutSoA256` → `Len ≤ 256` | L is 8-bit |
| `LayoutSoA256` → global allocation only | Stack cannot be page-aligned |
| `LayoutSoA256` → `Align == 256` | Pages must start at $xx00 |
| Number of fields × 256 ≤ available RAM | Pages must be contiguous |

## PBQP application: page assignment optimisation

When a struct has many fields and code accesses different subsets in different
loops, the order of pages matters — we want frequently co-accessed fields in
consecutive pages to favour `INC H` over `LD H, page`.

This is a quadratic assignment problem:
- **Nodes**: field column arrays
- **Variables**: page offset (0, 1, 2, ...)
- **Edge cost**: `INC H` (4T) if consecutive, `LD H, imm` (10T) if not
- **Optimisation**: minimise total `LD H, imm` instructions weighted by access frequency

This is a natural PBQP instance. Implemented in Phase 5b after profiling data is
available from Phase 4 programs.

## Consequences

### Positive
- Iterator-biased access becomes the natural default for array-of-struct patterns
- SoA256 enables O(1) random access without multiply on Z80
- BC register freed for DJNZ in hot loops with multiple columns
- Foundation for PBQP page assignment optimisation

### Negative
- SoA256 is globals-only — stack-allocated structs stay AoS
- Linker/allocator must support aligned segment placement (Phase 3 linker work)
- Memory layout changes are ABI-breaking — external code cannot mix layouts

## Implementation phases

| Phase | Work |
|-------|------|
| Now   | Add `ArrayLayout`, `Align` to `ArrayTy` in types.go (types only, no codegen change) |
| 3     | `OpField`, `OpPtrBump` opcodes; SoA lowering pass; basic AoS codegen |
| 3     | SoA256 codegen: `INC H` / `INC L` / `LD L, i` patterns |
| 3     | Verifier constraints for SoA256 |
| 5b    | PBQP page assignment optimiser |
