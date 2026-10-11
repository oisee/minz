# Authoritative direct-global-word-load CLI evidence

Built parent compiler executable dba34fdc489d4fa00f985bff82952076f8de8f68488999dfe98364af242c9ca5. Four actual CLI runs completed: full original SelfLanz30 calls for speed/size, then two-call collection and same-executable supplied-profile replay. Each run measures four native candidates; all16 saved candidates passed independent source-model state/events and typed native hashes. No old profile was relabeled current.

| Candidate | Full30 T | Bytes |
| --- | ---: | ---: |
| Original | 1135370 | 2000 |
| Arena | 1135211 | 2121 |
| Append | 1079795 | 1978 |
| Both | 1079636 | 2099 |

Speed selects both; size selects append. Small collect/replay saved selections and native outputs match exactly. Profiles retain authoritative executable identity and original source/unit hashes; decision SHA binds exact stored selection JSON bytes. These are current actual measurements, not inferred from the prototype test process or previous compiler.

Large JSON/MIR/ASM payloads use deterministic gzip with zero mtime and exact original hashes in `preserved-payloads.json`. `audit_cli.py`, `verify_host.py` and `verify_typed_payload.go` can verify saved raw/gzip files without CPU execution. General compiler/package/showcase/budget gates are owned by the parent and reported separately.
