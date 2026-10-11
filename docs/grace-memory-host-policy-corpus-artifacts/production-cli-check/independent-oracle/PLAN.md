# Explicit policy-aware closed CLI admission

Read-only proposal against authoritative `0dceafabf7fcaf3160c69899e66c08cb84a80dbf`. This checkpoint changes documentation only. No native run, default policy change, CSE, new host permission, or parent edit is part of this plan.

## Smallest public slice

Add a separate opt-in mode, `--memory-host-arena-append-policy DIR`, with required `--memory-host-compare-policy legacy|preserve-speed`, existing `--memory-host-workload FILE`, and optional `--memory-host-association FILE`. Accept exactly the two named policy values; absent, empty, numeric, and unknown values are errors. Resolve a value-owned `MemoryHostBaselineOptions` before parsing/factory execution. A policy flag or association flag without the new mode is an error.

The existing `--memory-host-arena-append DIR` and `--memory-host-profile FILE` remain on their unchanged Legacy/v1 path. Reject mixing either old flag with the new mode. Even an explicitly requested Legacy policy in the new mode uses mandatory v2 envelopes; it does not silently fall back to the old loader. Reject bare source-profile v1, association v1, and baseline-only payloads instead of detecting/converting their schema. No generic compare flag for ordinary compilation is introduced.

`-Osize` controls the selector goal only. Compile all four variants with the same explicitly requested backend policy. Do not translate size into Legacy, set pipeline OptimizeForSize, or add policy as a fifth choice bit. The existing finite original/arena/append/both masks and exact source proof remain unchanged.

## Inputs, collection, and replay

Retain the old CLI's single cached Nanz source byte buffer (8 MiB), exact logical filename, CPM target, complete one-unit SHA provenance, no imports, strict workload JSON (4 MiB), and unsupported-option exclusions. Cache workload and association bytes before callbacks. Association input uses `pipeline.MaxMemoryHostPolicyAssociationBytes` (32 MiB), not the old 8 MiB bare-profile cap. Check size before copying/decoding/factory execution; use the strict association loader rather than generic json.Unmarshal or prefix-based schema probing.

With no supplied association, invoke `RunZ80MemoryHostLoopProfileWithOptions(factory, workload, options)` and serialize the whole returned v2 envelope. Feed these bytes to `MeasureMemoryHostArenaAppendWithOptions`. With a supplied association, feed its unchanged bytes directly to that selector. Its preflight/load/full fresh association replay performs admission; do not add a redundant exported Replay call or bypass it after collection. A collect-only invocation is outside this minimal slice.

The HIR source profile keeps its existing schema/context and counts. Policy authority comes from the mandatory outer compile_options, nested baseline-v2 policy, and fresh whole-envelope replay. Source SHA, workload, complete function/path origins, source sites, compiler executable ID, baseline options, and current native diagnostics must match. A profile from a different executable or requested policy is refused. Internally consistent invented counters/native hashes are not an execution certificate and must fail fresh replay. No inherited post-transform source counts are exported.

Collection before selection adds its own existing producer lifecycle; do not relabel the selector's exact 24 factory callbacks as the complete CLI lifecycle. Keep the selector's graph-retention and fresh/source identity checks intact. Native infrastructure/gas/admission errors return no selected evidence; typed partial source diagnostics are not silently admitted as complete profiles.

## Output identity and filesystem boundary

Use a separate explicit inventory: `source.nanz`, `workload.json`, `association-v2.json`, `selection-v2.json`, `decisions-v2.json`, `selected.a80`, `selected.bin`; each original/arena/append/both directory contains `baseline-v2.json`, `target.a80`, `target.bin`, `final.mir2`, and `allocation.json`. Do not label a bare profile file as the replayable association. The association stored on disk must be the exact submitted/collected bytes, including legitimate pretty whitespace; loader allocation RawMessage compaction follows validation and must not rewrite the preserved input.

Decision schema proposal: `minz.memory-host-arena-append-cli-decision/v2`, with goal, compile_options, selected mask, typed choices, SHA256 of exact stored selection bytes, SHA256 of exact stored association bytes, selected binary SHA256, measured T states, and byte count. Store mandatory whole selection-v2 and candidate baseline-v2 envelopes, never just nested v1 containers. Policy0 must be present rather than omitted. Select assembly/binary directly from the selected candidate's owned Target record; do not recompile them under implicit/default options. Optional `-o` receives those same assembly bytes.

Extend the complete path inventory before any execution: source, workload, supplied association, all prospective artifact files, and optional output. Reject resolved pathname/symlink-parent aliases and existing hard links via SameFile. Recheck immediately before writes. New filenames must be included in that check rather than reusing the old v1 inventory unmodified. Static admission failures create no output directory. Runtime failures preserve existing diagnostics outside an attested selection; do not manufacture an incomplete decisions file.

## Required focused gates before native measurements

1. Old no-flag and old-mode controls preserve existing flags, schemas, Legacy output and global flag restoration. New-mode tests restore every added Cobra/global flag, including explicit policy presence.
2. Named0/1 resolution; absent/empty/unknown policy, orphan flags, old/new mixed flags, incompatible target/backend/import/options, and size-policy independence. Malformed/capped/schema/policy input refuses before source factory where the API contract requires it.
3. Strict duplicate/case/missing-zero-policy/null/trailing/unknown-field inputs; v1/bare/failed/gas-incomplete payload refusal; requested/nested policy mismatch; actual stale executable profile refusal. Synthetic counter/native forgeries are labeled synthetic and fresh replay refuses them.
4. Source/workload/association/output aliases, symlink-parent and hard-link cases; selected output exactly matches candidate Target bytes, decision digests exactly match stored files, mandatory v2 policy survives pretty serialization.
5. Small real allocated u16 control with both source sites observed: collect and same-executable supplied replay for each named policy; real size selection under fixed policy. Do not treat a u8-only control as compare activation evidence.

After frozen source and independent review, bound full native work to one fixed workload per requested policy initially, with supplied replay controls chosen deliberately rather than duplicating all four goals automatically. Re-audit new executable identities, all four candidate native diagnostics, source-profile eleven-site per-call/cumulative counts, full persistent resources/ten globals/ordered host events, proof compatibility/function/path maps, decision/association hashes and selected ASM/BIN. Whole-workload profits are measured anew; prior −2921 T/+1 B applies only to the previous executable/workload evidence, not an assumed cost model. No PC or target loop-lineage claim follows from function/source-profile evidence.

## Implementation ownership boundary

Next implementation can be restricted to cmd/minzc registration/early dispatch, a separate policy-mode file, and meaningful CLI tests. Existing pipeline v2 association/selector APIs suffice. Sharing the old path/strict-workload helpers is reasonable; avoid a generic callback engine, edits to old v1 admission, or relaxing native certificates to make the new mode pass.
