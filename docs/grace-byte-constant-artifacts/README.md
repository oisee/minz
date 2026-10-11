# Byte constant operand correctness repair and negative slot experiment

This pack preserves the general two-guard ALU repair, 450 actual
machine regression source and before/after logs, full MIR2 pass, complete original
and manual slot workload evidence, and the initial wrong-code history.
Exact JSON/MIR/HIR/ASM/patch bytes use gzip with zero mtime. Older nested manifests
retain their historical filenames; root final manifest will bind packaged paths.

`history-slot-failure/` contains the unregistered manual HIR prototype and failed
actual target. Its null target is an explicitly rejected run, not completed
proof. `backend-before/` retains the investigated wrong code and allocated MIR.
`slot-after-repair/` proves the observed 36-call caller after the general repair:
141354T/1047B. `verify_slot_after_repair.py` independently checks its source,
full results/global state and every global byte's read/write counts, plus machine
binary digest and actual cost. The retained audit passes.

This prototype is slower and larger than the earlier original133994T/984B.
It is not a production rule, registered PGO choice, or source-origin proof.
`original-after-repair/` retains the current isolated general repair cost:
133994T/984B, exactly matching previous word-shift8 source/workload/ASM/binary.
Final guards fix wrong code while preserving materialized constants safely. No
general speed gain is claimed. Final450machinecases pass0.291s and both36caller
workloads pass0.648s. Intermediate conservative costs/patch and old fullMIR gate
are retained under history-conservative-guards, without final-gate attribution.

Final parent pipeline passed271.656s, packages HIR18.482/MIR4.664/Nanz1.591/C895.254s, CLI20.054s; showcase31same0new. Budget freshcompile251.813s and independent assembly audit prove exact source/ASM/BIN compatibility with previous wordshift machine gate. Current tokenizer CLI independently remeasured all4 variants for both goals and host/typed evidence audits passed. See final-*.txt, budget-check/ and cli/.
