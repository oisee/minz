# Word constant TSMC spill: accumulator preservation

The16-bitTSMC branch of OpConst previously patched immediate bytes using A while preserving onlyHL. A live byte parameter was overwritten with the word constant's high byte. Orphaned reload rewriting could redirect the same stores to ordinary spill data, retaining that clobber. The local repair preserves AF around only this constant-patching branch; existing move paths and centralTSMC emission are unchanged.

Minimal assembled-machine regression covers three constants1/ABCD/FFFF with ordinary patched reload and orphaned spill rewriting. All six fail before and pass after, checking A=103, originalHL, carry, balancedSP and correct16-bit payload. The unchanged-fix overlay was used for historical before execution, rather than manually altered assembly. FullMIR2 passed5.924s. The emitted patch and minimal test are preserved here.

The bug emerged during a **negative** `region_eq` pointer-loop experiment. Full30-call tokenizer host/state/resource-traffic checks pass raw, but hoisting pureguard temporaries initially causes actualZ80 str_bump82 instead of91. Historical sources/ASM/binaries/logs/raw evidence remain in `region-prototype-history/`. After the constant repair the candidate is correct at2,341,065T/4,644B, still slower than original2,270,286T/4,346B. The prototype is not registered as a Grace rule or production selector. Other preserved alternatives, guardPC attribution, failures and limitations must not be presented as a speedup.

`verify_fixed_tokenizer.py` reuses the independent persistent Python lexer: all30 results, complete globals values/resources and every resource read/write count match the corrected candidate's preserved raw/target evidence. It checks candidate binary SHA256. Globaltraffic is compared by VM/target only, not independently counted by host. `independent-fixed-tokenizer-verification.json` captures the result.

`budget-check/` compiles ONLY the current four constant-if/invariant masks using the real two-Grace-pass path, then compares source/exactASM/assembledbinary to scope-stage artifacts. It does not rerun the previous compiler or long workload. PriorOptSize deduplication applies because explicit policies exclude versionedpointer/copy rules; see existing mixed-intern budget-size documentation. Parent owns global artifact manifest and final gates.

Reproduce:

```sh
python3 docs/grace-tsmc-const-artifacts/verify_fixed_tokenizer.py
cd minzc
go test ./pkg/mir2 -run '^TestZ80TSMCWordConstantPreservesAccumulatorAndFlags$' -count=1 -v
go run ../docs/grace-tsmc-const-artifacts/budget-check/compile.go ../docs/grace-tsmc-const-artifacts/budget-check/current
go run ../docs/grace-tsmc-const-artifacts/budget-check/audit.go ../docs/grace-tsmc-const-artifacts/budget-check
```


Built CLI speed/size runs after the localOpConst repair are preserved under `cli/`. Reused Python independent host verifies all30 persistent calls and both candidates per objective, including fullresource snapshots/readwrite counts, source run, decisions and selectedASM/bin identity. TypedGo verifier confirms four complete evidence digests. Original/fulltokenizer production binaries remain identical: speed2259865T4475B SHA9755ac65f59ed3ee83d28316a85de378edb50e381c6d18c2103786927c2a79f8; size2270286T4346B SHA048b95267b15d26a4a71d0a7051a16dd12f5f66c36e9ce1c922b45d109babc33. The repaired bug is exercised by the separate region-hoist prototype, not by these unchanged baselinebinarycandidates.
