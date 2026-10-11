# One bounded region_eq experiment: compare the first byte before guard setup

Read-only plan. No compiler, prototype or parent files changed.

Baseline evidence: full tokenizer original 2,270,286 T / 4,346 B; first pointer candidate 2,435,966 T, region_eq exclusive 390,934 T versus 224,008 T original. That candidate spends 297,260 T on guard/setup across 178 entries, and executes only 285 paired-read iterations. The later hoisted, accumulator-correct candidate still costs 2,341,065 T / 4,644 B. These are negative results, not a selected production optimization.

Independent opportunity count: translate the existing persistent Python host lexer only by adding counters inside its unchanged intern comparison. Exact saved 30-call workload yields 178 region entries: 1 zero length, 177 positive; 125 mismatch at the first byte, 52 equal at first byte, of which 6 have length one. Therefore only 46 entries need a guard or remaining-byte loop after a peeled first comparison. This measures source workload behavior, not hypothetical T-state savings. Counters saved in /tmp/minz-region-first-byte-opportunity.json.

Candidate pseudocode, transformed HIR only, source unchanged:

    var i:u8 = 0
    if len == 0 { return 1 }
    if src[src_start] != arena[arena_ptr + 1] { return 0 }
    i = 1
    if len == 1 { return 1 }
    let limit:u16 = 65535 - u16(len)
    let right_limit:u16 = limit - 1
    if existing_exact_nowrap_guard(src, arena, src_start, arena_ptr, limit, right_limit) {
        left = src + (src_start + 1)
        right = arena + (arena_ptr + 2)
        for generated:u8 in 1..len {
            if load(left) != load(right) { return 0 }
            left = left + 1
            right = right + 1
        }
    } else {
        original_while_body_with_i_now_1
    }
    return 1

The first comparison uses original IndexExpr arithmetic and preserves left read, right read, mismatch return. Do not precompute the right pointer before the left read. Zero length bypasses all memory; len one returns after precisely one pair. This remains faithful for left or right first-load traps, overlapping/same resource values, either logical-offset or physical-pointer wrapping, and early mismatch. When guard fails, the original loop resumes at i=1 and must not reread byte zero. The unchanged conservative guard establishes both logical u16 and physical pointer arithmetic plus final one-past update for fast iterations 1..len-1. Pure arithmetic after a successful first pair is unobservable. No calls/stores or purity inference are introduced.

Only experiment file edits after approval: existing isolated pkg/hir/region_compare_prototype.go and pkg/pipeline/region_compare_prototype_test.go. No Grace registration, production policy/API, source rewrite or parent mutations. Keep all previous negative candidates and accumulator-failure history.

Validation once for this single candidate: original and transformed raw MIR against existing independent exhaustive 6,144 cases (all u8 lengths, 16-byte and 64K heaps, wrap boundaries and aliases), exact ordered successful reads and first-trap address/result plus unchanged full heap; add a focused first-equal/second-trap case for both left and right, since first-mismatch fixtures often exit before the tail. Then unchanged full 30-call tokenizer: original/pre/post raw plus actual Z80, independent host results, full snapshots and all resource access counters, VM/target global-traffic equality, binary binding. Record actual T/bytes and all failures. Full broad compiler/package gates are not needed for an isolated negative-or-positive experiment. A gain is unknown until measurement; the added two short entry tests and duplicated first-address setup can still erase it.

Why this candidate before widened endpoint guards: it eliminates entire expensive setup for 132 of 178 actual calls while reusing existing bounded arithmetic proof. Widened u32 guards introduce new lowering/register costs; merely hoisting u16 limits has already remained negative. This candidate has one clear source-semantics proof and a full real caller already available.
