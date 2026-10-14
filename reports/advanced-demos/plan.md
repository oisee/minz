# Advanced demos: implementation and acceptance plan

The full objective is preserved in [objective.md](objective.md). This is one product
milestone with rendering and declarative-library work, not a completion claim.

| Requirement | Evidence required | Current status |
|---|---|---|
| Flicker-free compiled and PUSH sprite demos | Independent screens at every display frame, ROM/ULA timing, dirty writes and IRQ/SP checks | 48K model: baseline detects partial display; dirty renderer 0/64 partial frames in all six variants. Physical display/release pending |
| Tetris removes intermediate actor erasure | Composited frame, dirty publication, independent gameplay and display oracles | 14 cached-render fixtures × 3 modes: final full RAM, gameplay state, IM2 and fetched ULA pixels; baseline detects flicker, composition 0 partial frames. Idle composition skips preparation; downward compose+prepare still 75,504 T, just over a frame; real game input/substeps pending |
| Smooth pixel-step falling piece | Pixel geometry and substep/rotation/locking model, native frames | Pending |
| Optional TV sine/noise/row-shift effects | Deterministic render oracle; game state identical with effects on/off | Pending |
| Declarative typed Nanz, object composition and compile-time interfaces | Real consumers using common reusable APIs; compile-time diagnostics | Pending |
| Shapes, masks, tables and specialized procedures baked | HIR/ASM/BIN inspection, runtime excludes generator and avoidable preparation | Pending |
| Measured selection under Speed/Size/frame budget | Admitted alternatives, preserved winners, heldout native measurements | Full-clear candidates measured; scene-wide automated selection and heldout timings pending |
| Explicit region, timing, IRQ, RAM and SMC contracts | Valid and rejected cases, native memory/PC/SP/IFF proof, beam deadlines | Pending |
| Integration and updated demo release | Main PR/CI, six packaged platforms, playable artifacts and checksums | Pending |

## Work sequence

1. Preserve current ASM/BIN, screen and timing baselines. Add temporal display
   checks: a correct final RAM image does not establish a complete displayed frame.
2. Remove full bitmap clears per sprite frame; restore only the previous actor
   region with a baked library kernel. Commit after native ROM/IRQ checks.
3. Compose Tetris output before publishing changed bytes. Keep input/game logic
   separate from rendering; establish a pixel-substep collision oracle.
4. Build the common declarative actor/scene/effect API in typed Nanz, using
   compile-time interfaces and metafunction specialization. Migrate consumers.
5. Add pixel actors and optional deterministic TV effects, measure admitted
   kernels on independent workloads, and publish the verified winners.
6. Complete requirement-by-requirement audit, integrate in main and release
   all six host packages. Keep logs, source, exact outputs and SHA256 on the share.

No global optimality is claimed: selection means the best admitted candidate for
its explicit objective and measured workload. Deep corpora remain release/nightly
checks; focused temporal/gameplay regressions run on PRs.
