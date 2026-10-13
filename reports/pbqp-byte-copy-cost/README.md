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

Independent workloads: five different lengths, both declaration orders,
with/without a live byte value, three optimization modes, full native RAM
and return PC/SP. All 60 variants pass. Their baseline and candidate costs
are identical. Order still changes entry setup by 6 bytes/20T in this family;
this model does not fix edge/entry allocation moves.

MIR2 core and 346-entry fast pipeline are being checked separately. This
experiment is not integrated or proposed as a general performance win.
Next work needs correlated operand costs and/or edge-aware allocation
candidates, with full boundary moves measured.
