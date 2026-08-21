# 2026-08-21 Wisdom: four audits, six fixes, and what they taught

## The one thing to carry forward

**We could not see our own failures, and that mattered more than any bug we found.**

`pkg/vir` took ~25 minutes and `pkg/pipeline` never finished, both against `go test`'s 10-minute
default. So `TestVIR_Assert_GCD` — a test in our own repo, asserting that `gcd(12,8) == 4`, failing
because the default backend returns 0 — had been red and invisible. Corpus gates logged their rate
and passed unconditionally. `LABEL AUDIT` printed undefined labels and exited 0. 96% of asserts ran
on the MIR2 VM, a u16 interpreter that masks exactly the register-allocation and flag bugs we kept
hunting by hand.

Fixing visibility took one afternoon and turned a 45-minute run that never completed into
`make test-quick`: **49 seconds, 47 packages ok, 9 failing tests named.** Do that first, always.

## Verification lessons

**`go build` does not compile test files.** Removed `pipeline.Options.UseVIR`, saw a clean
`go build ./pkg/... ./cmd/...`, pushed — and `fun_import_test.go` still referenced the field. After
deleting a public field, run `go vet ./...`, not `go build`.

**`head` at the end of a pipeline masks `grep`'s exit code.** `grep -q ... | head -2 && echo FOUND`
prints FOUND unconditionally, because `head` exits 0 on empty input. This produced six false
"Pascal emits LD L,(A)" hits; the real count was one.

**zsh aborts the whole command when any glob fails.** `ls a* b* c*` with no `b*` match runs
nothing, not two-thirds of it. Looks exactly like a hang.

**Agents are wrong often enough to matter.** Of four survey agents, one claimed `cfgsolver.go`
calls none of the four pre-passes (it calls two), one produced a four-row table of miscompiling
loops that did not reproduce, and one swept 254 multipliers and reported no carry dependence for a
sequence that demonstrably has one. Each report's *core* finding held. Verify the load-bearing
claim before writing it down; mark the rest `(unverified)` with a reproduce-first step.

**Never `git stash push <path>` inside a `&&` chain that also `cd`s.** The push silently did
nothing, the `pop` then took someone else's stash and spilled conflict markers into three files.
Separately, a subagent saw my uncommitted edit, assumed a read-only violation, and reverted it.
Commit before doing anything clever; both losses were survivable only because I had.

## Codegen lessons

**A "provably optimal" GPU-searched sequence is optimal under its fixture.** The mul tables fixed
CY=0 in test vectors instead of sweeping it, so `a * 23` emits `LD B,A / RLA / ...` and returns 161
with CY clear, 169 with CY set. `k=2` is tabled as `RLA` where `ADD A,A` is the same 4T and
carry-safe. When generating tables, sweep every input the sequence can read — including flags.

**Codegen depended on the working directory.** `mul_table.json` loaded by bare relative path, so
the same binary on the same source emitted an 8-instruction GPU sequence from the repo root and a
generic ~80T `__mul8` loop from anywhere else. Any benchmark taken before noticing that is
unfalsifiable.

**Loaders that `continue` on error produce silent wrong answers.** `LoadEnrichedBinary` accepted a
Z80T v1 file through the ENRT parser, desynchronized on record one, loaded 2,052 of 156,506, and
**returned success**. Validate the header and check that the parsed count matches what it declares.

**z3 can reject part of a query and still say `sat`.** A malformed term draws `unsupported`, z3
drops it, solves the rest, reports success. PFCCO's bare top-level `(ite …)` had been silently
losing the bool-return cost term this way. Treat any solver complaint as an error.

## Claims lessons

**Our research documents were honest; our summaries were not.**
`research/abi-paper/philipp-reply-2026-03-25.md` already contained an "Honest Caveats" section
anticipating most of what the prior-art audit found. `CLAUDE.md` carried the enthusiastic version.
The correct wording existed — filed in a letter instead of the file everyone reads.

**Naming a technique after what you did not implement is the most expensive kind of overclaim.**
"WFC" promises minimum-entropy ordering and backtracking. `Entropy()` has zero call sites,
`Collapse()` iterates in program order, and there is no backtracking anywhere in `pkg/lir`. What is
there — arc-consistency propagation with PBQP value ordering — is competent engineering that the
name invites a reviewer to discount.

**"100%" over a denominator that excludes failures is not a number.** `948/948` counted "codegen
returned no error", `continue`d failing files out of the denominator, and multiplied functions by
abstract machines. Three different values circulate for the same SDCC benchmark (−60/−61/−71%); the
harness actually prints −29%.

## Decisions made

**VIR retired as a backend, kept as an offline oracle** (ADR-0043). Measured: PBQP 37/2 vs VIR 33/6
on C89 asserts; ABAP 28/30 vs **0/30**. The fact that made it safe: PFCCO — the strongest
publishable asset — is `pkg/mir2/contracts.go` on the production path, not the Z3 reimplementation.
Retiring VIR does not touch the paper.

The oracle is worth keeping because the GPU tables are validated by the same heuristic cost
function that produced them. An independent SMT formulation agreeing is evidence; a search agreeing
with itself is not.

**`--vir` no longer the default** — implementing a decision made in April and never wired up. This
also un-broke `--lir`, which was dead because `UseLIR = useLIR && !useVIR` with `useVIR` defaulting
true.

## Related

Reports [116](../reports/2026-08-21-116-Prior-Art-and-Novelty-Audit.md),
[117](../reports/2026-08-21-117-Solver-Architecture-Audit.md),
[118](../reports/2026-08-21-118-GPU-Superoptimizer-Artifacts-Audit.md),
[119](../reports/2026-08-21-119-Language-and-Codegen-Audit.md) ·
[Roadmap](../docs/Codegen_Remediation_Roadmap.md) ·
[Innovation Agenda](../docs/Innovation_Agenda.md) ·
[ADR-0043](../docs/adr/0043-vir-demoted-to-offline-oracle.md)
