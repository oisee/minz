# Next Session Briefing: Z80 Codegen — Last 12 Errors

**Previous session:** 2026-03-21, 61 commits, ~3000→12 errors (−99.6%)
**Branch:** master
**Working dir:** minzc/

## Current State

```
107 compilable files, 104 CLEAN (97%)
3 files with errors, 12 total
FatFS (ff.c, 7K LOC) = ZERO errors
```

## The Final 12

### 1. arena_allocator.nanz (8 errors) — PBQP ClassFlag

```
3× INC F, 1× DEC F, 1× LD F, 0, 1× SBC HL, F, 2× LD A, HL
```

**Root cause:** A bool vreg gets ClassFlag from OpCmp (cost 0 for LocFlag=F).
Same vreg feeds through block params into OpAdd (INC) — but F can't INC.

**What we tried:**
- `propagateClassHints` Pass 0: demote ClassFlag→ClassAcc for ALU-used vregs — didn't help because vreg flows through block params that reset class
- `nodeCosts` aluUsed exclusion — too blunt, caused more spills
- `locCompatible(LocFlag) = false` — broke 2 tests (TestAllocateFlagGetsCPUFlag, TestE2EFlagReturn)

**Real fix needed:** `locCompatible` must accept `RegClass` parameter. When class IS ClassFlag AND type is bool → allow LocFlag. When class got demoted to ClassAcc → reject LocFlag. This requires changing `locCompatible(Ty, PhysLoc)` → `locCompatible(Ty, PhysLoc, RegClass)` and threading class through all callers.

**Alternative:** Add 2-pass allocation — first pass identifies multi-use vregs, second pass forces ClassAcc for those.

**Files:** `pkg/mir2/alloc.go:locCompatible`, `pkg/mir2/pbqp.go:nodeCosts`, `pkg/mir2/precoalesce.go:propagateClassHints`

### 2. nc.nanz (2 errors) — genCmp16 LD L, IYH

```
2× LD L, IYH  (in get_dir_entry and get_dir_entry_full — identical functions)
```

**Root cause:** genCmp16 lhs zero-extend path. `lhs = "IYH"` (8-bit half-reg).
Our guard at line ~6264 checks `isIXYReg(lhs)` and routes through `emitMovViaAltA("L", lhs)`. The guard EXISTS and FIRES for some instances. But these 2 remaining go through a DIFFERENT path.

**What we tried:**
- Added isIXYReg guard in genCmp16 (fixed 6 of 8 similar errors)
- Replaced all raw `LD L, %s` with emitLD8 (fixed more)
- emitLD8 has IXY guard at top
- emitMovViaAltA works correctly

**Debug approach:** The `LD L, IYH` at assembly line ~3827 follows `LD A, 0` directly. Search for ALL code paths that emit `LD L,` followed by `LD H, 0` — there might be a genBinOp byte-op or block-param copy path that bypasses our guards. Add temporary `fmt.Fprintf(os.Stderr, "DEBUG LD L, %s\n", src)` before each raw LD L emit to trace.

**Assembly context:**
```asm
.fs_fat12_get_dir_entry_if_then15:
    LD A, 0
    LD L, IYH      ; ← BUG: should be EX AF,AF'; LD A, IYH; LD L, A; EX AF,AF'
    LD H, 0
```

**Files:** `pkg/mir2/z80codegen.go` — search for remaining `emitf.*LD.*lhs` that don't go through emitLD8

### 3. sieve.pas (2 errors) — Pascal IndexExpr TyPtr

```
2× LD (D), A  (array store Flags[I] := 1 and Flags[K] := 0)
```

**Root cause:** `IndexExpr.ExprTy()` returns `ElemTy` (TyU8 for boolean array) instead of TyPtr. When used as `StoreStmt.Ptr`, the ptr vreg gets type TyU8 → PBQP allocates to D (8-bit register) → codegen emits `LD (D), A` which is invalid.

**The weird thing:** `lowerIndex()` at line 1707 calls `l.bld.PtrAdd(base, offset, ClassPointer)` which creates vreg with `Ty: TyPtr`. So the PtrAdd result IS TyPtr. But something between PtrAdd and OpStore loses the type.

