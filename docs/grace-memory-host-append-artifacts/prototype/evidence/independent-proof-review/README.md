# Isolated append-copy precompile proof

Base: frozen selector integration 0bfe99a6. Original self_lanz_parser.nanz is unchanged.

Two u16 declarations after original header poke hoist src_buf+start, then
str_bump+1. Original u8 j starts at zero. Loop condition/increment, search,
header, final bump and return remain exact source. Only body address expressions
become dst_base+j and src_base+j. Every u16 addition wraps modulo 65536.

The closed byte hosts cannot mutate globals. This precondition is established
for both source and actual raw VM module allocations: no declared resource may
start below the VM reserved storage extent. PrepareMemoryHostWorkload alone did
not enforce this; independent review found and fixed that gap before freeze.
Native resource/global/spill/stack exclusion remains a separate native gate.

The promise is exact ordered host byte events, partial stores/first typed denial
and final state. Header store precedes all reads. Alias propagation is sequential,
not a memmove snapshot. Internal compiler global-load counts and gas differ.
No arbitrary callback purity or postcompile HIR proof is claimed. Standalone
validator inputs must be independent immutable snapshots; higher public factory
ownership remains required for any future measured API.

All module metadata and unrelated functions remain exact. Origins are bound to
logical function/path; original roots s4..s7 move to s6..s9. New declarations have
no invented origin. Detached/shared statements, prior policies and tampering
are refused. Original p/j global collisions and reserved-name collisions are
also refused.

From minzc:

```
go test ./pkg/hir -run '^TestMemoryHostAppendCopyIndependentProof$' -count=1
python3 docs/grace-memory-host-append-copy-proof/append_oracle.py --selftest
```

15 structural cases pass. Python independently checks 1028 append cases: all
256 lengths at four alias shifts, three typed-denial prefixes and modular wrap.
The oracle's RAM-zero wrap success is byte semantics only; actual closed VM
layouts forbid module-storage overlap. Target/raw domain tests must retain this
restriction and compare denial prefixes rather than invent zero-address success.

Separate owner's preliminary actual original30 results: 1221494 T / 2091 B
original, 1150124 T / 2076 B hoisted. This is not a production profile/selector or
loop-lineage certificate. No default optimization is enabled.
