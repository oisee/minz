# Fixed winning compiler-mode artifact on a heldout workload

The CLI now runs a previously selected compiler-mode artifact on another
controlled workload, without searching again or changing the selected mode.

From the repository root, use the same `mz` executable for both commands:

```sh
mz examples/nanz/pfcco_weighted_packet.nanz --target cpm \
  --hir-profile-compiler-modes --hir-profile-auto ./packet-training \
  --hir-profile-auto-workload docs/grace-pfcco-prospective-heldout/artifacts/training-workload.json \
  -o ./packet-training.bin
mz examples/nanz/pfcco_weighted_packet.nanz --target cpm \
  --hir-profile-compiler-modes-replay ./packet-training \
  --hir-profile-auto ./packet-heldout \
  --hir-profile-auto-workload docs/grace-pfcco-prospective-heldout/artifacts/heldout-workload.json \
  -o ./packet-heldout.bin
```

Replay requires the same compiler executable, source-unit names/paths, complete
source/import bytes, HIR/ABI and training context. A rebuild or relocation that
changes source-unit identity requires fresh training. It accepts the existing
`profile-workload/v1` schema; byte host callbacks require the separate memory-host
workflow. `--Osize`, the search flag and other selector modes cannot alter replay.

Training directories contain saved ASM/BIN and MIR text, rather than a serialized
compiled Steps graph. Replay therefore compiles **only the recorded winning
mode** to recover call ABI/allocation metadata. Its ASM and final MIR must match
the saved winner exactly; assembling the saved ASM must produce the saved BIN
byte for byte. The existing `RunZ80ProfileWorkload` API executes the saved ASM with
the recovered metadata, and the executed BIN hash must equal the saved BIN hash.
There is no candidate search on the heldout input.

Admission verifies every required manifest entry, payload hash, complete source
copies, selection digest, fixed four-mode options/compilation IDs, selected index
and saved winner. It rejects omitted entries, unexpected files, symlinks or
paths escaping the training directory. Source/import semantics and training
artifacts are checked again after execution; mutable compiled MIR/allocation
metadata is snapshotted and checked too. Digests provide integrity, not a signed
trust boundary against deliberately fabricated complete evidence.

Replay saves a new workload, MIR VM oracle, native run, selected ASM/BIN and replay
record referring to the training selection and exact BIN hashes. The new directory
uses atomic no-replace publication; `-o .bin/.a80` saves the exact chosen artifact.
An existing replay directory is refused. The ordinary independent-path atomic
publication limits remain: this is not one transaction across directory and `-o`.

## Actual public CLI proof

The unchanged PFCCO packet source and existing prospectively frozen inputs are
used: five training calls and twelve heldout calls. The inputs were not selected
or modified for this CLI experiment.

| Stage | Mode | Z80 T-states | Bytes |
|---|---|---:|---:|
| Training search winner | 1: ContractOpt on, MIR Grace off | 2 320 | 52 |
| Heldout replay of saved winner | same mode and exact BIN | 5 568 | 52 |

Both external BIN files and both directories' selected BIN files are identical:
SHA256 `23a46320f4d5adaf3a706457cf721145f0ff0df85efcd4c8e0e2b0f0d83642f0`.
An independent Python oracle verifies modular arithmetic, the persistent trace
counter and two byte reads/writes per counter byte for all twelve calls. It also
checks winner identity, replay hashes and manifests. All checks pass.

Native CLI regression tests cover the complete training→heldout flow and
independent arithmetic/state expectations. Negative cases cover corrupted BIN,
manifest omission, corrupted selected index/options with regenerated selection
hashes, changed primary/imported sources, selector/objective conflicts and writes
inside training artifacts. Focused checks pass in 3.023 seconds; final replay and
memory-host admission checks pass in 2.925 seconds.

Every training candidate and heldout result is saved here. Frozen final CLI:
`/home/alice/dev/minz-artifacts/grace-checkpoints/mz-compiler-mode-replay-final`,
SHA256 `fd1834f762935a3413738d0fabc429a5ba5cd23c732f40ee12a2224f8d769047`.
This adds a reusable user workflow to the earlier heldout evidence. It does not
claim a new optimization, a new heldout population or globally optimal modes.