**Debug approach:** Add `fmt.Fprintf(os.Stderr, "OpStore ptr=%v type=%v\n", ptr, g.ar.Loc(inst.Src[0]))` in genInst OpStore to see what type the ptr vreg has at allocation time.

**Fix options:**
1. In Pascal lowerer `lower.go:334` — ensure StoreStmt.Ptr type is TyPtr
2. In `collectRegInfo` — if a vreg is used as OpStore.Src[0] (ptr), force TyPtr
3. Change `IndexExpr.ExprTy()` to return TyPtr when the IndexExpr is used as a pointer (context-dependent, harder)

**Files:** `pkg/pascal/lower.go:325-336`, `pkg/hir/hir.go:475`, `pkg/mir2/alloc.go:collectRegInfo`

## Neighbor's Bugs (from commit c4ef9464)

### Still open:
1. **`@z80_c` → ClassGeneral** — inline asm `@z80_c` maps to ClassGeneral, not physical C. PBQP free to assign B/C/D/E. Need ClassRegC or LocPin.
2. **Caller skips `LD A, imm`** — first const arg to ClassAcc param not loaded. emitCallArgs/constprop issue.

### Fixed:
3. **screen_addr assert value** — 0x4B8F was correct, assert had wrong expected value. Fixed.

## Key Architecture References

### Universal helpers (all in z80codegen.go):
- `emitLD8(dst, src)` — handles spill, IXY, F, pair, width mismatch
- `emitADDHL(rhs)` — IX/IY/spill guard for ADD HL
- `emitSBCHL(rhs)` — same for SBC HL
- `emitMovViaAltA(dst, src)` — EX AF,AF' routing for DD/FD conflicts
- `emitLDA(src)` — safe LD A with pair/spill/F guards
- `promote8toPair(r)` — A→BC (not AF), others→parentPair
- `isSpill(s)` — checks both `$F0xx` and `_spill_` prefix
- `emitLD8` pair guard: truncates pair→lowByte at top

### Spill infrastructure:
- Named labels: `_spill_{func}_r{N}: DB/DW 0` in data section per function
- TSMC labels: `_tsmc_{func}_r{N}_{useIdx}` — patched immediates
- Spill data emitted at end of genFunc, filtered by function + TSMC exclusion
- `combined.Spilled` populated in pipeline.go (both CompileAll paths)

### HIR Splitter:
- `pkg/hir/split.go` — SplitHighPressure, EstimatePressure, FindSplitPoints
- Threshold: 8 live vars. Max interface width: 6. Max depth: 3.
- 21 splits across corpus (ff.c: 6, nc.nanz: 13, others: 2)

### Strategy doc: `docs/Z80_Register_Pressure_Strategy.md`
### Remaining issues: `docs/Z80_Codegen_Remaining_Issues.md`
### Report: `reports/2026-03-21-106-Z80-Codegen-99-Percent-Clean.md`

## Quick Start

```bash
cd minzc

# Rebuild
go build -o ~/.local/bin/mz ./cmd/minzc/

# Run corpus check (non-archive)
for src in $(find ../examples -name "*.nanz" -o -name "*.minz" -o -name "*.c" -o -name "*.pas" -o -name "*.lizp" -o -name "*.abap" | grep -v _archive | sort); do
  log=$(mz "$src" -o /tmp/_ci.a80 --lir=false 2>&1)
  [ $? -eq 0 ] && echo "$log" | grep -q "Z80-VALIDATE" && echo "$(echo "$log" | grep -c '>>>') $(basename $src)"
done

# Expected: 8 arena, 2 nc.nanz, 2 sieve.pas = 12 total

# Run tests
go test ./pkg/mir2/ -count=1 -timeout 60s
go test ./pkg/hir/ -count=1 -timeout 30s
```

## Priorities for Next Session

1. **nc.nanz (2)** — quickest win, trace the specific LD L, IYH path
2. **sieve.pas (2)** — Pascal frontend IndexExpr type fix
3. **arena (8)** — PBQP locCompatible + RegClass, biggest architectural change
4. **Neighbor bugs** — @z80_c ClassGeneral, caller LD A skip
5. **Assert coverage** — add Z80 asserts for tetris, nc, fatfs functions
6. **Release** — update README, version bump, changelog
