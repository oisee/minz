# Final narrow lifetime exclusion follow-up

At 272814d1, the TSMC scan now includes OpCallCond and conservatively OpAsm along with direct and indirect calls. This closes the concrete conditional-call omission identified at cff26. It excludes only candidates live across such barriers; call results and last-use call arguments remain eligible. CFG live-out plus terminator uses cover values consumed in successors. This does not globally disable TSMC.

The caller-home collector now tests actual g.loc(r)==g.spillLabel(r), rather than requiring the static allocation to be LocMem. This includes generated named spill overrides, for example a flag predicate materialized to a home when scratch is unavailable. It still requires current-function regInfo, uses the established native spill width, excludes result definitions and respects relocated values. Original static allocation and dynamic physical locations are therefore treated consistently.

The new tests cover all four barrier opcodes, both local and successor uses, new call-result eligibility and last-use argument eligibility. These are focused scanner controls: some raw indirect/asm test instructions do not provide full executable operands, so they do not attest native indirect/asm behavior. They meaningfully exercise the lifetime filter and complement the separately gated direct recursive native fixtures.

No remaining blocking source issue was found in this bounded home snapshot and lifetime exclusion. Native execution, combined accumulator fallback and actual recursive source compatibility remain separately reviewed/tested lanes. No CPU by this reviewer. Exact source hashes are retained in the adjacent JSON inventory.
