# Fresh full30 preparation review

Reviewed executor preparation checkpoint `bfda095f` without execution. The copied source is 7164 bytes, SHA256 `0e92689c6bb4a92fd681c9bc70d5d50549631a0fdd669cd026f723cf4e197526`. The workload is 33501 bytes, SHA256 `ac7d90cb3501e183a7fc9d45242259551e9cd260db53731704ef0a31ee6f933f`, with thirty calls. Both match the immutable earlier inputs.

The runner validates the real logical source in the selected compiler cwd and the executable's expected SHA before any execution. It retains an executable copy, uses a fresh output directory, runs serially, preserves logs/status and stops on failure. Success checks all fifty CLI outputs, selected external assembly bytes and byte-identical supplied associations for replay. Source/executable hashes are rechecked before each invocation. Mutable output files remain diagnostic artifacts until independent full semantic and typed-hash audits.

Two collect-only goals require twenty native workload executions, or 600 observations. Collect plus paired replay requires thirty-six executions, or 1080 observations. Replay runs fresh native candidates; it is not an offline audit. Root must select scope and release the exact frozen executable after the pending decimal-printer repair. No new execution was performed during this review. Historical native contexts must remain historical.
