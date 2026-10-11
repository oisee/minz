# Guarded region_eq pointer prototype: negative profitability result

Isolated worktree `/tmp/minz-region-compare-prototype` starts at568269a8 and copies current stable parent `pkg/hir/lower.go` to enable structured lexical scopes. Parent compiler/project files were not edited. Only two new prototype files are in `prototype.patch.gz`; the copied parent lowerer is deliberately not in that patch.

`ApplyRegionComparePrototype` manually accepts one exact original region_eq signature/body/binding set, immutable non-SMC parameters, u8 count/index, two ordered reads, early mismatch return and no calls/stores. It is not registered in Grace and has no production PGO/proof API. It preserves the original while as fallback and uses source PtrIter ForEach plus a carried right pointer for fast reads. Guards conservatively require logical u16 offset ends and both pointer ends to stay below65536, including the final one-past bump. Arithmetic setup is pure; zero-trip does not read either memory even when fallback is selected.

Independent raw cases cover all256 lengths, two heaps and twelve address/offset/alias scenarios (**6144** cases). Every variant is checked for exact ordered successful read address/value events, first load trap cause/address, return value and unchanged whole memory. Unknown effects, extra reads, reversed reads, changed stride and return shape refuse. This is preliminary prototype coverage; longer equal regions and exhaustive mismatch positions, production structural postproof/lineage, and broad reserved-name metadata checks need further work before any rollout.

Full unchanged typed tokenizer source/corpus is reused with the independent persistent host resource/state oracle, all30 calls, source/global snapshots and exactVM/actualZ80 traffic. Transformed raw and normal pipeline assembled target agree. No scanner or interning source rewrites.

**Negative result:** pointer compare whole caller **2,435,966 T /4,723 bytes** vs saved original **2,270,286 T /4,346 bytes**. It adds165,680 T (**7.30%**) and377 bytes. Guards and five-parameter/register pressure outweigh address savings for this workload; no optimization gain or production readiness is claimed. Candidate machine SHA256 is in `cost.json.gz`. The parent has the authoritative original artifact pack; this directory preserves the failing profitability candidate rather than selecting it.

Reproduce isolated from its `minzc` directory:

```
MINZ_REGION_TOKENIZER_ARTIFACTS=/tmp/region-reproduction go test ./pkg/pipeline -run '^TestRegionComparePrototypeWholeTokenizer$' -count=1 -v
go test ./pkg/pipeline -run '^TestRegionComparePrototype(Raw|ExactRefusals)$' -count=1 -v
```

The kernel is structurally eligible on the actual source but has no independently exported production validator or source-site policy. Existing copy/sum APIs do not support it. A next investigation should attribute candidate PC costs or seek a smaller guarded shape; it must preserve read order, traps and zero-trip behavior and retain this negative history.


## Follow-up experiments and concrete backend repair

Exact physical-PC replay of the first negative candidate matches2,435,966T and its binary. `region_eq` alone costs390,934T vs224,008T original; guards account297,260T and fast loop93,674T, with no fallback taken. The unchanged caller's other physical costs differ only1,246T in interning due to callee clobber/allocation. Average compare executes1.6 iterations; repeated guard setup dominates.

A ForRange pointer-carrier alternative retains identical guards/read order/traps: raw6144 and full30 caller pass, but2,401,051T/4,702B remain slower. Hoisting the pure limit once (and then both limit temporaries) exposes actual Z80 wrong code: first ordinary source call arena bump82 instead of91. Raw oracle remains correct; failures and emitted binaries/assembly are preserved separately.

The failure is a real OpConst backend bug:16-bit TSMC constant-spill emission uses A to patch two bytes while preserving onlyHL. A live parameter is overwritten by the constant's high byte. Orphaned TSMC reload rewriting redirects patch stores to normal memory without removing the scratch-A clobber. Minimal six machine cases (three constants×normal/orphan reload) fail before the repair and pass afterward, checking payload, A, HL, carry and SP.

`minz-tsmc-word-const-fix.patch.gz` repairs only the16-bitTSMC branch of OpConst with PUSH/POP AF; it does not change centralTSMC moves or add duplicated saves elsewhere. FullMIR2 passes5.924s. Corrected hoisted region prototype passes6144raw+30actualcaller but still costs2,341,065T/4,644B (**3.12% slower** than original), so no region rule is proposed for production. Backend bug repair is separable from the still-negative optimization experiment. `opconst-fixed/` retains its complete candidate machine evidence; `one-hoisted-failure/` and `two-hoisted-failure/` retain prior failures.
