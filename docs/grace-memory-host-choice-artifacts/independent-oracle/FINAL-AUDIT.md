# Independent boundary choice audit

All 24 saved raw/native candidate records and six fresh CLI source profiles pass
the independent original SelfLanz memory model. The source is frozen to SHA256
`0e92689c6bb4a92fd681c9bc70d5d50549631a0fdd669cd026f723cf4e197526`;
the executable context is the existing `1bd95516...` binary. No compiler or CPU
execution was repeated for this audit.

| Workload | Original / arena / append / both, T-states | Speed selection | Size selection |
|---|---|---|---|
| zero body | 784 / 763 / 948 / 927 | arena, 763 T | append, 2076 B |
| first equal | 1672 / 1587 / 1836 / 1751 | arena, 1587 T | append, 2076 B |
| first mismatch | 1608 / 1564 / 1772 / 1728 | arena, 1564 T | append, 2076 B |

The four binaries are 2091 / 2212 / 2076 / 2197 bytes. These short workloads select
arena alone for speed; the earlier four parser corpora selected both transforms
for speed. The measurements establish workload-dependent choices, with no
universal winner inferred.

Both source sites have positive Entries in the three successful fixtures;
append BodyEntries is zero. The admission rule correctly accepts an observed
zero-body loop. The repeated-hit fixture has no append Entries and refuses
selection. Source-tail and destination-tail denials preserve their exact reads
and earlier stores; readonly-header denial happens before any host event.
Those failed profiles remain diagnostics. The one-step gas failure has no typed
memory failure and refuses selection as infrastructure failure. All five CLI
refusals exit 1 with absent output directories; their logs and raw profiles are
preserved in the owner evidence pack.

Checks cover every invocation's results, all ten globals, full declared resource
snapshots, ordered host events, and independent per-call/cumulative 11-site
counts. They also bind typed precompile proof fields to actual baseline
compatibilities/function maps and the two source/actual loop origins and paths;
verify compact sidecar, call-frame, allocation and MIR hashes; compare embedded
and saved assembly/binary; and recompute actual goal selection and the exact
saved selection-file hash in decisions.json. This artifact check does not
re-prove MIR-to-instruction semantics or claim a post-compile HIR certificate.
The compact JSON hash audit uses the current payload order and ASCII field names;
the owner's Go typed verifier provides the schema-level serialization check.

Reproduce without native execution from this branch root:

```sh
PYTHONDONTWRITEBYTECODE=1 python3 docs/grace-memory-host-choice-oracle/audit_raw_profiles.py "$EVIDENCE/raw-profile" "$EVIDENCE/source.nanz" raw-audit.json
PYTHONDONTWRITEBYTECODE=1 python3 docs/grace-memory-host-choice-oracle/audit_cli_pack.py "$EVIDENCE" --source "$EVIDENCE/source.nanz" --out final-cli-audit.json
```

`EVIDENCE` is the saved `memory-host-choice-boundaries-evidence` directory. The
helpers accept lossless gzip payloads where the owner pack compresses JSON,
assembly, MIR or binary artifacts.
