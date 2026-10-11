# Independent workload-driven SelfLanz oracle

Base b770f3be. New artifact-only Python model; no compiler or permission changes.
Input is actual closed MemoryHostWorkload JSON, not a fixed text list or saved
expected output. Initialize complete source-declared globals and workload overrides;
apply each call's globals and region patches in source workload order. Preserve
resource and string arena state between calls, resetting only explicit patches.

Exact source semantics: cur/whitespace/comments/recursive parse, original list
allocation and sibling stores, len:u8 truncation, digitfirst classification even
for mixed non-digit tails, modulo16 numeric accumulation, Pascal arena search,
arena-before-source compare and header/read/store append sequence. All writes
and reads use modular u16 addresses and consult declared regions; read-only or
undeclared first denial retains every previous event and partial memory effect.
Full10globals and all full region bytes compared per invocation.

Initial supported caller is parse_sexpr; core pure/source helpers may be admitted
explicitly if corpus requests them. Printing/main/external user callbacks are
excluded, matching closed harness constraints. Step gas is not independently
modeled and native T-state costs are not predicted. A protective Python recursion/
operation cap is infrastructure refusal, not fabricated typed semantic denial.

Verifier reads saved baseline evidence (plain/gzip), recomputes every invocation
from workload, compares raw/native results, globals, regions, successfulresource
events and typedfirstdenial. Bind workload profile and sourceSHA; verify binary
SHA and actual percallTsum. It never trusts compiler-produced expected snapshots.

Corpora planned by owner: repeatedshortatoms, longatoms<=255, mixednestedforms,
fresh/matching symbols. Require bounded source/node/string capacities and actual
both-site observability. Costs/winners measured by separate owner actualnative;
no sum/extrapolation/prediction. Single-file/source identity unchanged.

Checks before native: model terminalcomplete, regioncapacities and exacthostevent
semantics independently reviewed, bothsites genuinely reached, persistent resets
explicit. Then owner samefinalCLI speed+size actual4 for each selected corpus once.
Reviewers audit saved payloads without extra CPU replay.

The implemented model is explicitly frozen to original source SHA256
0e92689c6bb4a92fd681c9bc70d5d50549631a0fdd669cd026f723cf4e197526.
Modified source is refused even with internally consistent evidence hashes.
Infrastructure/gas failure has no typed semantic denial and is never accepted.
