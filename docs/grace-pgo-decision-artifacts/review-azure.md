Reviewed committed `HEAD` **2b867fff**, excluding concurrent uncommitted changes. I found six concrete defects. Reproductions below are derived from source and saved assembly; I couldn’t execute new tests because Go’s temporary build directory is blocked by the read-only filesystem. I made no edits.

1. **P1 — Ordinary source calls produce profiles that the loader rejects.**  
   [profile.go](/tmp/minz-grace-pgo-resume/minzc/pkg/hir/profile.go:228) updates guard observations only for the function passed directly to `LoopProfiler.Call`. Instrumented loop counts also include internal calls. [profile_compat.go](/tmp/minz-grace-pgo-resume/minzc/pkg/hir/profile_compat.go:304) then requires both totals to match.

   **Reproduce:** retain the exact sum idiom, add `fun main()->u8 { return sum_array(&arr, 5) }`, and profile only `main`. The sum loop records `Entries=1, BodyEntries=5`, while its guard row remains zero. Collection succeeds; loading returns `incomplete or inconsistent guard observations`. Profiling an actual program entry point therefore fails before selection.

2. **P1 — Saved decisions bypass profile compatibility when applied.**  
   Committed [profile_policy.go](/tmp/minz-grace-pgo-resume/minzc/pkg/hir/profile_policy.go:91) checks site IDs and variants but ignores `d.Model`, including its HIR hash and target. Site identity excludes the source hash and semantic HIR.

   **Reproduce:** select pointer-walk for the existing workload, then change a dependency or change code after the loop without moving its source offset. Parse again under the same unit name and apply the old decisions directly. The site ID still matches, so application accepts evidence that `LoadLoopProfile` would reject. This can silently reuse profitability decisions across changed programs or builds.

3. **P1 — Cost evidence is not bound to the assembled configuration.**  
   Committed `LoopCostModel` contains no compiler identity, pipeline options, assembler mode, or measured-domain restrictions. Selection checks only target string, HIR hash, function, and nonempty evidence ID. The test measures default assembler mode with `Options{AssertMode:"none"}`.

   **Concrete reproduction:** assemble the saved programs with `OptSpeed`. Their `JRS` loop backedges become `JP`, reducing each iteration by 2 T-states; the taken fast guard jump also saves 2. For six zero-length calls plus one length-16 call:

   | Configuration | Original | Pointer |
   |---|---:|---:|
   | Committed model | 2739 | 2747 |
   | `OptSpeed`, calculated from saved assembly | 2707 | 2701 |

   The accepted model chooses **original**, although the assembled configuration favors **pointer-walk**. Rebuilding the compiler and collecting a fresh profile also leaves old cost evidence acceptable when the HIR and target string remain equal.

4. **P1 — Pipeline Grace can override an explicit “original” decision.**  
   `ApplyProfileDecisions` supplies a temporary selection map. It does not preserve that policy on the module. [grace.go](/tmp/minz-grace-pgo-resume/minzc/pkg/hir/grace.go:122) permits all eligible pointer rewrites when the map is nil; both pipeline entry points invoke Grace this way.

   **Reproduce:** select the short workload `{0,1}`, apply its `original` decision, then compile with `UseHIRGrace:true`. Grace versions the loop anyway. The recorded costs change from **279 to 467 T-states**, directly defeating the profile decision. The committed test avoids this path by compiling with HIR Grace disabled.

5. **P1 — Foreach lowering silently truncates accepted lengths to eight bits.**  
   HIR verification accepts a `ForEachStmt` with a `u16` length, but [lower.go](/tmp/minz-grace-pgo-resume/minzc/pkg/hir/lower.go:1532) always creates a `TyU8` counter and performs eight-bit decrements.

   **Reproduce:**
   ```nanz
   fun count(ptr:^u8,n:u16)->u16 {
       var s:u16=0
       for x:u8 in ptr[0..n] { s=s+1 }
       return s
   }
   ```
   With 256 readable bytes and `n=256`, the first loop edge converts the count to zero: the function returns **0 instead of 256**. Profiling records zero body entries and can load those observations successfully. This is an existing lowering defect exposed by the new profiling path; the generated sum fast path itself uses a valid `u8` length.

6. **P2 — Callback inlining loses loop lineage during lowering.**  
   [lower.go](/tmp/minz-grace-pgo-resume/minzc/pkg/hir/lower.go:2836) clones callback statements through `renameStmt`. Its `WhileStmt` case creates a new node without transferring lineage, while attribution lookup uses statement identity.

   **Reproduce:** seed lineage for a callback containing `while x>0 {x=x-1}`, then invoke that callback through `ptr.forEach(callback,n)`. The callback’s original loop has a site ID, but its inlined loop in the caller has no `LoopOrigins` entry. Instrumented profile hooks can still count its execution, leaving observations without corresponding raw-MIR attribution.

The committed artifacts do demonstrate assembled execution of the selected sum function at six lengths. They do not yet establish a reusable source-program PGO workflow: internal-call coverage fails, decision application can skip compatibility, later passes can reverse selection, and cost evidence can describe a different binary configuration.