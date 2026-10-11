# PFCCO: measured native calls, adapters, and saves

The runtime unsigned displacement helper in `examples/nanz/pfcco_weighted_packet.nanz` is called twice per packet. It increments a global counter and returns `(position-base) mod 65536`. The caller keeps the first result live across the second call and returns `first + 3*second mod 65536`. Stateful writes prevent trivial inlining; the regression requires exactly two actual `CALL displacement` instructions.

Seventeen input pairs include zero, reversed operands, equal operands, 255, the signed midpoint, and 65535. Workload/source are saved before any convention choices. The independent Go integer oracle checks all results and the wrapping counter (initially 65530); both raw HIR and each resulting MIR execute it. Actual assembled Z80 runs compare output, counter snapshots, and exact per-byte data-access counts. The source-independent `audit.py` repeats the integer/state/count checks on saved native observations.

| Contract choice | Actual Z80 T-states, 17 calls | Whole BIN bytes |
|---|---:|---:|
| off | 8857 | 52 |
| production greedy | 7888 | 52 |
| PBQP | 7888 | 52 |

Both optimizers save 969 T (10.94%) without increasing binary length. Every input saves 57 T; this is data-independent straight-line call adaptation. T-states include calls, adapter moves, save/restore instructions, and helper execution. Harness setup is separately recorded and excluded consistently for all variants. BIN includes both functions and the global counter.

Off and greedy use actual `CompileHIRSteps` output, with the repaired ContractOpt boolean. PBQP is explicitly experimental: it applies `OptimizeContractsPBQP` to the same off-pipeline MIR, then runs the normal allocator/code generator. It is not made the production default. Saved parameter/return classes are checked against applied classes; returns remain fixed. In the helper, off assigns base=HL and position=DE; both optimized modes choose base=DE and position in the pair class, physically IX. This frees HL for the counter access and removes the original four stack operations needed to reverse the subtraction inputs. Caller adapters still have a cost: those instructions and the live first-result saves remain in the measured output.

This is a reusable two-function workload, not a claim about whole application performance or globally optimal conventions. No profile chooses a winner here; all three predefined modes are measured on all frozen inputs, so this is not a held-out PGO claim. The earlier weighted-u8 candidate had no benefit: all three modes measured 6474 T / 68 B. Its source, workload, output and assemblies remain under `no-benefit` rather than being discarded.

Validation: the focused API/native tests pass (0.09 s native workload); contract/PFCCO unit tests pass (0.022 s). Run `python3 docs/grace-pfcco-native/audit.py` for the saved independent audit. No whole slow pipeline suite was rerun.

Frozen test executable is retained locally at `/home/alice/dev/minz-artifacts/grace-checkpoints/pfcco-native.test`, SHA256 `a701f48ff228cf2a3a8e31f1d5b9bdc64629174ef0063c1a49128aa87653e18a`. Run it with cwd `minzc/pkg/pipeline` and `-test.run '^(TestPFCCOMeasuredNativePacket|TestCompileHIRStepsRespectsContractOption)$'`. JSON records the executable identity. Raw source, workload, baseline/greedy/PBQP ASM/BIN, MIR, allocation, observations and SHA256 manifest are retained here.
