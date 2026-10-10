# Grace PGO per-invocation memory oracle snapshots

The raw workload oracle now captures declared globals and resources after each
successful invocation, including void calls. Z80 replay compares that state
immediately after each return, before starting another invocation. A later reset
can no longer hide an incorrect earlier store. Final-state checks remain in place.

Files changed by this task:

- `minzc/pkg/hir/profile_run.go`: adds `ProfileMemorySnapshot` and explicit
  `ProfileRun.Snapshots`; independently owns each boundary and final memory copy.
  Failed calls do not produce a successful-return snapshot.
- `minzc/pkg/pipeline/profile_workload.go`: requires one complete snapshot per
  invocation, validates declared names and storage widths before execution,
  compares every boundary, and records owned snapshots in invocation evidence.
  Legacy final-state-only oracles, missing/null maps, and null empty resource
  observations are rejected.
- `minzc/pkg/hir/profile_run_boundary_test.go`: tests void set/reset observations,
  untouched globals/resources, empty resources, JSON round trips, independent
  ownership, partial failure, and the strict separation of loop-profile JSON
  from the raw-run envelope.
- `minzc/pkg/pipeline/profile_workload_boundary_test.go`: tests unchanged workloads
  and mutant assembly whose first void call stores 99 before a later reset to 0.
  Both global and resource mutants fail at invocation 0 with got `[99]`, want
  `[1]`. Also covers owned evidence and serialized missing, null, short, extra,
  reordered, wrong-name, and wrong-width snapshots, including empty memory.

This task made no commits. This task did not edit `pkg/mir2`, `profile_joint.go`,
`grace.go`, or CLI files. Other agents' changes were left intact. No `AGENTS.md`
was present in this checkout or its ancestor directories. The independently
modified joint runner now also populates the new snapshot field.
During final verification, another agent committed the shared checkout as
`d80c352c` (`feat(pgo): measure common Grace variants with invocation memory and MIR lineage`),
including these four implementation/test files. This task did not run a commit
command or alter that commit. This report remains an uncommitted new file.

Commands run from `minzc`, with the requested Go 1.26.8 toolchain and cache:

```sh
GOCACHE=/tmp/minz-grace-gocache /home/alice/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.8.linux-amd64/bin/go test ./pkg/hir ./pkg/pipeline -run 'Test(ProfileRunMemorySnapshots|ProfileRunSnapshotsPartialFailure|Z80ProfileWorkload)' -count=1
GOCACHE=/tmp/minz-grace-gocache /home/alice/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.8.linux-amd64/bin/go test ./pkg/pipeline -run 'TestZ80ProfileWorkload' -count=1
GOCACHE=/tmp/minz-grace-gocache /home/alice/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.8.linux-amd64/bin/go test ./pkg/nanz -run 'Test(ControlledProfile|LoopProfileCompatibilityAndRefusals|AutomaticProfileContext)' -count=1
```

All three commands passed. The second command includes the final null-empty-buffer
validation. Targeted `git diff --check` passed.

Broader validation command:

```sh
GOCACHE=/tmp/minz-grace-gocache /home/alice/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.8.linux-amd64/bin/go test ./pkg/hir ./pkg/pipeline ./pkg/nanz -count=1
```

HIR passed (9.800s). Pipeline passed (102.123s), including the independently
modified joint-runner integration. Nanz failed solely in
`TestShowcaseCompileAssemble`: missing imports such as `zx.console`, `agon.tui_render`,
and `tui.render`; invalid generated assembly in examples; frontend/type errors;
and example assertion failures such as `ex19_assert_forms.nanz`, `crypto.lizp`,
and `promoted_arithmetic.nanz`. These example/compiler issues are outside this
task's workload runner scope and were not changed. This command started before
the last null-empty-resource validation change; that change was covered by the
subsequent passing workload tests above.

Nanz validation excluding the failing showcase test:

```sh
GOCACHE=/tmp/minz-grace-gocache /home/alice/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.8.linux-amd64/bin/go test ./pkg/nanz -skip '^TestShowcaseCompileAssemble$' -count=1
```

Passed (2.229s). The four changed/new Go files are gofmt-clean.
