# Fresh full30 production collect/replay

All four serial CLI invocations passed under the preserved e51846f1 executable SHA9ec48c5f2eaa42e786be9adddc0b824c374d5c39a599a8d2dd0ef3181a2b65d1. Source0e926 and workloadac7d remain exact original inputs with 30 calls. No source rewrite or new selection dimension was introduced.

| Actual goal | Policy/mask | T states | Whole module bytes |
| --- | --- | ---: | ---: |
| Speed | 1/3 | 1233170 | 2490 |
| Size | 0/2 | 1236250 | 2368 |
| Legacy original control | 0/0 | 1291565 | 2388 |

Speed reduces 58,395 T against the current Legacy original while adding 102 bytes. Size reduces 55,315 T and 20 bytes. These are this exact workload's measured outcomes, not a generic benefit or a held-out result. PureSpeed still ranks T alone; Size still ranks actualBIN length alone.

All eight candidate envelopes are byte-identical across Speed/Size collect and paired replay. The two whole association inputs are preserved exactly; external -o assembly equals the chosen measured ASM, and selectedBIN is copied directly. Each directory retains 50 outputs. Independent source/state/events, 11 counters, aggregate provenance, policies and typed call-spill/native hash audit passes and is copied under reviews/independent-source. The compressed packet independently rechecks all 1,200 saved records without CPU execution.

The four runs represent 36 native workload executions / 1,080 actual invocation observations: two collections include ten native workload runs each; each replay executes eight fresh candidates. Saved envelopes contain 1,200 observation records because replay retains 120 copied input-baseline records. The packet labels that difference and does not count copies as new CPU runs.

Earlier full30 records remain historical under their own executable contexts. Whole-module footprints changed with corrected string/decimal printing and caller-home handling; this packet does not relabel old costs. Broader parent gates and CP/M full-main execution are separate lanes.

`final-evidence/evidence` contains deterministicgzip copies of the exact retained executable, inputs, commands, logs, exit statuses, all 200 CLI outputs and selected externalASM. No raw artifact was altered. The terminal-only packaging helper and source/build inventories are adjacent.

The complete local packet retains 575 exact compiler/source snapshots and the exact CLI ELF (also preserved at /home/alice/dev/minz-artifacts/grace-checkpoints/memory-host-final-full30-build/minzc-production, SHA9ec48). A web copy may omit the retained ELF for hosting capacity, but must label that omission explicitly and preserve the full local packet and executable identity.

Portable full-local verification: `PYTHONDONTWRITEBYTECODE=1 python3 audit_pack.py PACKET_ROOT /tmp/audit.json`.
