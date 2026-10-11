# Private frozen-choice evaluator: architecture review

Reviewed `memory-host-transfer` at `91673af89440f78434cfefe570f1914396618eac`, including the original `08b9c2ae` draft. Only `minzc/pkg/pipeline/memory_host_transfer_test.go` is added; production code is unchanged. This was a source review, with no target execution or duplicated tests.

The architecture is suitable for completing the private export harness and ownership tests. It is not yet a release for the 16 native measurements.

- Inputs bind the exact preregistration, original source bytes and logical source name, four workload byte hashes, and both audited training decision byte hashes before invoking the factory. An independently parsed original also binds full compatibility, function origins, assertions, sandboxes, assertion statistics, and warnings.
- The four fixed tuples are control `0/0`, supporting original `1/0`, trained Speed `1/3`, and trained Size `0/2`. No selector is invoked on evaluation workloads. Four support executions are additional to twelve reported measurements; their policy is not relabelled from the control.
- Each tuple uses the closed baseline runner and the existing same-policy proof binder. The exact arena/append constructor and independent precompile proof remain the only transformation authority. Native global, resource, spill, and call-frame permissions remain those of the existing runner.
- The private baseline lifecycle really is original, compiled candidate, sentinel. Excluding only callback 2 for an untransformed tuple matches that lifecycle. All caller graphs are retained before compilation or transformation. A transformed tuple uses six caller returns; an original tuple uses three, giving eighteen fresh caller returns per workload.
- Original modules are checked before and after subsequent caller invocations and at the end. Source graphs remain strongly rooted, including compiler-mutated candidates. Shared graphs are rejected by the existing freshness check.
- Every raw and target observation must succeed. Same-policy and cross-policy observation comparisons omit only `Steps`; return values, globals, region snapshots, ordered byte events, errors, and typed failures remain compared. Measured costs are retained without ranking or suppression of regressions.

Remaining before native release: final export-harness review, exact eighteen-return assertion, and meaningful delayed retained-source mutation/forced-GC tests. The preregistration field `executable_sha256` is the training CLI `0d6...`; a preserved evaluation test executable will have its own context. Export and label both identities explicitly. Do not claim the new evaluation was executed by the training CLI.

The four workloads were previously used during development. The valid claim is retrospective frozen-choice evaluation outside the training workload, not blind unseen held-out evaluation. No aggregate universal winner is established by this protocol.
