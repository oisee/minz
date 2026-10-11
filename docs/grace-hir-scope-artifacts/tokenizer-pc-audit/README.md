# Read-only physical-function timing audit

Baseline machine binary is unchanged: SHA256 `048b95267b15d26a4a71d0a7051a16dd12f5f66c36e9ce1c922b45d109babc33`; 4,346 bytes; exact original 30-call runtime **2,270,286 T**. The audit reuses the production workload harness, bounds, bootstraps, emulator and oracle, adding only per-instruction start-PC/cycle counters through a temporary Go build overlay. The repository source is never edited.

`audit.go` recompiles the saved typed source with all optional HIR choices off and refuses if assembly differs byte-for-byte from `baseline.a80`. The production harness then assembles that exact text. Replay asserts binary hash, all invocation results/snapshots/full access traffic, globals/resources, bootstrap hashes/costs, executed steps and total timing against saved target evidence. Instrumentation creates a different host executable/context identity, so the old compatibility field is explicitly excluded from runtime-evidence equality. It does not change the tested machine binary.

`pc-records.json.gz` contains call/PC/instruction count/T-states, symbols and all invocation runtime evidence. `attribute.py` resolves every physical function's assembly header against actual symbols and assigns instruction costs to the start-PC address range. Bootstrap is excluded. Generated `scan$split_19` is separate. CALL/RET and tail-call costs are charged where executed, with **exclusive**, not caller-inclusive, totals.

Largest physical functions: intern 524,924 T (23.12%), scan 458,306 (20.19%), peek_char 301,086 (13.26%), region_eq 224,008 (9.87%). These observations guide a next proof proposal; they are not a measured optimization gain.

Reproduce with repository and preserved measured-artifact paths:

```sh
/path/to/pack/reproduce.sh /path/to/minz /path/to/tokenizer-artifacts/measured
python3 /path/to/pack/attribute.py /path/to/pack/reproduced-pc-records.json /path/to/tokenizer-artifacts/measured/baseline.a80 /path/to/pack/reproduced-function-costs.json
```

Use the same Go toolchain as the repository. The saved overlay is a complete copy of production `pkg/pipeline/profile_workload.go` plus audit counters; review its diff against the corresponding source before using it with a later revision. It preserves the harness used for this baseline, rather than silently adapting to unrelated future runtime changes. `reproduce.sh` writes its overlay mapping and fresh outputs only inside this evidence pack; baseline resources remain untouched. `MINZ_PC_BASELINE_DIR` makes the Go helper's baseline location adjustable. Initial measurement logs are retained in `replay.log`.

`manifest.json` binds every preserved file except itself. Raw PC JSON uses deterministic gzip. No parent documents, compiler files or project manifest were changed to create this pack.
