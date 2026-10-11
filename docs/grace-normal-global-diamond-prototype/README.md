# Isolated normal global diamond prototype

Base6f0403a0. One new MIR helper and one new MIR test file; no default pipeline,
Grace registration, HIR rewrite, PC hook or native execution. API
ReuseNormalGlobalDiamondLoads(module) returns changed loads or an error. Inputs
must be structurally verified SSA. Acyclic entry conditional/single-predecessor
successor only, no successor parameters. All normal u16 globals have unique
non-function names, At nil, plain normal storage, at most2 initializer bytes,
no string/vtable representation.

The helper replaces a repeated direct-symbol u16 OpLoad with OpMove, retaining
its Dst/type/class/hard-class/source location. It does not copy memory-only
attributes. Whole SSA use remapping is unnecessary. Transactional post-Verify
restores original instructions on failure. Remaining native origins must later
be rebuilt from transformed MIR; none are supplied by this helper.

The first bounded interval whitelist is deliberately smaller than a generic
arithmetic CSE: canonical typed integer const/move/comparison and exact normal
u16 AddrOf/load only. No malformed additional operands/symbols/call attributes;
operand types and comparison condition are checked. Any store/call/asm/unknown
read/alloca/conversion/arithmetic outside this first whitelist refuses reuse.
Calls after the second read remain allowed and in place. Both predecessor
suffix and successor prefix must satisfy these checks. Whole function cycles
are conservatively excluded. Internal normal-global read counts/VM steps may
change; host resource effects and state must remain equal.

Focused raw MIR tests pass: safe and late-callback both branch outcomes at
limits0/5/6/65535, preserved sameDst/source metadata; mutation callbacks/stores
retain the second value9; call/store/asm/resource-read/opaque/mixedwidth/merge/
cycle/At/baked/signed refusals; nil/duplicate/shared/typednil input errors;
function-global collision, oversized/string/vtable global and malformed pure
instruction metadata refusals. Full tests completed0.008s, no native CPU.

Command from minzc:
GOCACHE="$CACHE" "$GO" test ./pkg/mir2 -run '^TestNormalGlobalDiamondPrototype' -v

Proof/source independent review requested before actual source caller native
measurement. The helper is experimental only; no profitability or broad CSE
claim is made.

Independent review caught applicability guard excluding ordinary unsigned CmpUge.
Canonical comparisons now allow Eq/Ne/signed and unsigned relational conditions
only (0..9), while synthetic carry predicates10/11 still refuse. An actual
CmpUge cur-shaped branch with reversed then/else passes bounds0/5/6/65535.
