# Closed memory-host source-loop observations: bounded API design

Read-only review/probe based on pushed memory-host checkpoint5fdc6846, which includes the final protected native baseline. No production source is edited in this branch. The baseline's native instruction sidecars bind actual semantic global memory accesses and generated spill accesses; they are access permissions, not loop counters. They must not be used to infer HIR statement entries from PC visits, because compiled transformations, helper splitting, coalescing and recursive calls can change that relation.

## Exact minimum API

Add a separate HIR entry point:

    RunMemoryHostLoopProfile(source *Module, workload MemoryHostWorkload)
        (*MemoryHostLoopProfileRun, error)

with an owned result:

    MemoryHostLoopProfileRun {
        Schema: "minz.memory-host-loop-observations/v1"
        Workload: owned MemoryHostWorkload
        Compatibility: original-source ProfileCompatibility
        Sites: original input HIR LoopSiteMap
        Counts: []LoopCounts
        CompletedCalls, FailedCalls: uint64
        Invocations: []MemoryHostLoopInvocation
    }
    MemoryHostLoopInvocation {
        Observation: MemoryHostObservation
        Counts: []LoopCounts  // invocation deltas, including zero entries
        Completed: bool
    }

This is an observation API, with no caller VM, callbacks, pre-supplied counters, guard choices or reusable PGO decision. Every site in the original map has one ordered counter record, even if unexecuted. Add a separate loader/compatibility schema only if persistence of these observations is actually needed; existing no-host/profile decision schemas remain strict and cannot import this new family accidentally.

Before construction own the workload with the same canonical serialization as the closed baseline. Call PrepareMemoryHostWorkload and BuildFunctionSiteMap (validates detached origins), then BuildLoopSiteMap. Fail closed when UnmappedLoops is nonzero; do not fabricate sites for missing origins, generated loop nodes, callbacks or split helpers. Bind original complete typed HIR, dependency/source hashes, exact origin paths, target, compiler executable, host implementation/permissions/reset/layout/workload digests. Counts are obtained on original input before SplitHighPressure or Grace. Normal optimized native compilation remains separate, so legitimate later splitting does not rewrite input-site attribution.

Instrumentation must remain a private HIR copy, lowered to verified unoptimized MIR and never emitted to Z80 or optimized. The existing profiler already implements entry/body hook insertion in nested blocks/if/while/range/foreach/switch, creates a collision-free namespace and retains counts on errors. Its public low-level VM exposure is unsuitable for this API, but the internal construction can be reused without exposing it.

## Counter semantics

An Entry increments immediately before the HIR loop statement is reached, before its condition/range expressions execute. A BodyEntry increments at the beginning of each entered body, before the first user statement. These are events, not completed iteration counts. A zero-trip loop has Entry1/BodyEntry0. An exception or denied byte in a condition after the entry hook keeps Entry1; an error inside the first body keeps BodyEntry1. Continue counts the body once; break/return count the body already entered, with no extra completion event. There is no inferred exit counter.

Recursive activation of the same function accumulates into the same original source-site ID. A nested call does not reset the outer activation's count. The workload's top-level calls alone determine CompletedCalls/FailedCalls; recursive calls are not extra workload invocations. Per-invocation counters are exact snapshots before/after each top-level call, so all recursive/nested work is included in that delta. Stop at the first failed top-level call; keep its count/event/state prefix and leave later calls unexecuted. uint64 increment overflow must fail rather than wrap into a plausible profile.

A partial profile is diagnostic evidence, not a completed reusable profile: FailedCalls must be nonzero and the completed/failed invocation boundary explicit. MemoryHostObservation.Failure retains only typed actual semantic byte denial(kind/address/read-write); gas or infrastructure failure has no semantic failure attestation. The profiler must not turn gas, instrumentation defects or unsupported native metadata into expected memory failure.

## Closed VM construction and instrumentation traps

Refactor the existing closed memory-host VM setup into an unexported common implementation taking either original lowered MIR or a private, verified instrumented MIR. The accepted user workload host set remains exactly the two registered byte builtins; synthetic counter hooks are added by the private profiler implementation only. Validation of the instrumented copy must admit exactly the constructed hook identities/signatures, never a prefix whitelist supplied by the user. Original source extern validation and native contract rejection remain unchanged.

