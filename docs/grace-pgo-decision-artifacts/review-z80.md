The evidence supports profile-guided selection for the six-call workload: recorded Z80 totals are **48,495 → 31,483 T-states**, and the selected variant matches the forced pointer variant. It does not yet prove complete address-domain correctness or that decisions retaining the original loop are applied correctly.

I made no edits. My test run was blocked by the read-only sandbox when Go tried to create its build directory; the numbers below come from inspected artifacts.

Actionable findings:

1. **P2 — Decisions retaining the original loop are never executed.** [profile_guided_sum_test.go:165](/tmp/minz-grace-pgo-resume/minzc/pkg/pipeline/profile_guided_sum_test.go:165) checks short-workload and size decisions only as strings. An implementation that ignores `"original"` could pass these checks. Apply each decision to fresh HIR, compile it, and execute it in VM and Z80. Assert original costs and size. Also test single-call lengths **2 and 3**, immediately around the model’s crossover: pointer minus original is `120 − 52*n`.

2. **P2 — Address-guard coverage remains incomplete.** The requested test uses only `0x4000`. The existing untracked [profile_cost_domain_test.go:37](/tmp/minz-grace-pgo-resume/minzc/pkg/pipeline/profile_cost_domain_test.go:37) improves coverage substantially: all 256 lengths at `0x4000`, plus 255 fallback cases at `65536−n`. Its saved 511 rows match the fast linear model and show a constant **139-T-state fallback overhead**. However, these fallback inputs end at `0xFFFF`; they never wrap. For every length, add:
   - The last admitted base, `65535−n`.
   - For `n>0`, the first rejected base, `65536−n`.
   - Actual wrapping ranges, such as base `0xFFFF` with `n≥2`.
   - `n=0` at `0xFFFF`, checking zero data reads.

   Execute both forced and profile-selected pointer variants. Trace the emitted branch to establish fast/fallback execution; equal sums alone cannot establish which path ran.

3. **P2 — Memory checks prove final input preservation only.** [profile_z80_test.go:62](/tmp/minz-grace-pgo-resume/minzc/pkg/pipeline/profile_z80_test.go:62) misses out-of-range reads, writes outside supplied input addresses, and writes later restored. Add access monitoring and surrounding canaries. Allow expected instruction fetches, stack traffic, and compiler spill writes; reject input writes and unexpected data reads. Check VM memory in the full-domain test too. Before boundary execution, explicitly verify that input, code, spill storage, and bootstrap stack do not overlap.

4. **P2 — Cost evidence is insufficiently bound to the compilation that produced it.** [profile_policy.go:39](/tmp/minz-grace-pgo-resume/minzc/pkg/hir/profile_policy.go:39) checks HIR, target, function, and a nonempty evidence label, but the model carries no compiler/build identity or codegen options. The same HIR and target can retain stale coefficients after compiler changes. Bind measured models to compiler identity and relevant options, and test rejection of mismatches. Also avoid publishing `OriginalTStates` for unrelated functions: line 64 computes every site’s total using this function-specific model before checking coverage.

5. **P2 — Conservative fallback selection needs direct policy tests.** [profile_policy.go:66](/tmp/minz-grace-pgo-resume/minzc/pkg/hir/profile_policy.go:66) correctly requires complete all-fast coverage before considering pointer selection. Add valid workloads containing only fallback calls and mixed fast/fallback calls. Assert `"original"` in speed and size modes, then apply and execute those decisions. Separately train on fast calls, select pointer-walk, and execute held-out wrapping inputs to verify the retained runtime guard.

The Z80 harness itself uses assembled machine code and CPU stepping. Its timing boundary correctly excludes bootstrap setup and `CALL`, includes the final `RET`, and checks return PC plus restored SP. Disabling ROM protection makes boundary memory writable. The reported **128/177 bytes are whole-module sizes**.

For stronger ABI evidence, assert argument locations and values at function entry and add an ordinary compiled caller. The bootstrap uses production allocation/contracts, so the present tests establish compatibility with that bootstrap, but provide limited independent evidence about normal caller interoperability.