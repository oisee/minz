# Source-bound ordered-byte function measurements

`api/` retains all two candidates for each speed/size objective. `cli/` retains
actual native executable invocations on the unchanged full hashmap source and
36-call persistent workload. Both objectives select the rewritten exact `rd16`
helper: 126680 T / 980 B, versus original133994 T / 984 B. Every binary is measured
on actual assembled Z80; selected SHA256 is
`31e686b05d328346cd092ee65a310c111de010c53c775d3c3cd8b92ccd016f11`.

Function identities use separate `minz.function-sites/v1`, including complete
source-unit fingerprints, function ABI/body and exact function/path source-origin
hash. The evidence schema is `minz.ordered-byte-function-variant-sets/v1`, and CLI
decisions use `minz.ordered-byte-function-decisions/v1`. Every legal helper is a
choice dimension, including unobserved helpers: at most four functions and 16
whole-workload variants. No helper invocation counts are inferred from loop
observations. Equal primary costs prefer fewer enabled rewrites, then stable
ordering, so an unobserved equal-cost helper stays original.

Portable independent Python host audits check every call's result, complete82B
global state and each global byte's read/write count on candidate raw,
postcompile raw and actual target executions. Separate typed Go decoders
recompute evidence JSON SHA256 and each candidate binary SHA256. These are saved
evidence audits, not redundant compiler/emulator runs. Actual native commands
and executable SHA are recorded in `cli/commands.txt` and
`cli/native-compiler-identity.json`. The final native binary was built with
`-buildvcs=false`, explicitly recorded rather than claiming VCS stamping.

From repository root:

```sh
python3 docs/grace-ordered-byte-function-artifacts/verify_api_host.py
python3 docs/grace-ordered-byte-function-artifacts/cli/verify_host.py
```

From compiler `minzc`:

```sh
go run ../docs/grace-ordered-byte-function-artifacts/verify_api_evidence.go
go run ../docs/grace-ordered-byte-function-artifacts/cli/verify_evidence.go
```

Each completed audit result is retained beside its script. Exact JSON/HIR/MIR/ASM
bytes use gzip with zero mtime; selected binaries are retained directly. Earlier
CLI evidence before function-origin identity binding is preserved only under
`preliminary-history/pre-origin-identity/`. It is not final source-bound proof.
Independent prototype and production boundary reviews, plans, HIR logs and
parent source inventory are retained in separately owned subdirectories/files.

Focused pipeline checks cover16variants/fifthlegalcap, unobserved ties, allprior
policies/sharedfactory state, both fresh and retained source-origin mutations,
legacy API rejection and equal-cost different-popcount selection. CLI validation
requires joint mode, auto workload and exclusive ordered-byte mode. Actual
production boundary review verifies ordered low/high reads and no extra byte
reads on its72cases; raw differential domains cover partial heap traps. Declared
traffic totals alone do not establish all possible native trap ordering.

Parent full pipeline passed 280.522s. HIR/MIR2/C89 passed20.062/5.608/5.909s; Nanz excluding separately compared showcase passed1.570s; CLI20.009s. Showcase31same0new, aggregate package log explicitly retains baseline showcase failure. Fresh budget4compile250.748s +independentassembler audit confirms exact source/ASM/BIN identity and retained earlier722.080s machine gate. See parent-*.txt, showcase-comparison.json and budget-check/.

Historical nested manifests name original files before packaging; root manifest binds final packaged paths. For an archived `.gz` diagnostic or patch, decompress with gzip to recover exact original bytes before reproducing older commands.
