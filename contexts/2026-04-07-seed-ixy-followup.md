# Next Session Seed — 2026-04-07

**Previous:** IXY half-reg codegen fixes (LD + BIT/SET/RES), Tetris spawn-path diagnosis, VIR edge-move root cause.
**Wisdom:** [contexts/2026-04-07-wisdom-ixy-codegen-fixes.md](2026-04-07-wisdom-ixy-codegen-fixes.md)

---

## Context

MinZ is a Z80 compiler with 8 frontends. This session fixed two classes of invalid Z80 instructions in the MIR2 codegen (IX half-reg in LD and BIT/SET/RES), and diagnosed the Tetris immediate-game-over as a VIR CFG solver edge-move bug (fixed in minz-vir by codex).

## Read First

1. `CLAUDE.md` — project overview, current priorities
2. `reports/2026-04-07-Claude-IXY-Half-Opcode-Audit.md` — what was fixed and what's latent
3. `reports/2026-04-06-Claude-FatFS-Precision-Blockers.md` — remaining FatFS blockers
4. `reports/2026-04-07-Next-Sprint-Plan-RU.md` — two-week plan with refs

## What To Do Next (in priority order)

### 1. Verify Tetris with VIR edge-move fix from minz-vir

Codex applied a 3-bug fix in minz-vir (bridge.go, cfgsolver.go, pipeline.go). If merged/synced to minz:
- Recompile `examples/nanz/tetris_cpm.nanz`
- Run under mze: does the game loop enter? Does render() produce VT100 output?
- If yes → run Probe A/B/C from `reports/2026-04-07-Claude-Tetris-Probe-Cutpoints.md`

### 2. SRL/SRA/SLA IX half-reg audit (latent bug, parked)

Same class as BIT/SET/RES. CB prefix encoding doesn't support IX halves. Not yet triggered in test corpus but structurally vulnerable. Audit `genShift` in z80codegen.go. Small fix.

### 3. FatFS blocker #2: `&local_var` address-taken

169 occurrences in ff.c. Needs address-taken marking in semantic analysis → force local to $F0xx memory with emitted label → `OpAddrOf` resolves. Medium effort.

### 4. FatFS blocker #3: u8 truncation in QBE

Insert `& 0xFF` masks in QBE emitter for u8 operations. Restores QBE as correctness oracle. Small fix.

## What To Avoid

- Don't start broad VIR cfgsolver surgery — the edge-move fix was done by codex in minz-vir
- Don't modify VIR solver/pipeline without checking minz-vir first (repos may diverge)
- Don't run full FatFS compilation without checking cycle cost first (feedback: prefer smallest repro)
- Don't fix `TestStrLenZ80` — pre-existing, unrelated string DB format issue

## Active Coordination

- `~/dev/minz-vir` — codex session handles VIR backend, enriched tables, pointer threading
- Use `dedelulu explore` to find current session ID before sending
- Always reply via dedelulu when completing cross-session tasks

## Corpus Status Post-Fix

- All Z80-VALIDATE errors eliminated for `stdlib/fs/fat12.minz`
- FatFS differential test: assembly still fails (JP instruction issue), but no IX/IY invalid ops
- MIR2 test suite: 1 pre-existing failure (`TestStrLenZ80`), all others pass
- Tetris: compiles clean, assembles, runs without crash (immediate game_over is VIR bug, fixed in minz-vir)
