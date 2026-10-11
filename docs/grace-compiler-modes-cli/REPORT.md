# Measured compiler modes from the CLI

The ordinary `mz` command can now measure four production combinations of
cross-function register contracts (PFCCO) and MIR Grace, then save the winning
assembly and binary. It uses the existing controlled-workload JSON format and
`MeasureCompilerModeVariants` API. It does not rebuild the winner after measuring.

From the repository root:

```sh
mz examples/nanz/pfcco_weighted_packet.nanz --target cpm \
  --hir-profile-auto ./packet-speed \
  --hir-profile-auto-workload docs/grace-pfcco-prospective-heldout/artifacts/training-workload.json \
  --hir-profile-compiler-modes -o ./packet.bin
```

Add `--Osize` to choose the smallest binary; omit it for the fewest native Z80
T-states. These are separate, strict objectives. Ties keep the earliest mode:

| Mode | ContractOpt (production greedy) | MIR Grace |
|---|---|---|
| 0 | off | off |
| 1 | on | off |
| 2 | off | on |
| 3 | on | on |

Here `--Osize` selects the measurement objective only. Every candidate's compiler
`OptSize` policy is false, so speed and size compare the same four executables.
There is no byte ceiling on Speed. HIR transformations remain disabled in this
search, and independent Grace/profile selectors cannot be combined with it.

`-o` accepts `.bin` or `.a80`, writing exactly the measured winning bytes. It is
optional: the evidence directory always contains `selected.bin`, `selected.a80`,
all four candidates' ASM/BIN/final MIR, the source and imported source bytes,
workload, selection evidence and SHA256 manifest. `source-paths.json` maps the
frontend source-unit names to their saved copies. Nanz imports are resolved with
the ordinary source/stdlib search, reparsed for each candidate, and bound to the
full HIR/ABI/dependency identity. The archival resolver conservatively rejects
ambiguous or unlocatable source units. Input, output and artifact aliases,
including hard links involving imported files, are rejected before output.

All four candidates must pass the controlled MIR VM/native Z80 comparison;
any compilation, oracle or identity failure cancels the selection. This checks
the workload's return values, state and memory access requirements. Source
assertions outside that workload are not an additional verification suite.
Writing the artifact directory is ordinary filesystem I/O, not an atomic
transaction: an I/O error may leave a partial directory and returns an error.

## Actual CLI check

The input is the unchanged PFCCO packet example and the five training calls
frozen in the earlier prospective experiment. This checks a usable CLI workflow;
it is not a new heldout performance experiment.

| Goal | Selected mode | Training T-states | Binary bytes |
|---|---|---:|---:|
| Speed | 1 | 2 320 | 52 |
| Size | 0 | 2 605 | 52 |

The native arithmetic/state/access evidence also passes an independent Python
oracle in `audit.py`. Both real CLI invocations save every candidate; the external
BIN/ASM outputs match the measured winner byte for byte. Four actual compilation
regressions cover both goals and both output formats. An imported arithmetic
helper checks wrapping `250 + 7 = 1`, complete dependency identity and saved
source hashes; an imported hard-link output rejection checks source preservation.
Existing profile JSON/flag/joint-caller checks also pass: 4.947 seconds total.

The frozen CLI executable is recorded in `executable.json` and retained under
`/home/alice/dev/minz-artifacts/grace-checkpoints/mz-compiler-modes`, SHA256
`d49dc066e0f3eb0b29fea3eca7f75b521fc8e463e9e774354352c1cb3d1a4555`.
Rebuilding changes the executable identity recorded in the evidence.

This command is a controlled function-call workload runner targeting CPM's Z80
model. Its saved BIN is the measured assembled image, not a runnable CP/M COM
application with startup/runtime linkage. A user-provided workload remains
necessary; the CLI does not invent representative or heldout inputs.
