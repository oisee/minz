# Exact source LUT admission restores valid profile workloads

Root c2b9a77f admits unchanged source globals or the complete readonly LUT
global list independently regenerated from source MIR. Exact type, alignment,
init bytes, constness, name, order and source-global prefix remain mandatory.
Arbitrary extra globals and partial/reordered/substituted LUTs are rejected.

All five failures from the frozen8bf fullsuite replay now PASS together
on root (23.127s), including sum, loop-cost, factory mutation, popcount and
ranged ABI. Fifteen metadata/source-prefix mutation controls PASS0.012s;
all TestZ80ProfileWorkload boundary/refusal/state tests PASS1.513s.
This is a focused replay, not a full current pipeline PASS.

Four ignored ASM/BIN payloads were missing from the first donor commit;
followup4f729870 tracks them. All14 manifest payloads SHA verified by root.
Saved source/workload/profile/selection/BIN observations and corrected
payloads copied to the share. Historical fullsuite failure log unchanged.

First-class error semantics and sequential-byte fusion remain in isolated
branches; integration/testing required before final goal completion.
