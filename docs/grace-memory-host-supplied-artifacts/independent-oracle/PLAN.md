# Supplied-profile CLI refusal scope

Artifact-only review. Keep the current `1bd95516...` executable, original SelfLanz
source, and closed CLI unchanged. Do not add a failed-profile exporter or imply
that a synthetic payload came from an actual failed current-executable run.

The existing loader checks exact canonical typed JSON fields, then source,
workload, context, compatibility and source-site identity; next it checks sample
completeness/FailedCalls, individual invocation failure, and counter deltas/sums.
The selector additionally re-observes the complete accepted profile. Therefore a
real raw failed profile from a different executable proves the earlier context
refusal only, not the failed-sample gate.

Two minimally distinct cases:

1. Supply the authentic saved `852cf9a5...` CLI profile from
   `memory-host-arena-append-cli-evidence/small-collect/source-profile.json`, with
   its own saved workload. Run the unchanged `1bd95516...` CLI. Source bytes are
   the same original source; the expected first refusal is incompatible
   family/source/workload because compiler context differs. Preserve the entire
   original payload and its SHA; do not rewrite its context to make it current.
2. Deep-copy the successful current `1bd95516...` zero-body profile. Keep its
   workload, context, compatibility, sites and count arrays exactly unchanged.
   Change CompletedCalls from 2 to 1 and FailedCalls from 0 to 1, and mark the
   second invocation incomplete with a clearly synthetic nonempty Error. This
   is an internally consistent *failure claim*, not authentic failed-run
   evidence. The expected refusal is incomplete or failed sample. It isolates
   the failed-sample gate after genuine current source/context admission.

Each CLI invocation must exit nonzero with no output directory, selection,
selected binary, or decision. Check logs for the specific first-gate reason,
input hashes and payload deltas; preserve supplied bytes. The failed synthetic
profile must not be used as an independent memory-semantic observation or count
certificate. No native execution is duplicated by this reviewer.

Optional separate cases, only if needed, should be labeled explicitly:
individual invocation Error with healthy aggregate completion is a deliberately
inconsistent forgery; altered-but-consistent healthy counts tests fresh replay.
Neither is needed to make an actual-current-failure provenance claim.
