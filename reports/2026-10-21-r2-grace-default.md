# R2: Grace defaults and infinite-loop preservation

Baseline: `origin/main` / `840b1c465` (the worktree's starting revision).

## Step 1: preserve reachable infinite loops

The root cause is MIR Grace's `empty-block-elim` action
(`redirectAndDelete`), not HIR Grace or return-based reachability.
For an empty `cycle: jmp cycle`, redirecting predecessors still targets
`cycle`; removing that block then leaves dangling edges. The sibling
`param-forward-elim` action (`elimParamForward`) has the same problem for
self-loops with block arguments. Both actions now reject self-targeting
jumps. Multi-block cycles can still collapse to a valid self-loop.
`EliminateDeadBlocks` already traverses forward from entry and needs no change.

Failing-first tests ran with all compiler files still identical to the baseline:

- `TestGraceKeepsSelfLoopTargets`: both plain and argument-carrying self-loops
  fail verification with `branch to unknown block @cycle`.
- `TestGracePreservesInfiniteLoops`: C `for (;;)` / `while (1)` and Nanz
  `loop {}` / `while true` empty loops fail with MIR Grace, and pass with
  HIR Grace alone. The exact C23 example fails with the reported
  `branch to unknown block @loop_body2` on both assertion backends.
- Side-effecting loops use an external call; MIR VM counts repeated calls
  until its gas limit. A bounded native execution prefix checks that each
  loop stays in its assembled program. Empty VM loops are checked by CFG
  traversal because MIR VM terminators do not consume gas.

Logs: `/mnt/ssd-510/minz/bench/r2-logs/both-before.log` (clean baseline;
initial fixture syntax mistakes were corrected before this replay).

Explicit patch reversal (`git diff` / `git apply -R`), followed by the same
focused tests, reproduced both failures; the patch was restored on exit.
Log: `r2-logs/step1-reversed.log`. With the fix, both new tests pass, along
with `TestGraceDJNZExitRedirectKeepsTarget`,
`TestGraceRunner_PointerThreadingRealLoopShape`,
`TestMIRGraceAssemblyAPIUsesSameNativePipeline`, and
`TestHIRGracePointerWalkAssertions` (`r2-logs/step1-after.log`).

## Step 2: production defaults

Validation and benchmark results will be recorded after the separate default switch.
