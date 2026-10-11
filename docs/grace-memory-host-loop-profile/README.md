# Closed source-loop observations (WIP)

`hir.RunMemoryHostLoopProfile(source, workload)` owns and validates the original
source-bound memory-host workload, constructs its own instrumented MIR2 VM and
installs only privately generated zero-argument void counter hooks. Source loops
must have complete valid input origins; detached origins are refused. Sources
with goto are refused because arbitrary label entries bypass structured counters. Existing
memory-host baseline and legacy no-host profile APIs retain their contracts.

`minz.memory-host-loop-observations/v1` reports real loop-statement entries and
body entries, including zero-trip statements and interrupted iterations. Nested
recursion accumulates on the original source SiteID. Each top-level invocation
has an exact delta; successful and failing invocations retain the closed memory
observation and typed access failure prefix. Later calls stop at the first error.
Counter overflow returns an error. No bound-derived guard estimates are present.

Compatibility appends `minz.memory-host-loop-events/v1` to the closed memory-host
semantics. Private hook instructions consume the declared VM budget. Counts are
observations, not a legality proof or target cost. A failed/gas-limited run is not
successful profile evidence. No externally supplied callbacks, MIR or counts are
accepted. Instrumented VM globals and generated-symbol storage are checked anew.

Current focused checks: zero-trip, recursive deltas, partial denied-read prefix,
owned inputs, repeatability, changed workload compatibility, missing origins and
baseline memory/result/access equality, condition failures, reset deltas, generated
symbol heap bounds and global-name collisions. Full original-program and independent
oracle comparison are pending. Parent production has not been changed.
