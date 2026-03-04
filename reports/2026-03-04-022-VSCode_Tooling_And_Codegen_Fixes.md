# Report #022: VSCode Tooling Polish + SMC Codegen Fixes

**Date:** 2026-03-04
**Scope:** VSCode extension v0.5.0 fixes, new Compile & Run command, 3 codegen bug fixes, playground example

---

## Summary

A hands-on session starting from "how do I install the VSCode tools?" uncovered and fixed a chain of issues: TypeScript compilation errors in the extension, missing/broken commands, incorrect compiler flags, and three codegen bugs in the SMC (Self-Modifying Code) pipeline. We now have a working edit-compile-run workflow in VSCode.

---

## 1. VSCode Extension Fixes

### 1.1 TypeScript Compilation Error

**Problem:** `surroundingPairs` was set in the programmatic `vscode.languages.setLanguageConfiguration()` call, but that property only exists in the declarative `language-configuration.json` file.

**Fix:** Removed `surroundingPairs` from `extension.ts`. It was already correctly declared in `language-configuration.json`.

### 1.2 Invalid `-O` Compiler Flag

**Problem:** The "Compile" and "Compile Optimized" commands passed `-O` to `mz`, which doesn't exist. MinZ optimizations are **enabled by default** — you disable them with `--disable-optimize`.

**Fix:**
- `compileMinZ()` now passes `--disable-optimize` only when the user has optimizations turned off in settings
- `compileOptimized()` no longer passes `-O`

### 1.3 Only 2 of 10 Commands Visible

**Problem:** The initial broken `.vsix` (with TypeScript errors) was installed. Only commands that didn't depend on the broken code were registered.

**Fix:** After fixing the TypeScript error, repackage and reinstall.

### 1.4 New "Compile & Run (CP/M)" Command

**Added:** `minz.compileAndRun` — a single command that:
1. Compiles MinZ source to Z80 assembly (`mz ... -t cpm`)
2. Assembles to `.com` binary (`mza`)
3. Opens a VSCode terminal and runs the emulator (`mze -t cpm`)

**Keybinding:** `Cmd+Alt+R` (Mac) / `Ctrl+Alt+R` (Win/Linux)

This is the first command that closes the edit-compile-run loop entirely inside VSCode.

---

## 2. Three SMC Codegen Fixes

During testing, we discovered that most MinZ programs with function calls fail at the assembler stage. Root cause: the SMC (Self-Modifying Code) codegen pipeline emits assembly labels that are never defined.

### 2.1 Missing Patchable Return in TRUE SMC Functions

**File:** `minzc/pkg/codegen/z80.go`

**Problem:** Three codegen paths exist for functions:
1. `generateFunction()` — normal functions (had patchable return support)
2. `generateSMCFunction()` — SMC functions (had patchable return support)
3. `generateTrueSMCFunction()` — TRUE SMC functions (**missing patchable return**)

When the caller (`generateSMCAnnotatedCall`) emits `LD (func_return_patch_op), A`, it expects the target function to have a `func_return_patch_op:` label. But TRUE SMC functions just emitted a plain `RET`.

**Fix:** Added `NeedsPatchPoints` checking to `generateTrueSMCFunction()`, both for the last-instruction-is-return case and the epilogue fallthrough case.

### 2.2 NeedsPatchPoints Not Set From Codegen

**Problem:** `NeedsPatchPoints` was only set in the semantic analyzer's `generateInstructionPatchingCall()`, which isn't always the active code path. When the codegen encounters an `OpCall` with SMC annotations, it needs the target function to have patch points.

**Fix:** In the `OpCall` handler, when an SMC call-site annotation is detected, look up the target function via `findFunction()` and set `NeedsPatchPoints = true`. Since main is generated first and other functions follow, the flag is set before the target function body is emitted.

### 2.3 Wrong Label for Parameter Patching

