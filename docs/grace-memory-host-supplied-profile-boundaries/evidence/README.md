# Supplied profile admission

Five actual invocations use the unchanged original SelfLanz source and the unchanged authoritative CLI executable 1bd95516. The healthy supplied current profile was accepted and freshly replayed: arena mask1, 763 T-states, 2212 bytes. All four candidate payloads pass independent original-source state/event and typed native-byte/hash audits.

| Supplied profile | Nature | Actual first refusal |
| --- | --- | --- |
| Current 1bd | Authentic healthy profile | Accepted |
| Prior 852cf | Authentic healthy profile from another executable | incompatible family/source/workload |
| Failed sample | Synthetic failure claim, completion tallies 1/1 and second invocation false/Error | incomplete or failed sample |
| Failed invocation | Synthetic inconsistent invocation false/Error with healthy aggregate completion | failed invocation |
| Cumulative mismatch | Synthetic count sum forgery | cumulative count mismatch |

Old/current authentic profile data become exactly equal after replacing only the two compiler identity fields. The supplied workload and original source path are identical. This isolates the stale executable gate. The other profiles preserve current source/workload/context identity and change only the declared synthetic fields. They are not authentic failed executions; the CLI has no failed-profile exporter. No profile identity is overridden and no production files changed.

Each rejected invocation exited 1 and created no output directory, native binary, selection or decision. Commands, input hashes, exact cause logs and output absence are preserved. Accepted profile bytes were compared against the supplied current profile; native output bytes and decision hash match the selected candidate. Existing node_tag label-audit warnings are preserved verbatim and are not new compiler regressions.

Twenty-six large payloads use deterministic gzip with mtime zero, preserving original byte digests in `preserved-payloads.json`. To rerun saved-file checks from repository root:

```sh
python3 docs/grace-memory-host-supplied-profile-boundaries/evidence/audit_inputs.py docs/grace-memory-host-supplied-profile-boundaries/evidence/inputs
python3 docs/grace-memory-host-supplied-profile-boundaries/evidence/audit_results.py docs/grace-memory-host-supplied-profile-boundaries/evidence
```

`verify_host.py` and `verify_typed_payload.go` audit each accepted candidate without running the CPU again. Native assembly remains a bounded workload artifact with closed adapters; no unrestricted extern execution claim is made.
