# Isolated status

Compiler-owned snapshot capability remains experimental, unmerged. Supported runtime proof is deliberately narrow: one AF snapshot, one byte result from A to a caller-saved non-A/F byte, exact fixed return/flags restoration sequence. Other generated result ABI forms refuse attestation. No generic IX or numeric stack permission is added.

The source body and descriptor are not optimization certificates. Exact typed frame SHA binds emitted finite instruction/slot records; actual opcode bytes, frame initialization, caller-save PUSH roles, current SP, anchored IX and frame retirement are checked separately. Original30 baseline remained 1221494T/2091B with unchanged binary in focused verification. The arena-peel candidate still has no accepted measurement until independent review authorizes a fresh isolated run.

Focused positive and adversarial tests pass. Independent review identified typed byte-source and flags-restoration gaps in earlier WIP snapshots; history is retained, and current proof now independently derives the entire allowed byte-result restoration sequence. General compiler tests have not run; parent production remains untouched.
