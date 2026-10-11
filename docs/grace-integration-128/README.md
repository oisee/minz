# Canonical register identity and current joint native audit

Pascal sort inputs frozen before selection in2adac9b3 (donor9baf339e):
complete original source, three training arrays, eight heldout arrays. No
selection or heldout result claimed yet; canonical dead-runtime admission
and actual source loop locations are being implemented separately.

MIR RenumberRegs previously omitted Asm.Ins/Outs.7ff8b1fb collects/remaps
both, preserving contract input and ASM-only output identities. Root test
plus sequential metadata remap PASS. e6d36bde excludes zero-sized immutable
pointees from fresh ownership overlap checks: Go may share their address.
Fresh BreakStmt AST accepted; shared mutable BoolLit and retained factory
mutation still rejected. Focused root result in asm-fresh-zero-root.txt.

Current12-way ordinary/PFCCO off-greedy-PBQP/Grace/checked table matrix
PASS6.917s with actual Z3 path supplied (no skip): full44→20B, caller256
60928→36352T (238→142T/call). Independent raw/final VM and assembled native
all bytes. No new prospective workload or extraPFCCO gain claimed.

Shared z80spec/rewrite/VIR completePASS (VIR2.449s). All supported pkg/cmd/
internal/tests compile on current integrated source; build-only gate is
not execution. This confirms no required live LIR import reintroduced.
Full pipelinea7954042 remains live and predates these integrations.
