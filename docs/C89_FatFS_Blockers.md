# C89 Frontend: FatFS Compilation Blockers

> Task list for enabling FatFS R0.16 (`examples/c89/fatfs/ff.c`, ~7K LOC) to compile through MinZ C89→HIR→MIR2→Z80 pipeline.

**Status:** 4 blockers identified. All are HIR-level (C89 frontend), orthogonal to LIR backend work.
**Priority:** High — FatFS is the target application for the Nanz FatFS library (`examples/nanz/fatfs_nanz.nanz`).

---

## Blocker 1: `break` inside `switch` (PANIC)

**Severity:** Crash (panic: "break outside loop")
**Occurrences in FatFS:** Extensive (every `switch/case` uses `break`)

**Problem:** The HIR lowerer treats `break` as loop-only. When `break` appears inside a `switch` statement (not nested in a loop), it panics. This is the #1 blocker — FatFS can't even start compiling.

**Root cause:** `switch/case` was implemented (`151beae6`) but the break target stack doesn't include switch as a valid break context.

**Fix approach:**
1. In `pkg/c89/lower.go` (or equivalent HIR lowerer), find the break target stack
2. When entering a `switch`, push its exit block onto the break target stack
3. `break` inside switch → jump to switch exit block (same as loop break → jump to loop exit)
4. `continue` inside switch should still target the enclosing loop (not the switch)

**Test cases:**
```c
// Must compile without panic
int classify(int x) {
    switch (x) {
        case 0: return 0;
        case 1: return 1;
        default: break;  // ← this panics
    }
    return -1;
}

// Nested: break targets switch, continue targets loop
void process(int *arr, int n) {
    for (int i = 0; i < n; i++) {
        switch (arr[i]) {
            case 0: continue;  // → loop continue
            case -1: break;    // → switch break (NOT loop break)
            default: arr[i]++;
        }
    }
}
```

**Files to modify:** `pkg/c89/lower.go` (or `pkg/hir/lower.go` — wherever switch lowering happens)

---

## Blocker 2: `do-while` loops (constprop infinite loop)

**Severity:** Hang (infinite loop in constant propagation)
**Occurrences in FatFS:** 49

**Problem:** `do { ... } while (cond)` loops cause the constant propagation pass to loop infinitely. The README notes this as a known bug.

**Root cause:** Likely the constprop pass doesn't handle back-edges correctly for bottom-tested loops. The fixpoint iteration never converges because the loop body's state keeps changing.

**Fix approach:**
1. In constprop, add iteration limit for fixpoint (cap at N iterations, then stop)
2. Or: detect back-edges and widen lattice values at loop headers (standard technique)
3. Or: skip constprop for functions containing do-while (conservative)

**Test case:**
```c
int sum_do(int n) {
    int total = 0;
    int i = 1;
    do {
        total += i;
        i++;
    } while (i <= n);
    return total;
}
// assert sum_do(5) == 15 via mir2
```

**Files to modify:** `pkg/c89/lower.go` (do-while HIR lowering), `pkg/mir2/constprop.go` (fixpoint)

---

## Blocker 3: `&var` (address-of local variables)

**Severity:** Compile error / wrong code
**Occurrences in FatFS:** 169

**Problem:** `&var` (address-of operator) only works for globals. For local variables, the C89 frontend either errors or produces wrong code. FatFS extensively passes pointers to local structs (`&fs`, `&fp`, `&dir`).

**Root cause:** Local variables on Z80 are either in registers or absolute memory ($F0xx). Taking the address of a register-allocated variable requires spilling it to a known memory location first.

**Fix approach:**
1. When `&var` is used on a local, mark that variable as "address-taken"
2. Address-taken locals must be allocated to memory ($F0xx), not registers
3. The address-of expression returns the memory address as a u16 constant
4. This is a standard optimization barrier — address-taken vars can't be register-allocated

**Test case:**
```c
void fill(unsigned char *p, unsigned char val) { *p = val; }
unsigned char test_addr_of(void) {
    unsigned char x = 0;
    fill(&x, 42);
    return x;
}
// assert test_addr_of() == 42 via mir2
```

**Files to modify:** `pkg/c89/lower.go` (scan for &var, mark address-taken), `pkg/hir/lower.go` (allocate to memory)

---

## Blocker 4: `memcpy`/`memset`/`memcmp` (libc stubs)

**Severity:** Link error (undefined symbols)
**Occurrences in FatFS:** 75

**Problem:** FatFS calls `memcpy`, `memset`, `memcmp` which are C standard library functions. MinZ doesn't have a libc — these need Z80 implementations.

**Fix approach:**
1. Create `stdlib/libc/string.minz` with Z80-optimized implementations:
   - `memcpy` → LDIR (block copy, 21T/byte)
   - `memset` → fill loop or LDIR with self-copy trick
   - `memcmp` → CPI loop or byte-by-byte compare
2. Or: add as `@extern` intrinsics that compile to inline Z80
3. Or: implement in the C89 frontend as builtin functions that lower directly to MIR2

**Z80 optimal implementations:**
```asm
; memcpy(dst, src, n): HL=dst, DE=src, BC=n
__memcpy:
    LDIR        ; block copy DE→HL, BC bytes
    RET

; memset(dst, val, n): HL=dst, A=val, BC=n
__memset:
    LD (HL), A
    LD D, H
    LD E, L
    INC DE
    DEC BC
    LDIR        ; self-copy trick: first byte propagates
    RET

; memcmp(s1, s2, n): HL=s1, DE=s2, BC=n → A=0 if equal
__memcmp:
    LD A, (DE)
    CPI         ; compare (HL) with A, inc HL, dec BC
    JR NZ, .diff
    INC DE
    JP PE, __memcmp  ; BC not zero
    XOR A       ; equal
    RET
.diff:
    LD A, 1     ; not equal
    RET
```

**Files to create:** `stdlib/libc/string.minz` or `pkg/c89/builtins.go`

---

## Execution Order

```
Blocker 1 (switch-break) → unblocks parsing of ~90% of FatFS
Blocker 2 (do-while)     → unblocks remaining loops
Blocker 3 (&var)         → unblocks struct pointer passing
Blocker 4 (libc)         → unblocks linking
                              ↓
                         FatFS compiles!
```

Blockers 1-2 are parsing/lowering fixes (hours of work each).
Blocker 3 is a semantic analysis change (day of work).
Blocker 4 is runtime library code (hours, mostly Z80 asm).

---

## Validation

After all 4 blockers are resolved:
```bash
# Should compile without errors
mz examples/c89/fatfs/ff.c -o fatfs.a80

# Should assemble
mza fatfs.a80 -o fatfs.bin

# Existing construct tests should still pass
go test ./pkg/pipeline/... -run TestLIR_C89Corpus
```

FatFS construct readiness tests are in `examples/c89/fatfs_constructs.c` (292/292 already pass for the subset they cover).
