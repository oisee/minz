# Claude Task Template

Use this template for bounded Claude tasks so the result stays reviewable and does not drift.

## Goal

State one narrow goal only.

Good:

- Implement Shape A pointer threading for 2-block counted loops.
- Diagnose why `tetris_cpm` under `mze` never reaches keyboard input.

Bad:

- Improve codegen quality.
- Fix Tetris and TUI.

## Scope

List exactly what Claude may touch.

- files or directories
- one transform family
- one report

Example:

- Work only in `/home/alice/dev/minz/minzc/pkg/mir2`.
- Touch only `pointer_threading.go` and related tests.
- Write one report to `/home/alice/dev/minz/reports/...`.

## Non-goals

State what must not happen.

- no EXX
- no wider redesign
- no unrelated cleanup
- no changes in other repos
- no extra docs beyond the requested report

## Done Criteria

Make completion checkable.

- one positive test
- one negative test
- no broadened scope
- real end-to-end or focused proof
- report written

Use the formula:

- Done only if A, B, C are all true.

Example:

- Done only if Shape A fires on the targeted test, Shape B is rejected, and focused MIR2 tests pass.

## Required Output

Always require a concrete artifact.

- report path
- commit id if requested
- exact final line

Recommended:

- Write short report to `/home/alice/dev/minz/reports/NAME.md`
- Reply exactly: `DONE: /home/alice/dev/minz/reports/NAME.md`

## Review Questions

Ask for the facts needed to review quickly.

- What exact pattern was recognized?
- What exact rewrite was performed?
- Which files changed?
- Which tests were run?
- What remains as the next step?

## Preferred Task Shapes

Claude works best when the task is one of these:

- bounded implementation
- narrow diagnosis
- focused design note
- proof-of-hit / proof-of-non-hit

Avoid giving Claude mixed tasks like:

- implement + redesign + measure + document + refactor the same subsystem

## Good Prompt Skeleton

```text
Goal:
Implement only <one narrow thing>.

Scope:
- touch only <files/dirs>
- write report to <path>

Non-goals:
- do not touch <X>
- no broader redesign
- no unrelated cleanup

Done only if:
1. <checkable condition>
2. <checkable condition>
3. <test/proof condition>

At the end reply exactly:
DONE: <report path>
```

## Review Labels

Use these labels after review:

- `groundwork` — useful substrate, no user-visible win yet
- `real win` — actual pipeline/runtime effect
- `not enough` — too broad, too vague, or not wired

This keeps progress honest and prevents “description x10, effect x1”.
