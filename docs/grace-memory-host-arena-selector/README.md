# Bounded measured arena-peel selector (WIP)

`MeasureMemoryHostArenaPeel(factory, workload, serializedProfile, options)` accepts
only original-source factories and one helper (`MaxSites:1`). Goal speed/size
selects by actual measured T-states/whole emitted binary bytes. Compiler settings
remain native baseline defaults; equal primary costs retain original.

The new HIR loader accepts only closed memory-host loop-profile v1, bounded JSON,
exact typed keys (including zero fields), no duplicate keys, complete compatible
source/workload and internally consistent per-invocation deltas. Measurements
then replay the closed association and require exact serialized-profile equality:
caller counts never become trusted observations merely by loading JSON.

The variant is privately constructed from fresh original HIR. The independent
ValidateMemoryHostArenaPeel proof checks the complete precompile function/module
and exact original-origin movement. This proof is recorded as precompile only.
Normal compiler preparation may split functions: final MIR/allocations/native
sidecar are diagnostics and measured execution evidence, not a reused post-HIR
proof. Both variants execute their own raw/native baseline and must match ordered
byte events, results, all regions/globals and exact failure identity.

Factory outputs are independently allocated. The association's one compiler-
prepared candidate is explicitly excluded from retained original-identity checks;
all other originals remain checked through later callbacks. No caller transformed
factory, compiler options or assembly are accepted. This WIP needs independent
lifecycle review and meaningful selector/JSON tests before integration.
