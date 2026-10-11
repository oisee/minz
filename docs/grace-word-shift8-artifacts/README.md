# General unsigned word byte extraction evidence

`before/` retains the earlier verified 138440T/1013B hashmap baseline.
`after/` retains the current-parent actual 36-call 133994T/984B run, with complete
source/workload/host oracle, raw/precompile/postcompile observations, target,
ASM and binary. Exact JSON/MIR/HIR/ASM bytes are gzip compressed with zero mtime.
`sources/` snapshots the two touched backend files, 542-case machine regression,
original unchanged hashmap and its persistent workload test. `production.patch.gz`
contains the exact eight-line general optimization patch.

`verify_hashmap.py` is the independent sequential Python oracle adapted only to
this pack's paths and new actual binary cost/digest. Run `python3 verify_hashmap.py`.
It verifies every result, complete 82-byte global state and each byte's access
counts on raw/pre/post and actual Z80. The retained audit passed. Comparison JSON
checks exact source/workload/host equality and semantic state/access equality;
compiler profile contexts legitimately differ after changing the backend.

Directed 542 actual machine cases passed0.707s; full MIR2 passed6.420s. Logs and
review document register preservation, source/destination aliasing, spill and
live accumulator checks, unchanged signed paths and concrete scope limits.

Final fullpackages/pipeline/CLI gates passed, pipeline269.139s; showcase31same0new. ActualbuiltCLI saved all4candidates×30persistentcalls perobjective; independenthost+typedSHA auditsPASS. Separatebudget audit and combinedmanifest below.
