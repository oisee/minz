# Independent saved sum-image reassembly

PASS: all 18 directories (original, pointer-walk, selected at lengths 0, 1, 5, 16, 64, 255) reproduce both saved program and execution binaries through the current Z80 assembler. All 36 images match byte for byte. Structured listing bytes cover every byte exactly once, with no overlapping or missing addresses.

The bootstrap is inspected without executing it: ORG8000, SPFF00, resource pointer4000, length in C for original or IXH for pointer-walk/selected, CALL to the resolved sum_array symbol, then DI/HALT. Its 13-byte original or 14-byte transformed footprint accounts for the difference between program and execution image sizes.

Source SHA `fa0ad7d685c31aeb52b91b323bcef46e5c86c0e4390b2fd41ecd69ecfef58694`, resource hash, and typed ordered workload hash agree with the saved profile context. The stored workload file has a separately reported hash because its pretty JSON differs from the canonical typed JSON hashed by the producer. All 151 manifest payloads are tracked by Git at a7c3d33c.

Saved result rows agree with the independent triangular-sum formula modulo256. The original and pointer totals are 48495T and 13879T; module sizes are 147 and 202 bytes. The saved Speed decision selects pointer-walk, Size selects original. Both short decisions select original. These checks validate the saved measurements and ranking arithmetic; they do not retime instructions or replay a VM/Z80 CPU.

Run from minzc:

```
go run ../docs/grace-sum-reassembly-audit/audit_sum.go ../docs/grace-sum-cost-refresh-review /tmp/sum-reassembly.json
```

The tool assembles saved text only. It contains no emulator, execution runner or compiler lowering call. `report.json` records every binary hash and the assembler source inventory used for this independent check.
