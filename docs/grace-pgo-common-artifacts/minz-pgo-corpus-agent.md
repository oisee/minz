Added only [profile_corpus_test.go](/tmp/minz-grace-pgo-resume/minzc/pkg/pipeline/profile_corpus_test.go). No commit.

Passing coverage: 193 literal calls from full, unmodified sources:

- `06_pbqp_weighted.nanz`: `weighted`, including overflow.
- `grace_hir_constant_if.nanz`: `wrap`, `choose`.
- `sha256.nanz`: `shr32_lo/hi`, `add32_lo/hi`, `xor16`, `and16`, `not16`, `ch_lo`, `maj_lo`.

Each uses independent oracles, production profiling/measurement/Z80 execution, and limits of 1,000,000 steps and 65,536 bytes. Successful measurements assert one baseline candidate; no PGO improvement or complete SHA behavior is claimed.

`04_lut_popcount.nanz` inputs 0/1/127/128/255 pass raw checks but trigger `compiled parameter type mismatch`. A separate test explicitly reproduces that rejection.

Related tests passed with the specified Go toolchain/cache. Artifacts: `/tmp/minz-profile-corpus-final/real-source-corpus`.

Reproduce from `minzc`:

```bash
GOCACHE=/tmp/minz-grace-gocache MINZ_PGO_ARTIFACT_DIR=/tmp/minz-profile-corpus-final /home/alice/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.8.linux-amd64/bin/go test ./pkg/pipeline -run TestProfileRealSourceCorpus -count=1 -v
```