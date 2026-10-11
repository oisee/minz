# Replay reproduces the recorded winner's training execution

Replay now verifies saved training observations, rather than accepting a
self-signed selection digest as execution evidence. Every candidate must have a
complete training-context identity, ordered invocation names/counts, result/state
snapshots and required exact byte accesses agreeing with the saved MIR VM oracle.
Per-call T-states must be positive; total/bootstrap metrics must be nonnegative;
overflow-safe invocation sums must equal the saved total. Bootstrap digests,
execution limits and final snapshots are checked too.

After recovering the exact recorded winner's ASM/MIR/BIN, replay computes a fresh
MIR VM training oracle and runs **only that selected mode** on the original
training workload. Both the complete oracle and native training evidence must
match their saved records. Only then does replay run the same saved binary on
heldout. It performs no four-candidate search or remeasurement on heldout.
The replay record explicitly contains `training_reproduced: true`.

This reproduces the saved winner; it does not independently establish the cost
of the other three candidates. Their records receive consistency checks, not a
fresh native run. A deliberately fabricated, internally consistent cost for a
nonselected candidate is not authenticated by SHA256. The replay preserves the
recorded winner and confirms its training/heldout behavior; it is not an
independent proof of optimality across unevaluated candidates. Training artifact
hashes provide integrity, not signatures.

Heldout input is owned as bytes before execution and is read again before output
publication. A changed on-disk JSON file is rejected even when the change is
semantically harmless whitespace. Source/import/training-file and mutable
compiled-metadata checks remain in place.

The unchanged PFCCO source and existing frozen five training/twelve heldout calls
again pass through the actual CLI: mode 1, training **2 320 T / 52 B**, fixed saved
BIN heldout **5 568 T / 52 B**. Independent Python checks arithmetic, state,
byte access and exact BIN identity. All saved output hashes agree.

Regression cases include negative totals; a forged positive selected total with
per-call sums, digest and manifest consistently regenerated; forged results;
wrong training context; malformed/omitted manifest entries; modified imports and
source; corrupted selected flags/index/BIN; and heldout file mutation during
factory callbacks. The consistent forged selected timing reaches native training
reproduction and is rejected there. Focused replay tests pass in **4.531 s**.

Fresh executable retained separately:
`/home/alice/dev/minz-artifacts/grace-checkpoints/mz-compiler-mode-replay-trust-final`,
SHA256 `180bd2a027b427a6f582fc18080135a9222a39646b4982c0522ad53e7577506b`.
Earlier reports/executables remain unchanged. This is an admission/reproduction
improvement, not a new heldout optimization experiment.
