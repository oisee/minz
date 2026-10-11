Frozen measured arena selector + optional native call-frame codegen budget audit.

Four constant-if/invariant-base masks are compiled using two actual Grace passes
per mask. Only OptSize=false is compiled. Budget-specific deduplication follows
the existing rule allowlist: neither admitted rule depends on OptSize, empty
pointer policy disables pointer rewrites, and copy/region/ordered-byte policies
are absent. No eight-compilation claim is made. Optional native profile/sidecar
features are not enabled on this default budget compilation path.

Fresh assembler audit compares exact source, ASM and machine binaries with prior
closed loop-profile stage docs/grace-memory-host-loop-artifacts/budget-check.
The earlier722.080s machine gate applies only through exact byte equality. No old
compiler or long target workload is repeated. Fresh assembler audit PASS: all source/ASM/BIN bytes match the preceding loop
profile gate. Binary sizes344/344/232/344; compile times0.425/113.780/5.345/122.061s
(total241.611s). Prior722.080s target gate retained through exact byte identity.
compiler-identity.json binds all18 actual integrated compiler files to frozen
checkpoint0bfe99a6 on productionbase4fead12c. commands.txt and helpers preserve
execution paths. For relocated re-audit, run audit.go from checkout/minzc with
explicit current/prior artifact roots. Saved source/ASM may be raw or gzip;
binaries remain exact raw bytes.
