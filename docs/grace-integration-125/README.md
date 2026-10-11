# CY enforcement and sequential byte-word fusion integration

Original 15_error_enforcement source is unchanged and frozen before measurement.
All four byte-pair entries pass raw/final VM and assembled native in ordinary,
Grace Size and Grace Speed: 786432 outcomes, including A/CY, PC/SP and RAM.
Whole main prints 105 and returns NC for either incoming Carry. Original14
also replays successfully. Combined root test PASS88.975s (14:48.55s,
15:40.39s). Both-public-API native parity is part of this run.

Integrated HIR/MIR2/Nanz PASS23.259/20.266/12.111s. Focused fusion/CY
controls PASS (MIR2 0.009s, pipeline 7.671s). Generated exact catalog now
51 bounded VM/native paths and 86 pending independent oracles, out of194.

Optional byte-word fusion preserves sequential observations and demonstrates
FAT helper benefit; see ../grace-fat-word-fusion.md. Independent review after
integration found a counterexample: TyU8 Const256 executes as zero, while
adjacency proof used raw Imm256. Both matcher and verifier need type-aware
constant interpretation. Fix is in progress; these passing tests are not a
proof against that counterexample, nor a current fullsuite PASS.

Full pipeline and CLI were launched on708e77c5 plus the catalog row before
that correction. Their terminal results will remain attributed to this
checkpoint. No remaining goal requirement is waived.
