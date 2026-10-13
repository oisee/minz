# PBQP byte-copy costs: bounded experiment

The structured hardware copy generator exposed an allocation choice: with
source in HL and destination in DE, the emitter saves the live accumulator
with PUSH/POP AF for each byte store. Declaring destination first avoids this.

This experimental branch instead adds a node cost to BC/DE destinations of
byte stores fed by a direct byte load. It estimates the encoding bridge (4T)
and, when a byte ClassAcc value is live, current PUSH/POP AF (21T). Forbidden
locations remain forbidden. Other CPU cost tables do not use this model.
This is a heuristic estimate, not a fully correlated PBQP edge model.

After restoring source-first declarations in hw.copy, the three generator
comparison cases retain destination-first performance: 597T/41B, 30559T/30B,
725T/41B. This is allocation robustness on those cases, not extra speed beyond
PR #96.

Independent workloads: five different lengths, u8/u16 counters, both declaration
orders, with/without a live byte value, three optimization modes, full native
RAM and return PC/SP. All 120 variants pass. The original u8 family is unchanged.
For u16 counters with a live byte value, every tested length/order improves:

| Length | Source first T before/after | Destination first T before/after |
|---|---|---|
| 1 | 501 → 419 | 481 → 429 |
| 3 | 1007 → 821 | 987 → 831 |
| 17 | 4549 → 3635 | 4529 → 3645 |
| 127 | 32379 → 25745 | 32359 → 25755 |
| 255 | 64763 → 51473 | 64743 → 51483 |

Bytes: source-first 84→68, destination-first 78→71. All three compiler modes
have the same results. Costs are from separate baseline/candidate worktrees
with the same independent test source. No candidate was selected using these
heldout lengths. Order still affects entry setup; this is not complete
order-independent allocation or a global optimality proof.

MIR2 core passed in 135.696s; all 346 fast pipeline tests passed in four
processes. The final indexed-definition implementation and expanded workloads
also passed all 346 fast tests; final directed MIR2/pipeline rerun passed. This
experiment is not integrated; the evidence supports this bounded copy family.
Next work needs correlated operand costs and/or edge-aware allocation
candidates, with full boundary moves measured.

Default sweep compares exact output text. Its 838-file checkpoint changed five
ASM outputs but no exit code, stdout or stderr. That is not itself semantic
proof. Additional native tests for original IRC classifiers, CP/M tokenizer
main, checked tokenizer and interning passed in 23.689s. This does not prove
full equivalence of every changed program (notably FatFs and Snake).
