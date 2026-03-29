# LLVM Backend Action Plan

**Status:** 42/189 native binaries (22%), 42/87 compilable programs (48%)
**Goal:** 60+ native binaries, all frontends working

---

## Priority 1: Print Runtime Stubs (unlocks 13 programs)

**Problem:** 13 programs fail with `Symbols not found: _mir_io_print_u8, _mir_io_print_str, _mir_io_print_nl`

**Fix:** Implement a minimal LLVM runtime library that provides print functions via libc `printf`/`putchar`.

```llvm
; runtime.ll — link with: clang program.ll runtime.ll -o program

declare i32 @putchar(i32)
declare i32 @printf(ptr, ...)

@.fmt_u8 = private constant [4 x i8] c"%d\00"
@.fmt_nl = private constant [2 x i8] c"\0A\00"

define void @_mir_io_print_u8(i32 %v) {
  call i32 (ptr, ...) @printf(ptr @.fmt_u8, i32 %v)
  ret void
}

define void @_mir_io_print_str(ptr %s) {
  call i32 (ptr, ...) @printf(ptr %s)
  ret void
}

define void @_mir_io_print_nl() {
  call i32 @putchar(i32 10)
  ret void
}

define void @_mir_io_print_dec(i32 %v) {
  call i32 (ptr, ...) @printf(ptr @.fmt_u8, i32 %v)
  ret void
}
```

**Approach:** Auto-append runtime.ll when `--emit llvm` detects external deps.
Or: embed as string constant in Go, write alongside program.ll.

**TUI stubs** (tui_goto, tui_color, etc.): ANSI escape sequences via printf.
**SQLite stubs:** Later — need actual SQLite linkage or stub returning 0.
**peek/poke:** Map to array read/write in linear memory.

**Expected unlock:** +8-10 Nanz programs, +2 ABAP, +1-2 Lizp

---

## Priority 2: Fix i32→i32 Cast (unlocks 7 Frill programs)

**Problem:** `invalid cast opcode for cast from 'i32' to 'i32'` in 7 Frill files.

**Root cause:** `OpExt`/`OpTrunc` where SrcTy and DstTy both map to i32 (uniform type). The codegen already handles `srcTy == ty` → no-op add. But the conversion for call args (`emitConvert`) may still emit `bitcast i32 to i32`.

**Fix:** In `emitConvert()`, add early return for same-type:
```go
func (g *gen) emitConvert(dst, src, fromTy, toTy string) {
    if fromTy == toTy {
        g.sb.WriteString(fmt.Sprintf("  %s = add %s 0, %s\n", dst, toTy, src))
        return
    }
    // ... rest of conversion logic
}
```

Also check all call sites that insert conversions — may need `AND 0xFF` / `AND 0xFFFF` for range masking instead of ext/trunc.

**Expected unlock:** +7 Frill programs (basics, calculator, math, game, hello_cpm, interactive, graphics)

---

## Priority 3: Z80 Validator Stderr Leak (unlocks 5 programs)

**Problem:** `expected top-level entity` in 5 files — Z80 validator writes warnings to stdout, corrupting LLVM IR.

**Root cause:** `[Z80-VALIDATE] terminate: 1 invalid instruction(s)` appears in `--emit llvm` output.

**Fix:** In pipeline.go or cmd/minzc/main.go, ensure Z80 validation messages go to stderr only. The `--emit llvm` path should NOT run Z80 validation at all (it's not Z80 output).

```go
// In compileViaHIR, skip Z80 validation when emitFormat is set
if emitFormat == "llvm" || emitFormat == "wasm" {
    // Don't run Z80 asm/validation
}
```

**Expected unlock:** +3 ABAP (makt_search, mara_alv, mara_alv_real_zx), +2 Pascal (bubble_sort, records)

---

## Priority 4: Ptr↔Int in Screen Functions (5 programs)

**Problem:** `invalid operand type for instruction` in screen_alv, screen_customer, screen_declarative, screen_report, abap_screen.

**Root cause:** Screen metafunctions (@screen, @tui) inject host function calls with ptr arguments. The MIR2 IR has ptr types that don't flow correctly through LLVM codegen.

**Investigation needed:** Check if these programs use `@extern` host functions that bypass MIR2 IR. If so, they need LLVM `declare` + proper ptr handling.

**Expected unlock:** +5 programs

---

## Priority 5: Frill Duplicate Functions (3 programs)

**Problem:** `invalid redefinition of function 'max'`, `'dbl_then_inc'`, `'inc2'` etc.

**Root cause:** Frill frontend generates multiple functions with same name (possibly from pattern matching or lambda lifting).

**Fix options:**
- A) Frill frontend deduplicates at HIR level
- B) LLVM codegen appends suffix to duplicate names: `max`, `max_1`, `max_2`
- C) Skip duplicate definitions (keep first)

**Sent to main session for frontend advice.**

---

## Priority 6: Remaining Ptr Reaching Defs (5 programs)

**Problem:** `'%v396' defined with type 'ptr' but expected 'i32'` in self_parser, self_lanz_parser; similar in Frill/Lizp.

**Root cause:** Reaching definitions pass doesn't track ptr regs correctly. A ptr value flows through a block that expects i32 (or vice versa).

**Fix:** In the reaching definitions pass, also track type information. When propagating, if types mismatch, insert inttoptr/ptrtoint at block entry.

---

## Priority 7: No LLVM Output (18 files)

**Problem:** Frontend can't parse or lower to MIR2.

**Files:**
- Lizp: scheme_r5rs, scheme_test, functional, game_of_life, crypto, dsp (6 files — advanced Lizp)
- PL/M: assert_test, showcase (2 files)
- C89: assert_test, import_test, struct_promote (3 files)
- C99: c99_pointers (1 file)
- Frill: hello.frl (1 file)
- Pascal: logic_test, math_test (2 files)
- Nanz: assert_test, hello_cpm_fib (2 files)

**Triage:** Ask frontend maintainer — deprecate or fix per-file.

---

## Expected Impact

| Fix | Programs Unlocked | New Total |
|-----|:-:|:-:|
| Current | — | 42 |
| P1: Print runtime | +10 | ~52 |
| P2: i32→i32 cast | +7 | ~59 |
| P3: Stderr leak | +5 | ~64 |
| P4: Screen ptrs | +5 | ~69 |
| P5: Frill dupes | +3 | ~72 |
| P6: Ptr reaching | +5 | ~77 |
| **Total projected** | **+35** | **~77** |

From 42 → 77 native binaries = **83% improvement**.
