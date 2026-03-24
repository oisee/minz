# Session Extract: VIR Solver 520/520 = 100% (2026-03-23 afternoon)

## What We Did

Starting from 504/520 (97%) VIR corpus coverage, we closed the gap to 520/520 (100%) in one session.

### 1. 16-bit div/mod inline runtime
- `translateDivMod` in bridge.go now handles w=16: arg0→HL, arg1→DE
- Runtime stubs `__div16`/`__mod16`: shift-and-subtract long division, 16 iterations
- Uses `ADC HL,HL / SBC HL,DE` — the Z80 sweet spot for 16-bit division
- Inlined per call site in peepholeCleanup with unique labels (`.vir_div16_0:`, `.vir_div16_1:`, etc.)
- Also added `__mul16` inline expansion (was missing, only had shared routine)
- Unlocked: `__tag`, `__payload` (ADT u16 div), `is_even`, `mod10` (C89), `wr16`

### 2. OpAsmBlock passthrough
- New VIROp opcode `OpAsmBlock` with `AsmTemplate`, `AsmIns`, `AsmOuts` fields
- New PIROp field `AsmText` — when set, `Emit()` returns it verbatim
- `translateAsm` in bridge.go: MIR2 `OpAsm` → VIR `OpAsmBlock`
  - Conservative clobbers (all GPR + flags)
  - Input vregs pinned to contract-mapped locations via SrcHint
  - Template split on "/" into individual asm lines
- Solver treats OpAsmBlock like OpCall (clobbers, no pattern substitution)
- `splitVRegsAtCalls` and `insertCallSaveRestore` now handle OpAsmBlock too
- Removed the `HasAsm` skip in `CodegenModule`
- Unlocked: 9 functions (putchar, _putch, _esc, _dec, _puts, tui_read_key, main, _p, tui_puts)

### 3. Report + README + Philipp reply
- `reports/2026-03-23-109-VIR-100-Percent-Showcase.md` — full showcase with Z80 asm
- `research/abi-paper/philipp-reply-2026-03-23.md` — updated reply
- README featured with new benchmark table

## Wisdom Earned

### 1. Inline asm doesn't need solver integration — just isolation
The key insight for OpAsmBlock: don't try to understand the asm. Model it as a black box with clobbers. The solver optimizes *around* it — register placement before and after the asm block. The asm template is emitted verbatim. This is cleaner than GCC's approach (which tries to participate in register allocation for asm operands).

### 2. Inline runtime > shared routines for small CPUs
On Z80, CALL costs 17T + 10T RET = 27T overhead. An 8-bit div loop is ~160T. The CALL overhead is 17% of the work! Inlining eliminates this AND lets the solver know the exact clobber set, so it can keep vregs in non-clobbered registers. The tradeoff: code size grows by ~15 bytes per call site. On Z80 with 64KB, this is almost always worth it.

### 3. Per-instruction location variables are the key innovation
The `lv{vreg}_i{inst}` encoding is what makes the solver work for real programs. A vreg can be in A at instruction 3, IXH across a CALL at instruction 5, and B at instruction 7. The solver plans the moves *as part of the optimal solution*. Traditional compilers would need separate spill/reload logic with heuristics.

### 4. Z3-PFCCO is the SDCC killer
SDCC's biggest weakness isn't its instruction selection — it's the fixed calling convention. Every function pays stack push/pop overhead. Z3-PFCCO picks the optimal register for each parameter module-wide. `select_b(a,b)→u8` (dead-code elimination test: `let t=a; return b`) gets params in A,C so the body is just `LD A, C / RET`. SDCC: 20 instructions (can't eliminate dead code across stack ABI). This is a 10x improvement from one optimization.

### 5. The "/" separator in asm templates
MIR2's AsmExtra.Template uses "/" to separate instructions in a single-line asm block: `LD E, A / LD C, 2 / CALL 5`. VIR's `splitAsm()` splits on "/" and emits each part as a separate indented line. Edge case: don't split if there's no "/" (single instruction).

## Current Numbers

| Metric | Value |
|--------|-------|
| Nanz corpus | 216/216 (100%) |
| C89 corpus | 304/304 (100%) |
| Total | 520/520 (100%) |
| Z80-verified asserts | 55/55 |
| VIR vs SDCC | -60% (wins 4/5) |
| PBQP fallback | 0 functions |

## Known Issues

1. **abs_diff CFG solver unsat** — Falls back to per-block mode, producing 13 insts vs optimal 9. The CFG encoding hits an edge case with conditional paths. Not blocking (still correct, just suboptimal).

2. **Non-deterministic coalescing** — Go map iteration in `coalesceVRegs` causes different merge orders. Known flaky on AbsDiff test. Need sorted vreg iteration.

3. **fib_parallel_copy** — Loop back-edge uses sequential moves instead of parallel copy. Adds 1-2 instructions. Not blocking correctness.

## Files Changed

- `minzc/pkg/vir/vir.go` — OpAsmBlock opcode, AsmTemplate/AsmIns/AsmOuts fields on VIROp, AsmText on PIROp, splitAsm(), Emit() verbatim path
- `minzc/pkg/vir/z80.go` — asm_block pattern entry
- `minzc/pkg/vir/bridge.go` — translateAsm(), translateDivMod w=16, removed error bail
- `minzc/pkg/vir/solver.go` — OpAsmBlock in splitVRegsAtCalls/insertCallSaveRestore, AsmText in parseSolution/parsePerInstSolution
- `minzc/pkg/vir/pipeline.go` — Removed HasAsm skip, __div16/__mod16 stubs, inline expansion for div16/mod16/mul16
