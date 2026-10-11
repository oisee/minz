# Final supplied-profile audit

Five saved CLI runs pass the independent artifact audit. The real current
`1bd95516...` two-call profile was accepted unchanged, with all four actual
candidates independently matching the original source's full globals/resources,
ordered events and results. All eleven source-site counts match the independent
model; the selected actual speed minimum and saved binary/assembly/decision hash
match. Typed precompile source/actual maps and compatibility keys, sidecar,
call-frame, allocation and MIR diagnostics are bound to their candidate records.

The authentic old `852cf9a5...` profile differs from the current profile only in
Context.CompilerID and Compatibility.Context.CompilerID. Its own workload and
all other fields match. It refuses at incompatible family/source/workload,
proving executable-context admission; it does not test a failed-sample gate.

Three explicitly synthetic payloads retain current context/source/workload:
consistent failed completion tallies refuse at incomplete or failed sample;
individual failure with healthy aggregate tallies refuses at failed invocation;
a single changed cumulative entry count refuses at cumulative count mismatch.
They are serialized failure/count claims, not authentic failed execution records.
Every refusal exits 1 and leaves no output directory or selected binary. Original
inputs and diagnostic logs remain preserved in the owner's evidence pack.

No source, loader, CLI, permission or failed-profile exporter changed. No CPU or
compiler execution was repeated by this reviewer. The audit does not establish
MIR-to-instruction correctness anew or claim a post-compile HIR certificate.
Compact payload hashes use the current ordered JSON representation; the owner's
Go verifier checks serialization through the exact typed schema.

Reproduce from this branch without execution:

```sh
PYTHONDONTWRITEBYTECODE=1 python3 docs/grace-memory-host-supplied-profile-review/audit_supplied.py "$EVIDENCE" final-audit.json
```

`EVIDENCE` is the saved `memory-host-supplied-profile-boundaries-evidence` pack;
the helper accepts lossless gzip artifact payloads.
