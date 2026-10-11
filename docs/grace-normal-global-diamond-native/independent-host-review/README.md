The isolated normal-global diamond experiment is semantically correct on the saved original 30-call SelfLanz workload and slower: 1,137,339 T / 1998 B versus 1,135,370 T / 2000 B. It adds 1969 T and 1494 executed instructions while saving 2 bytes. This result supports keeping the helper isolated and disabled.

The independent source model checked all 60 raw/native invocation results, ten globals, complete persistent resource snapshots, ordered peek/poke events and typed failure fields. The audit verifies binary/assembly, final MIR, allocation, sidecar and call-frame hashes; the unmodified candidate exactly matches the current dba production reference. Source, workload, raw observations, HIR compatibility and function identity maps are unchanged within the experiment. Its executable context is fresh eca67 and no new loop counts or profiles are asserted.

The captured preoptimization MIR differs at exactly one instruction in cur: r57 load r56 becomes r57 move r49, with both pointers rooted at pos. Final allocation retains entry pos in DE across the comparison, adding PUSH DE / POP DE on every entry. The old inbounds pos load disappears and src_buf changes from DE to HL. These saves cost 21 T and two instructions per entry; the removed word load costs 20 T and one instruction on inbounds paths. This is a static assembly explanation, not new dynamic PC attribution. No CPU was replayed by this audit. Other module register numbers and spill labels shift after dead-address removal, so the whole measured result remains authoritative.

Reproduce using Python only (JSON and .gz payloads supported):

```sh
python3 audit.py /path/to/actual /path/to/current-production/original/factory-baseline.json final-audit.json
```

`actual` contains original/ and reuse/ saved bundles. The fixed independent model admits only the exact original SelfLanz source SHA 0e92689c6bb4a92fd681c9bc70d5d50549631a0fdd669cd026f723cf4e197526. Raw VM semantic success and closed target memory checks are reported separately from changes in internal normal-global reads and execution gas.