The instrumented VM's appended hook function symbol tags occupy VM heap space. Check all configured regions against the actual instrumented VM storage extent after construction, not only original baseline storage. Instrumented OpLoad/Store instruction identities differ from original MIR; derive bounded global-access provenance on the actual instrumented module before installing CheckDataAccess. Keep the same original global names/widths and reset bytes, all named RAM regions, byte permissions, ordered host effects and ordinary-region accesses. Installing synthetic hooks must never replace registered peek/poke, and caller code must never retain a mutable host map.

The current original profiler adds pointer-walk GuardCounts inferred from bounds and legal-plan recognizers. The new minimal memory-host loop observation family should exclude guard counts; it needs only actual Entry/BodyEntry hook events. Do not describe bound-derived FastBodyEntries as executed loop iterations or reuse old pointer-policy envelopes. Disable that optional guard sampling internally for this family, without changing existing public profiler behavior.

Step limits must be explicit: the minimal reuse charges the instrumented MIR including hook OpCall instructions, as the existing profile contract does. Record that in semantics ID; do not compare its VM steps with baseline/native cost. A baseline can complete while an instrumented run exhausts the same gas earlier; report it as incomplete observation rather than fabricating the missing counts. Prefer one controlled instrumented execution plus comparison against the already executed raw baseline observations when integrated into a higher-level factory API. A future separate source-instruction limit needs VM accounting changes and is not implicit in this design.

## Pipeline association and profitability boundary

A separate pipeline wrapper can own the existing fresh-source factory and workload, run the already closed raw/native baseline, and run this private instrumented source copy on an independently allocated fresh matching input. Recheck all retained input HIR/SourceLocations/function paths before and after each factory call, as the current baseline wrapper does. Match every invocation's return values, all globals, all RAM snapshots and ordered semantic events to the raw baseline (including denied memory prefix). Only then bind observations and the measured native binary/cost by exact source compatibility and workload. The counting hooks themselves are never included in the measured native binary.

Counts demonstrate which original HIR sites executed. They do not prove a host-aware transformation legal, prove a compiled loop's correspondence, or supply target entry/iteration costs. Existing copy/region/ordered-byte recognizers target their exact pointer-based shapes and cannot simply accept peek/poke by permitting externs. First collect original SelfLanz site counts and actual PC costs; select a later exact host-aware helper/loop candidate only with independent source/compiled proofs and actual full-workload native equivalence. No optimization or PGO choice is claimed by this stage.

## Required meaningful tests

* Zero-trip, one-trip, nested loops and recursion into the same site, including early continue/break/return.
* Failure in loop condition versus first body, denied read after committed write, typed denial identity and exact memory-event prefix; gas exhaustion is non-attestable partial observation.
* Side-effectful range expressions evaluated once in the original order; source-loop bodies with callable aliases/captures must match raw execution or be explicitly rejected where unsupported.
* Unmapped or detached origins, shared/factory-mutated HIR, stale host permissions/reset bytes and stale semantic source metadata rejected; no counter/site mismatch or user-supplied hook collision accepted.
* Input region close to original VM heap storage but overlapping appended instrumentation symbols rejected using actual instrumented storage extent.
* Counter overflow rejection and counts immutability; old no-host loaders/selectors cannot accept this new envelope.
* Original SelfLanz30 calls unchanged, 255/256/257-byte atom quirks, recursive whitespace/comment parsing and persistent intern tables. Counts per top-level call must agree with an independent bounded lexical/source oracle for chosen simple input cases before broader measurement.

## Probe evidence

`probe.go` uses the existing intentionally low-level profiler's exposed VM solely for this read-only experiment; its manually installed hosts are not a production API proposal. Run from minzc:

    go run ../docs/grace-memory-host-loop-profile-design/probe.go

`probe.json` records three actual top-level calls. First nested(0,2) gives Entry1/BodyEntry0. Then nested(2,1) enters the outer loop once and the recursive same-site loop twice: cumulative Entries4/BodyEntries4. Finally fail_loop enters once, writes byte0 at32768 and fails reading32769; its count is Entry1/BodyEntry1 and the committed write is retained. CompletedCalls2/FailedCalls1. The program checks these independent expected numbers and exits successfully. This confirms existing hook semantics; it does not certify new closed memory-host profiling or native count attribution.
