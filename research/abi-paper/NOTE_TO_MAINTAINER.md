# Note to Maintainer: Strengthening the PFCCO Paper

## What the paper needs to go from "draft" to "submittable"

### 🔴 Critical (Blocks submission)

**1. Fix the peephole regressions in showcase examples.**
The minmax example has a double `EX DE,HL` (lines 263–264 in EXAMPLES.md)
and the GCD example has a redundant `CP C` (line 173).  These are peephole
optimizer misses — trivial to fix, but they undermine the "production
compiler" claim.  Reviewers WILL notice.

**2. Multi-return must parse and compile.**
The `-> (u16, u16)` syntax does not parse in the current `/tmp/minzc` binary
for .nanz files.  The minmax/swap/min_of example (our crown jewel — EQU
collapse, 17× speedup) is hand-traced from optimizer logic, not compiled.
A reviewer running the compiler would not reproduce Example 4.

**3. Run Krause 2022's actual benchmarks through Nanz.**
Krause 2022 evaluated against Whetstone, Dhrystone, Coremark, and stdcbench.
Our examples are hand-picked to show PFCCO's strengths.  A reviewer familiar
with Krause's work will ask: "What happens on Dhrystone?"  We don't need to
beat SDCC on all benchmarks, but we need honest numbers.  Even showing that
PFCCO helps on 60% of functions and is neutral on the rest is a fine result.

### 🟡 Important (Strengthens significantly)

**4. Swap regression: contract optimizer picks BC for first param.**
When compiling swap(a: u16, b: u16), the contract optimizer assigns the first
parameter to ClassPair(BC) instead of ClassPointer(HL).  Root cause:
`inferNaturalClass` sees a Move to the return position and picks the wrong
class.  This adds 2 unnecessary LD instructions.  Fix this before
benchmarking — it's the kind of thing that turns a 17× win into a 20× win.

**5. Formal proof, not "proof sketch."**
The paper currently has a proof *sketch* for DAG optimality.  For CC/LCTES,
we need either a full proof or a reference to a known DP-on-DAG result that
we can cite.  The argument is straightforward (subproblem independence on
DAGs), but it must be rigorous.

**6. Add a "limitations" subsection to §6 (Evaluation).**
- PFCCO does not optimize shadow register classes (hardcoded by type)
- Multi-return return-class optimization is not implemented (known clobber bug)
- Recursive functions get heuristic, not optimal, contracts
- Comparison is against SDCC 4.2.0, not latest trunk (4.4.x has improvements)

Honesty here *strengthens* the paper.  Reviewers respect acknowledged
limitations far more than discovered-by-reviewer limitations.

### 🟢 Nice to have

**7. Measure compile-time overhead.**
PFCCO's O(n) claim is theoretical.  Measure wall-clock compilation time with
and without `--contract-opt` on a medium-sized program (≥50 functions).
Expected result: negligible (<5% compilation time).

**8. Graph visualization of contract propagation.**
A figure showing a call graph with register class annotations on edges —
before and after PFCCO — would make the paper much more intuitive.  Consider
generating this from the actual compiler output (OptimizeContracts can log
before/after state).

**9. Compare with Hi-Tech C (if available).**
Hi-Tech C's Z80 compiler was historically considered the best commercial
Z80 C compiler.  A comparison would add another data point, though finding
a licensed copy may be difficult.

**10. EQU collapse as a separate optimization pass.**
Currently the assembler may or may not collapse `JP target` to `EQU target`.
If this is explicitly implemented as a post-PFCCO peephole (detect when
function entry is a single `JP` to another function with identical contract),
it becomes a reliable, documented optimization rather than an emergent
curiosity.

---

## Priority order for implementation work

1. Fix peephole (double EX, redundant CP) — **30 min, huge ROI**
2. Fix multi-return parsing for .nanz — **needed for reproducibility**
3. Fix swap regression (inferNaturalClass) — **1-2 hours**
4. Port one Krause benchmark to Nanz — **1 day, essential for credibility**
5. Add limitations subsection — **30 min of writing**
6. Formalize the DAG optimality proof — **2-3 hours of writing**
7. Measure compile-time overhead — **1 hour**
8. Call graph visualization — **nice for presentation**
