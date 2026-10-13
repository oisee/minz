# Gallery asset integration verification

The integration branch contained the corrected `examples/nanz/rotozoomer.nanz`
and independent signed-16 gallery test, but its README GIF still came from the
earlier source. The resulting failure was frame 9, pixel (2,0). Copying the
corrected research `media/rotozoomer.gif` repairs the mismatch. Its SHA-256 is
`d975ec156a816b67ebc4a2f63715413772361c7d1ff8510f50d8c93f6d24d5d7`.

The research and integration media directories differ only in this GIF. The
rotozoomer source, generation script, and `reports/rotozoomer-pure` source,
freeze, and manifest files already match; they require no additional changes.

Validation from `minzc`, with cache disabled:

```sh
go test ./pkg/c89 ./pkg/nanz \
  -run '^(TestPlasmaRender|TestCanvasImplShowcase|TestRotozoomerGallery)$' \
  -count=1 -timeout=15m
go test ./pkg/nanz -short -count=1 -timeout=15m
```

Both passed. The focused packages took 0.691s and 6.392s; the complete Nanz
short suite took 21.007s. The rotozoomer test checks all 32 frames against both
the current-source VM and an independent signed-16 model, verifies frame 32
wraps to frame 0, and checks dimensions, infinite looping, and frame delays.
The existing integration core short checkpoint also passed: HIR 29.118s,
MIR2 71.640s, VIR 4.057s, and MZE 0.496s. This asset-only change does not
require repeating those compiler checks.

## Pipeline timeout and complete process sharding

The separate pipeline short run exhausted its **package-wide** 15-minute
deadline. At that moment `TestMemoryHostPolicySearchLifecycle` had run for
3m42s and its `/48` mutation probe for 1m0s. Its stack was runnable in
`buildAutomaticProfileContextCompatibility`, called while preparing a memory
host workload and validating original modules. The log therefore establishes
a budget failure, not a failed semantic assertion; it does not establish that
the complete pipeline suite passed.

The lifecycle test deliberately performs a successful 48-factory search across
eight candidates, mutation probes ending at factory calls 25 and 48, and a
sharing probe ending at call 25. Its successful factory also calls `runtime.GC`.
Repeated independent module checks and workload preparation make this an
expensive test even with `-short`; short mode does not skip it.

Use fresh test processes with a 60-minute budget per top-level test. Enumerate
the compiled binary's actual test list and run every entry exactly once,
preserving all its subtests. The integration inventory currently contains 567
top-level tests. A conservative sequential run avoids competing heavy native
compiler workloads; concurrency can be increased after measuring memory use.

```sh
# Run from minzc; evidence stays outside the checkout.
pipeline_evidence_dir=$(mktemp -d /tmp/minz-integration-pipeline.XXXXXX)
go test -c ./pkg/pipeline -o "$pipeline_evidence_dir/pipeline.test"
"$pipeline_evidence_dir/pipeline.test" -test.list '^Test' \
  > "$pipeline_evidence_dir/tests.txt"
pipeline_status=0
while IFS= read -r pipeline_test_name; do
  case "$pipeline_test_name" in
    Test*)
      "$pipeline_evidence_dir/pipeline.test" -test.short -test.count=1 \
        -test.timeout=60m -test.v -test.run "^${pipeline_test_name}$" \
        > "$pipeline_evidence_dir/${pipeline_test_name}.txt" 2>&1
      pipeline_test_status=$?
      printf '%s\t%s\n' "$pipeline_test_name" "$pipeline_test_status" \
        >> "$pipeline_evidence_dir/status.tsv"
      if [ "$pipeline_test_status" -ne 0 ]; then pipeline_status=1; fi
      ;;
  esac
done < "$pipeline_evidence_dir/tests.txt"
printf 'Evidence: %s; aggregate exit status: %s\n' \
  "$pipeline_evidence_dir" "$pipeline_status"
test "$pipeline_status" -eq 0
```

This recipe resets accumulated process state and the deadline for each test,
records individual exit codes, and retains lifecycle and other expensive tests.
A timeout in a fresh shard remains an unresolved check and should be reported
with its log, rather than converted into a skip or a success.

## Historical fixture audit

The subsequent process shards at `3cd87942` recorded nine failures among 440
finished tests. Their causes were missing fixture inputs and four historical
book freezes bound to the evolving live documentation. The missing files were
the ALV and report models, the CP/M, generated-screen, terminal-screen, and
Tetris TUI models, two commander event lists, and the historical PL/M hello
source. These inputs were restored byte-for-byte from research.

The four book freezes now point to
`reports/frozen-language-spec/Nanz_Language_Book_v7.md`. This snapshot was
recovered from commit `e3eef4baf245fccd60dd0e0254aca62d2c67b95f`; its SHA-256
matches the original frozen value
`c78a79e60a35235efe1beeb4282ec117b7244ab1f0b437d3490c5912672aec34`.
Only the resource paths changed. All original digests and semantic checks
remain intact, and no compiler or test code changed.

All nine failed tests passed together in a fresh focused run with
`-short -count=1 -timeout=60m` (8.078s): ALV, CP/M screen, generated screen,
report screen, strings/pointer/PLM adapter, terminal screen, Tetris TUI,
commander L3, and commander. The source/model freeze maps also match all
their referenced files. No recorded failure remains from this audit;
the remaining full-shard validation is tracked separately.