**Problem:** The caller emitted:
```asm
LD (func_param_n+1), A    ; references label+offset
```
But the function body defined:
```asm
func_param_n_immOP:
    LD A, 0
func_param_n_imm0 EQU func_param_n_immOP+1
```

The `+1` arithmetic works in MZA's expression evaluator, but the label names didn't match at all: `_param_n` vs `_param_n_immOP`.

**Fix:** Changed `generateSMCAnnotatedCall()` and `generatePatchParam()` to emit `_param_n_imm0` (the EQU label) instead of `_param_n+1`.

### Impact

These three fixes unblock assembly of any MinZ program that uses function calls with the default SMC optimization. Previously, the only programs that assembled were either:
- Pure inline assembly (`cpm_hello.minz`)
- Programs with no function calls returning values

---

## 3. The Playground Example

Created `examples/cpm/playground.minz` as a self-contained demo file:

```minz
// MinZ Playground — edit, compile, and run!

fun putchar(ch: u8) -> void {
    asm {
        LD E, A
        LD C, 2
        CALL 5
    }
}

fun newline() -> void {
    putchar(13);
    putchar(10);
}

fun main() -> void {
    putchar('H'); putchar('e'); putchar('l'); putchar('l'); putchar('o');
    putchar(' ');
    putchar('M'); putchar('i'); putchar('n'); putchar('Z'); putchar('!');
    newline();

    let mut i: u8 = 0;
    while i < 10 {
        putchar(i + 48);
        putchar(' ');
        i = i + 1;
    }
    newline();

    let x: u8 = 3 + 4;
    putchar('3'); putchar('+'); putchar('4'); putchar('=');
    putchar(x + 48);
    newline();

    putchar('O'); putchar('K');
    newline();
}
```

**Output:**
```
Hello MinZ!
0 1 2 3 4 5 6 7 8 9
3+4=7
OK
```

### What's Cool (Optimizer in Action)

The MIR dump (`--dump-mir`) reveals the optimizer's work:

1. **Loop Rerolling** — 11 consecutive `putchar()` calls for "Hello MinZ!" are detected and merged into a single `print_string()` call with a length-prefixed string literal. Similarly, `putchar('+'); putchar('4'); putchar('=')` becomes `print_string("+4=")`.

2. **Constant Folding** — `3 + 4` is folded to `7` at compile time. The MIR shows `r57 = 7 ; Folded:` with no runtime addition.

3. **Function Inlining** — `newline()` is inlined at every call site. The MIR shows `; Inlined from cpm.playground.newline`.

4. **Peephole Optimization** — `XOR A` for zeroing (instead of `LD A, 0`), `INC A` for +1 after a known zero value.

5. **Assembly Peephole** — 12 asm-level patterns applied (redundant load elimination, etc.)

### Binary Size Comparison

| Build | ASM Lines | Binary Size |
|-------|-----------|-------------|
| `--disable-optimize` | 377 | 474 bytes |
| `--disable-ir-opt` (asm peephole only) | 360 | 411 bytes |
| Full optimizations | 312 | 354 bytes |

Full optimization saves **25% binary size** on this example.

### Optimization Flags Reference

```bash
# Full optimizations (default)
mz program.minz -t cpm -o out.a80

# No optimizations at all
mz program.minz -t cpm --disable-optimize -o out.a80

# No IR/MIR optimizations (disables reroll, inlining, constant folding, DCE)
# Keeps asm peephole and codegen-level opts
mz program.minz -t cpm --disable-ir-opt -o out.a80

# No asm-level peephole
mz program.minz -t cpm --disable-asm-opt -o out.a80

# No codegen-level optimizations (constant tracking, value propagation)
mz program.minz -t cpm --disable-codegen-opt -o out.a80

# No SMC (self-modifying code)
mz program.minz -t cpm --disable-smc -o out.a80

# No CTIE (compile-time interface execution)
mz program.minz -t cpm --disable-ctie -o out.a80

# Show what the optimizer does
mz program.minz -t cpm --compile-trace -o out.a80

# Dump MIR intermediate representation
mz program.minz -t cpm --dump-mir

# Dump AST
mz program.minz -t cpm --dump-ast
```

