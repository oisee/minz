# Advanced demos: implementation and acceptance plan

The full objective is preserved in [objective.md](objective.md). This is one product
milestone with rendering and declarative-library work, not a completion claim.

| Requirement | Evidence required | Current status |
|---|---|---|
| Flicker-free compiled and PUSH sprite demos | Independent screens at every display frame, ROM/ULA timing, dirty writes and IRQ/SP checks | 48K model: baseline detects partial display; dirty renderer 0/64 partial frames in all six variants. Physical display/release pending |
| Tetris removes intermediate actor erasure | Composited frame, dirty publication, independent gameplay and display oracles | 14 cached-render fixtures × 3 modes: final full RAM, gameplay state, IM2 and fetched ULA pixels; baseline detects flicker, composition 0 partial frames. Idle composition skips preparation; immediate global-array addresses give baked16 downward compose+prepare+publish 44,724 T, at a 22,730-byte module cost; full board refresh exceeds a frame; real game input/substeps pending |
| Smooth pixel-step falling piece | Pixel geometry and substep/rotation/locking model, native frames | Typed `FallingPose` consumes actual game geometry; all 28 shapes × 8 phases VM/Z80, pixel contact oracle; staged publication 3,653 T. Bitmap/attribute composition and game-loop integration pending; [report](tetris/pixel-actor.md) |
| Optional TV sine/noise/row-shift effects | Deterministic render oracle; game state identical with effects on/off | Pending |
| Declarative typed Nanz, object composition and compile-time interfaces | Real consumers using common reusable APIs; compile-time diagnostics | Pending |
| Whole-sprite declarative Nanz API | One real multi-row sprite with size, pixels and mask; one draw call with coordinates; no public row-setter/address bookkeeping | First whole-sprite API verified: `Ship{x,y}.draw()`; 444 native and 888 VM calls; 99 fetched ULA frames in three modes. Byte-aligned X, arbitrary pixel Y; Tetris migration pending |
| Shapes, masks, tables and specialized procedures baked | HIR/ASM/BIN inspection, runtime excludes generator and avoidable preparation | Native rows and whole 16×8 sprite verified; checkpoint 4849b2ed: 596–597-byte full module, 3,847–4,185 T including placement and drawing; current staging API: 748 bytes, 3,588–3,644 T for direct draw; [whole-sprite report](whole-sprite/README.md). Wider workloads/selection pending |
| Checked SP-stream alternatives | Ordered memory effects, exact SP restoration, actual machine clobbers, native oracle and measured benefit | Astra research completed; automatic PUSH/POP selection pending |
| Measured selection under Speed/Size/frame budget | Admitted alternatives, preserved winners, heldout native measurements | Full-clear candidates measured; closed 15-candidate scene receipt selection preserves Speed/Size/bounded winners and rejects all-fixture frame budget; compiler integration/heldout timings pending |
| Explicit region, timing, IRQ, RAM and SMC contracts | Valid and rejected cases, native memory/PC/SP/IFF proof, beam deadlines | Pending |
| Integration and updated demo release | Main PR/CI, six packaged platforms, playable artifacts and checksums | Pending |

## Work sequence

1. Preserve current ASM/BIN, screen and timing baselines. Add temporal display
   checks: a correct final RAM image does not establish a complete displayed frame.
2. Remove full bitmap clears per sprite frame; restore only the previous actor
   region with a baked library kernel. Commit after native ROM/IRQ checks.
3. Compose Tetris output before publishing changed bytes. Keep input/game logic
   separate from rendering; establish a pixel-substep collision oracle.
4. Build a complete sprite description and unified draw API in typed Nanz,
   using compile-time interfaces and metafunction specialization. Keep row
   kernels and address/setter bookkeeping internal. Validate size, masks,
   coordinate mapping and full displayed frames on a real multi-row example.
   Migrate the Tetris actor; extend scene/effect composition from that consumer.
   Admit SP/PUSH kernels only with memory-order, liveness and SP/INT proofs.
5. Add pixel actors and optional deterministic TV effects, measure admitted
   kernels on independent workloads, and publish the verified winners.
6. Complete requirement-by-requirement audit, integrate in main and release
   all six host packages. Keep logs, source, exact outputs and SHA256 on the share.

No global optimality is claimed: selection means the best admitted candidate for
its explicit objective and measured workload. Deep corpora remain release/nightly
checks; focused temporal/gameplay regressions run on PRs.

## Temporal shadow checkpoint — 14 October 2026

The whole-sprite consumer now has a verified shadow compositor on Spectrum 48K:
201 fetched frames across three compiler modes, no partial frames; the early
publication control detects two mixed frames. Conditional-call flag lowering
and an unsafe CALL peephole are fixed (`72ca40b1`). MIR2 and all 393 fast pipeline
entries pass. The deeper native band matrix also passes independently.

See [the measured renderer report](tetris/temporal-shadow.md). The playable main
still uses the previous renderer. Pixel gravity/input integration, preparation
cost reduction, the new 0x6000-code/0xE000-shadow playable build, main integration
and release remain required. This checkpoint does not complete the goal.

## Playable pixel checkpoint — 14 October 2026

The actual main now runs pixel gravity/input, baked sprite instances and shadow
publication. Three complete native Spectrum runs each verify 39 game steps,
24 fractional poses, 10 locks and 359 fetched field frames without mixing;
game-over returns with restored SP. Independent physics adds 1596 cases per
mode across native Z80 and raw/final VM. The embedded Tetris candidate and
reproducible builder now use code at 6000, shadow at E000 and stack at FCE0.

See [the gameplay report](tetris/smooth-gameplay.md). Earlier table/checkpoint
statements describe their historical versions. Preparation cost reduction,
main integration and six-host publication are still required; the goal is active.

## Native preparation checkpoint — 14 October 2026

The measured winner walks across dirty rows and uses an eight-store typed Nanz
library kernel. Full-field restoration falls 403569→183229 T; complete renderer
with waiting falls 749064→539472 T on the same 14 scenes. All three modes pass
physics, full memory, the temporal negative control and 323 actual main field
frames each. The reusable fill kernel additionally passes all byte values and
screen/shadow boundary positions on native/raw/final VM.

See [the preparation report](tetris/native-preparation.md). Main integration and
six-host release remain required; 50-FPS rendering is not established.
