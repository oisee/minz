# Isolated peeled-first-byte region comparison: positive measured candidate

Source tokenizer and actual 30-call workload are unchanged. Only the isolated exact-source manual region_eq transformation changes. No Grace registration, production policy, lineage validator or profile API exists for this prototype. Parent and goto worktrees were untouched.

The first byte uses the original ordered indexed reads; zero count returns without reads; count one returns immediately after its pair. An equal first pair sets source i=1. The unchanged logical/physical nowrap guard applies only to the remaining tail; fast pointer loop starts at1 and uses the existing conservative one-past bounds; fallback resumes the original loop at i=1 without repeating byte0. Aliases, source offset wrapping and first/second traps retain source behavior.

Measured candidate **2,214,152 T / 4,732 B**, original reference **2,270,286 T / 4,346 B**: saves56,134 T (2.47%), adds386 B. All other HIR optimization policies are disabled, including forward copy. The independently measured production copy-only choice is2,259,865 T /4,475 B; this pack does NOT measure a composition of peeling and copy and does NOT report production PGO selection. Earlier unpeeled hoisted pointer candidate2,341,065 T /4,644 B remains preserved in the separate negative pack /tmp/minz-region-compare-evidence, including earlier guard/spill failure history. Exact pre-experiment two prototype files are preserved under before/, current files under after/, and experiment.patch.gz contains only changes relative to those files.

Validation: existing6,144 raw cases (all256 counts, two heaps,12address/alias cases) pass exact ordered successful reads, first trap cause/address, result and full unchanged memory. Two added independent fixtures make the first pair equal then trap on the left/right second load, checking that fallback neither repeats nor skips reads. Exact-source changed-effects/stride/read-order/return refusals pass. Full unchanged30-call host lexer/Pascal interning oracle checks results, complete globals/resource snapshots and all resource read/write counters for original/precompile/postcompile raw and actual Z80. VM/target global access counters agree, but independent host global traffic is not claimed. Standalone verify.py repeats the full resource/state oracle and binary/T bindings without a compiler.

Reproduce in the isolated worktree minzc directory (Go1.26.8, existing prototype source/test plus the separately preserved TSMC word-constant fix and structured lowerer):

```sh
MINZ_REGION_TOKENIZER_ARTIFACTS=/tmp/region-first-byte-repro go test ./pkg/pipeline -run '^TestRegionComparePrototype(Raw|ExactRefusals|SecondReadTraps|WholeTokenizer)$' -count=1 -v
python3 /tmp/minz-region-first-byte-evidence/verify.py
```

The Go command outputs plain JSON/HIR/MIR files; this pack deterministically gzips large files (gzip mtime0, empty filename). Standalone verifier consumes this packed measured/ tree. It does not assert a universal improvement: only this actual corpus was measured, and additional equal-region/mismatch-position coverage, generated-name/function metadata, independent structural pre/post proof and source-site opt-in lineage/policy remain required for production.
