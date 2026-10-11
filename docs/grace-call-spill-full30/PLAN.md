# Fresh production full30 checks after caller-spill certificate repair

Preparatory artifact-only branch based on a02a3cca. No compiler build, native execution or CP/M execution has been performed in this stage. Final executable identity will be captured only after the pending ordinary print.u8 ABI work is reviewed and the production source is frozen.

Inputs are exact prior full30 bytes: unchanged SelfLanz source SHA256 `0e92689c6bb4a92fd681c9bc70d5d50549631a0fdd669cd026f723cf4e197526`; workload SHA256 `ac7d90cb3501e183a7fc9d45242259551e9cd260db53731704ef0a31ee6f933f`, with 30 calls. The logical source argument remains `../examples/nanz/self_lanz_parser.nanz`, executed from the frozen compiler's minzc directory. Local copied source is an inventory record, not a substituted logical unit.

The prior runner invoked the separate search mode with `-b z80 -t cpm --memory-host-arena-append-search DIR --memory-host-workload FILE`; actual Size additionally used `--Osize`. The mode measures both explicit backend policies across masks 0..3, then writes the chosen measured ASM/BIN directly. It does not infer compiler OptSize from the goal or reuse old native evidence.

Prepared runner `run_cli.py` accepts a reviewed executable path and its exact SHA, cwd and a new permanent output root. Without `--execute`, it prints commands only. Before execution it verifies both input hashes, 30-call cardinality and executable SHA, then retains an exact executable copy. Runs are serial with a 1200-second timeout per CLI invocation. Every run retains combined stdout/stderr, exit status, 50 output files, selected external `-o` assembly and exact input associations; failure stops later runs without deleting earlier artifacts.

Two collect-only goals (`--collect-only`) produce 16 candidate envelopes and four additional collected original baselines: 20 native workload executions, 600 native observations. Full collect+paired replay produces four directories: Speed collect/replay and Size collect/replay, with 32 candidate envelopes plus four collection originals, totaling 36 native workload executions and 1080 observations. Replay necessarily runs fresh native candidates under the same executable; it is not a saved-only audit. Root will choose the authorized scope before execution; the preparatory script does not launch either scope now.

Every directory contains eight combinations, not eight policies. Stable ordering is Legacy masks0..3 followed by PreserveForSpeed masks0..3. Actual Speed ranks only measured T; actual Size ranks only measured BIN length. Native result/state/regions/globals/ordered host events, all eleven source profile counters, exact origin/aggregate proof, mandatory policy envelopes, call-spill typed SHA and selected file hashes require independent saved audits. Cross-goal candidate equality is checked only within the fresh executable context. Older full30 costs remain historical, without relabeling their contexts.

Ready invocation shape (placeholders must be replaced by the reviewed frozen executable):

```
python3 run_cli.py --executable FINAL_CLI --executable-sha256 FINAL_SHA --cwd FROZEN_MINZC_DIR --output NEW_PERMANENT_ROOT
```

Add `--execute` only after root releases the actual lane, and `--collect-only` if only the two collection goals are authorized. No source or public compiler API changes are part of this branch.
