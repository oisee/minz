# Prospective arithmetic/call-graph selection with PFCCO

The unchanged existing `examples/nanz/pfcco_weighted_packet.nanz` contains a reusable unsigned displacement helper called twice, with a state counter and the first returned value live across the second call. The caller computes `(b-a)+3*(a-b) modulo65536`. The previous 17 measured pairs are explicitly excluded. Five new training pairs and twelve disjoint new held-out pairs were committed and pushed in **eba3822b before any new candidate measurement**. Both splits begin with counter65532, exercising wrap.

The same public `MeasureCompilerModeVariants` API from the negative tail-recursion experiment runs four actual production modes. All are measured and validated; strict Speed/Size objectives select on training only. No rule, ABI or source is hand-changed to force a win.

| Training mode | Actual T-states, 5 calls | Whole BIN bytes |
|---|---:|---:|
| ContractOpt off, MIR Grace off | 2605 | 52 |
| ContractOpt on, MIR Grace off | **2320** | 52 |
| ContractOpt off, MIR Grace on | 2605 | 52 |
| ContractOpt on, MIR Grace on | 2320 | 52 |

Speed selects mode1: production greedy PFCCO, MIR Grace off (earlier candidate wins the tie with mode3). Size selects mode0 because every BIN is52bytes. Size has no speed tiebreak. The baseline is not the current default: DefaultOptions enables ContractOpt; this explicitly compares the disabled reference against the documented production pass.

On the twelve frozen held-out calls, the **same Speed-selected BIN** runs in **5568 T**, versus **6252 T** for the off baseline: **−684 T, −10.94%, unchanged52 B**. Size's selected original BIN runs in6252T. The native runtime includes helper calls, parameter adapters, live-result saves and counter accesses. A byte-for-byte comparison rejects any winner recompilation drift. These measurements are in the Z80 emulator, not physical hardware.

Independent Go integer logic checks displacement results and wrapping counter on raw HIR; the native harness validates the same outputs/full state/access counts. Portable `audit.py`, with no compiler imports, checks all eight training candidate executions, both heldout runs and both heldout baselines, strict selection, BIN hashes, exact counter state and per-byte read/write counts. It passes. The program has real calls but is a small reusable workload, not a whole arithmetic application or a claim that every family always gains.

This extends prospective evidence beyond unchanged HashMap and full self-Lanz parsing to a call-graph workload. The first arithmetic candidate remains an honest negative result in `grace-arithmetic-heldout`: MIR Grace loses there and automatic selection keeps the original. No workload from that experiment or known interning30 is relabelled heldout. PBQP/table superiority is not asserted; the successful choice is existing production greedy PFCCO.

Frozen executable is local `/home/alice/dev/minz-artifacts/grace-checkpoints/pfcco-prospective-heldout.test`, SHA256 `0b89914d9eee275b49b27b0e4667f080811bf1d3d744fdc2e1008ab2fa04bf16`. Run from `minzc/pkg/pipeline` with `-test.run '^TestPFCCOProspectiveHeldout$' -test.v`. Focused test and independent audit pass. All source/workloads/decisions, all four candidate ASM/BIN/MIR per actual goal, heldout/baseline observations and SHA256 manifest are retained. No whole slow pipeline rerun was done here.
