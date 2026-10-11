# Explicit preserved-pair comparison integration review

Read-only plan based66038053, backend draft13739381. This stage contains no code edits or CPU runs. MIR CSE, diagnostic globals and private hooks must not enter production.

## Typed opt-in and default behavior

Use mir2.UnsignedPairComparePolicy with Legacy=0 and PreserveForSpeed=1. Zero-valued existing callers remain legacy, including speed and OptSize. Never resolve PreserveForSpeed from !OptSize, Goal=speed or HIR flags. A public explicit option may select it only for Z80. Reject PreserveForSpeed together with OptSize, an unsupported backend or unknown policy at public admission, before source/factory execution. Low-level codegen currently falls back on an unknown enum; if preserved for compatibility it must not be represented as enabled. Prefer an entry error for new public typed options.

## Pipeline ownership and replay

pipeline.Options carries the requested enum. Resolve once at compilation entry, validate it and store the actual enum in Steps. Normal Z80Codegen uses that resolved Steps value. The native memory-host runner passes the same Steps field into its annotated re-codegen; it must not reread global CLI state or infer a policy from optimization goal. The existing plain-versus-annotated binary equality remains mandatory. Secondary CompileHIRWithOptions codegen and any assert path must also use the same explicit enum or refuse unsupported opt-in.

Closed baseline retains its current two-argument function as a Legacy wrapper. A separate WithOptions entry owns and validates immutable compile options before invoking a user source factory. Compile options contain only supported codegen choices; no user ASM, oracle, callbacks or arbitrary HIR/MIR pass may be supplied. Evidence records the resolved policy with a typed explicit field. Existing source/raw identity remains original; source semantic compatibility must not be rewritten to imply that a backend choice changed source.

If the bounded measured selector/CLI supports this option in the same slice, pass the owned policy through original baseline association and each candidate baseline. Every candidate must have the same resolved policy. A new policy-aware selection/baseline evidence or versioned schema is preferable to silently extending already-published v1 contracts. A source profile is collected before backend codegen and may be semantically identical across policies, but admission context/selection options must make policy identity explicit; any intended reuse requires exact replay and documented scope. Never accept an old selected-cost record under a different policy. Do not append a misleading bit to source compatibility while leaving existing input profile loaders unaware.

## Minimum reviewable gates

No broad tests before source freeze. Focused API tests check old Options/default/OptSize preserve exact legacy output; unknown/conflicting policies reject before factory callbacks; Steps retains resolved enum; normal and annotated paths emit byte-identical output for each policy; cloning/ownership prevents late mutation. No shared package globals. Existing register/predicate/cache/fallback tests must cover all admitted operand pairs, byte-half liveness, incoming flags, signed/constant/IX/spill/liveA/pending exclusions and subsequent condition consumers.

A real automatically allocated small typed source should activate a live-pair unsigned comparison and pass native observable results. Verify original source/raw MIR unchanged and actual allocation/sidecar hashes rebuilt. Test annotation regeneration with a deliberately inconsistent Steps policy must fail exact binary binding rather than run another target silently.

After independent frozen API review, build one current native CLI executable and retain its exact bytes/hash before running tests. Minimal real execution: unchanged full30 source Legacy baseline and explicit policy baseline (no CSE), plus short same-executable provided-profile collect/replay if the new closed CLI option is supported. Full four-candidate speed/size selection is needed only if selector wiring changed: policy must be explicit and uniform, size must either refuse the speed policy or remain Legacy. Preserve source/workload/contexts/ASM/BIN/MIR/allocation/native roots/decision hashes and independently audit full state/events. Root owns broad gates after integration. All previous private measurements remain separate historical evidence.

## Questions requiring a concrete API decision

1. Is this stage ordinary compiler option only, or closed baseline/selector/CLI too? Keep unsupported surfaces refusing explicit opt-in.
2. Which versioned evidence/context field binds policy? Existing published source profile compatibility alone does not describe backend policy.
3. Will unknown low-level enum be an error or documented Legacy fallback? Public admission must not silently ignore user intent.
