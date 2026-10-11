# Typed call-spill certificate plumbing

The caller-save repair introduced `EX (SP),HL` around compiler-owned spill homes. The closed runner refused its unproved stack access. This stage adds typed finite save/restore descriptors rooted in actual MIR, allocation, assembled call target, spill symbol and exact instruction bytes. Runtime admission checks initialized slots, call depth and identity, saved-word integrity and LIFO retirement.

The target captures every resolved stage and its canonical typed SHA. Policy associations bind that hash. Invocation setup and successful completion require no outstanding stages; result-frame recipes cannot overlap call-spill instruction PCs. Indirect/RST call snapshots and dynamic allocation overrides remain outside this admitted contract.

Both focused CLI tests passed under the retained executable `4d818aab4c5bbae1e5e794a7d6c1f9c8d22e1421f651c174fdc908a3f1b3d653`: policy collect/replay in 54.12 seconds and eight-combination search collect/paired replay in 272.39 seconds. The overall exit status is 0. Persistent cleanup copied the complete inputs, association envelopes, candidates, decisions and external selected assembly before temporary directory removal.

| Actual short test | Goal | Selected | T states | Bytes |
| --- | --- | --- | ---: | ---: |
| Fixed preserve-speed policy | Speed | mask 1 | 701 | 2366 |
| Eight-combination search collect | Speed | policy 1 / mask 1 | 701 | 2366 |
| Eight-combination paired replay | Size | policy 0 / mask 2 | 796 | 2224 |

These zero-body controls exercise transport, replay, exact output binding and typed stack permissions. They do not establish full-parser profitability or new dynamic compare counts. Earlier full-workload measurements retain their original executable context; new backend costs require fresh measurements before a current profitability claim.

The packet preserves the exact executable, terminal log and exit status, 19 frozen source files, compile-only debug captures, and all collect/replay files as deterministic gzip. The source inventory checks original file hashes; the manifest checks both stored and decompressed hashes. Independent saved audits are copied under `reviews/` when final. No CPU execution is needed to inspect this packet.

The previous parent pipeline failed on the missing certificate. This bounded success does not claim a fresh complete pipeline pass. Root owns integration and subsequent broad checks. CP/M full-main work remains a separate stage.