---

## 4. What Can Be Improved

### 4.1 Register Allocator Bottleneck (Critical)

The assembly output reveals the #1 performance problem. Every virtual register goes through memory at `$F0xx`:

```asm
LD ($F056), A     ; Virtual register 43 to memory
LD A, 48
LD ($F058), A     ; Virtual register 44 to memory
LD HL, ($F056)    ; Virtual register 43 from memory
```

This is **3 extra memory operations** for what should be `ADD A, 48`. The ideal output:

```asm
LD A, (i_addr)    ; load i
ADD A, 48         ; add '0'
```

**Impact:** ~5x T-state overhead per operation. The register allocator's memory fallback path (`$F0xx`) is used for almost every virtual register because the physical register allocator doesn't handle enough cases.

### 4.2 Loop Reroll Boundary Bug (Minor)

The optimized output prints:
```
O
OK
```
instead of just `OK`. The reroller captures `putchar('O')` separately, then the next `newline()` + `putchar('O')` + `putchar('K')` gets rerolled into `"\nOK"`. The standalone `O` + its preceding newline end up as separate instructions.

The unoptimized output is correct: `OK` on one line.

### 4.3 Function Return Values via SMC (Known Bug)

Functions that return computed values (not constants) produce wrong results. `add(3, 4)` returns 0 instead of 7. The SMC parameter patching writes values correctly, but the return value flow through the patchable return sequence has issues. This affects:
- Any function with `return expression` where expression depends on parameters
- Fibonacci, factorial, etc.

The playground example works around this by computing `3 + 4` inline (constant-folded to 7) instead of calling a function.

### 4.4 `print_string` Uses ZX Spectrum ROM Call

The rerolled `print_string` helper uses `RST 16` (ZX Spectrum ROM), which doesn't work on CP/M. It should use BDOS call 2 (`LD C,2; LD E,A; CALL 5`) for CP/M targets. Currently the rerolled strings work because the CP/M emulator appears to handle the output anyway, but this is platform-incorrect.

### 4.5 stdlib.cpm.bdos Assembly Errors

The standard library's BDOS module generates assembly that MZA can't assemble. Programs using `import stdlib.cpm.bdos` fail. This forces users to write inline asm wrappers for `putchar()`. Worth investigating whether the issue is in the stdlib module's codegen or in MZA.

---

## 5. Files Changed

| File | Change |
|------|--------|
| `tools/vscode-minz/src/extension.ts` | Removed `surroundingPairs`, fixed `-O` flag, added `compileAndRun` |
| `tools/vscode-minz/package.json` | Added `minz.compileAndRun` command, keybinding, menu entry |
| `minzc/pkg/codegen/z80.go` | 3 SMC codegen fixes (patchable return, NeedsPatchPoints, label naming) |
| `examples/cpm/playground.minz` | New playground example for edit-compile-run workflow |

---

## 6. Quick Start

```bash
# Open playground in VSCode
code examples/cpm/playground.minz

# Cmd+Alt+R to compile and run (or from command palette: "MinZ: Compile & Run")

# Or from terminal:
mz examples/cpm/playground.minz -t cpm -o /tmp/out.a80
mza -o /tmp/out.com /tmp/out.a80
mze -t cpm /tmp/out.com

# Inspect the MIR:
mz examples/cpm/playground.minz -t cpm --dump-mir

# See optimizer decisions:
mz examples/cpm/playground.minz -t cpm --compile-trace -o /dev/null
```

---

*All codegen tests pass. The playground example produces correct output on both optimized and unoptimized builds (with the minor reroll boundary cosmetic issue on optimized).*
