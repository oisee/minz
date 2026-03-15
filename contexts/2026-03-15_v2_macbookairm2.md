# Session 2026-03-15 v2 — macbookairm2 (alice-2.local, arm64, macOS 15.5)

## Final State

**Tetris v2: assembles clean, ABI verified, screen black (BUG-014)**

### Key Discovery (end of session)
```
mzd --verify-abi /tmp/tetris.a80 /tmp/tetris.code
→ 31/31 functions matched, all OK
```
ABI contracts are CORRECT. Black screen is a **logic/value bug** in codegen,
not a register assignment bug. Narrows BUG-014 root cause significantly.

### MIR2 VM Assertions
```
tetris_mir2_test.nanz — all pass:
piece_dx(0,0,0)==0, piece_color(0)==40, color_attr(0)==0, board_get(0,0)==0
```
HIR→MIR2 lowering is CORRECT. Bug is in MIR2→Z80 codegen only.

## Commits (this session, chronological)
```
b1cf9cd fix: tetris v1 & v2 compile and assemble to runnable binaries
43ddcc6 feat: output format flag (-f), computeDeadConsts u16 fix, showcase test
e6e9b7b chore: remove duplicate lizp files
037fbc5 fix: array index load + caller-save PUSH/POP
dca20c3 docs: Open_Bugs_RCA BUG-009/010/011
3d18c28 test: add Pascal to showcase test
f50ea08 fix: OpAddrOf 8-bit guard + emitMov pair↔8bit
47f277c fix: InlineTrivial ptr-op + ADD HL,IX guard → 0 asm errors
00eef29 docs: Open_Bugs_RCA BUG-012/013/014
daa72cf test: MIR2 VM tetris assertions + single tetris.nanz
e8e39c3 chore: contexts/ directory
a57b31c chore: rename cmd/mzv → cmd/mzv1
```

## Neighbor's Commits (merged)
```
5781677 feat: Lizp frontend (522 LOC, 17 tests)
a54ac36 feat: cross-language import (.lizp ↔ .nanz)
7c62a29 feat: Pascal frontend (TP3.0 subset)
44f2600 feat: Pascal auto-import system unit
1679c5a feat(mzd): --regs IN/OUT/CLOBBER annotations
1fc4cfa feat(mzd): --verify-abi compiler↔binary comparison
```

## Bugs: 14 tracked, 11 fixed, 3 open
| Open | Description |
|------|-------------|
| BUG-001 | GCD parallel-copy bloat (needs PBQP affinity edges) |
| BUG-008 | Arena IXL,(IX+d) (needs allocator+codegen fix) |
| BUG-014 | Tetris black screen — ABI correct, MIR2 correct, codegen value bug |

## Next Session Plan

### 1. New cmd/mzv — MIR2 VM with Ebitengine graphics
- Same graphics lib as mzx (Ebitengine)
- MIR2 VM executes tetris.nanz directly
- IO hooks: OpAsm → callbacks (OUT=border, LD(HL)=memory)
- Screen: 256×192 pixel buffer + 32×24 attr grid
- Input: Ebitengine keyboard
- Modes: windowed (Ebitengine) / headless (--screenshot) / --dump-keyframes
- TAS replay via pkg/tas/

### 2. Debug BUG-014 (tetris black screen)
- ABI verified OK → bug is in codegen VALUES, not register assignment
- Profiling: fill_cell only 35 calls (expected 1600+)
- Check: fill_cell loop body — does PUSH/POP restore correctly?
- Check: zx_screen_addr return value — correct screen address?
- Try: add border markers inside fill_cell loop body

### 3. Feature branch: review/mir2-codegen-hardening
- ADR-0020 EdgeCost (PBQP instruction-level constraints)
- TRS peephole refactor (Report #077)
- Validation pass: every Z80 instruction checked before emit
- mzd --verify-abi in CI pipeline

## Test Infrastructure
- TestShowcaseCompileAssemble: 89 files, 5 frontends (nanz/lanz/lizp/plm/pascal)
- MIR2 VM assertions: tetris_mir2_test.nanz
- mzd --verify-abi: 31/31 functions pass
- All 28 Go packages pass (TSMC benchmark flaky, pre-existing)

## Reproduce
```bash
cd minzc
make all                                    # build all tools
go test ./pkg/... -count=1 -vet=off         # run all tests
./mz ../examples/zx/tetris.nanz -o /tmp/t.a80
./mz /tmp/t.a80 -f code -o /tmp/t.code
./mzx --run /tmp/t.code@8000 --set EI,IM=1  # interactive
./mzx --run /tmp/t.code@8000 --set EI,IM=1 --frames 500 --screenshot /tmp/s.png
./mzd --verify-abi /tmp/t.a80 /tmp/t.code   # ABI check
```
