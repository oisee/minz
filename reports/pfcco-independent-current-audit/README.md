# Independent audit of current saved PFCCO replay

scripts/audit_pfcco_heldout.py imports no compiler code. It rechecks all four candidates under both goals against the frozen workload: modular arithmetic, wrapping counter, exact byte accesses, positive/summed T-states, strict Speed/Size selection, candidate BIN hashes, heldout winner and baseline BIN identity. Current packet reports/goal-audit-c1744a23/pfcco-heldout passes: Speed5568 vs6252T; Size6252 vs6252T. This is saved-evidence validation, not native reexecution or instruction legality proof.

Negative test independently corrupts arithmetic, binary, selected index, timing total and heldout BIN identity; all rejected with ValueError. One unittest covers positive plus five negative scenarios. Historical original independent native/prospective audit scripts also pass unchanged. The new auditor accepts a packet path and does not rely on Python assert, so checks remain enabled under python -O.

Full production cbf01110 and research c1744a23 are live; Lizp/PLM user decisions remain pending. No completion claim.
