# Corrected string native fixtures: final saved-only audit

PASS all fifteen corrected fixtures: twelve printing cases and three storage/dereference cases. Independent MZA reassembly of each exact full assembled source reproduces its BIN byte-for-byte, origin 0x0100, and complete symbol map. The module-only ASM remains a separate diagnostic. No CPU execution was repeated.

The saved executable hash is `8d60ea6b6a80a76e89046cde9bbeacd9946ea5abc85a6883f469e5821f8801db`, matching the corrected-run identity at source `9eb88b7c`/inventory `4b5d3a35`. Every execution has an empty error, positive model-cycle count, SP 0xF000, and PC at the actual HALT opcode after DI.

Independent byte oracles agree for S/C/L, UTF-8, empty strings, embedded NUL, S255, and L256. All prints execute the later `!` and store the independently expected live-value result 4661. Generated-name globals stay 123 and 456. Storage images have exactly `[2,A,B]`, `[A,B,0]`, and `[2,0,A,B]`; actual first-byte loads return 2, 65, and 2. These checks distinguish the expected pool bytes from stdout bytes.

The repaired S image explicitly saves AF, loads the allocated H byte into A, outputs it, and restores AF before decrementing the live A count. The L image loads its allocated D byte into A. Allocation is ordinary PBQP; no manual location override is introduced. Combined with the exact outputs and successor state, these fixtures close the observed builtin argument-materialization failure.

`assembly.txt`, `bytes-state.txt`, `identity.json`, and `raw-inventory.json` record the audit. Scripts are adjacent at `audit_string_assembly.go.txt` and `audit_strings.py`. First failed artifacts remain separately under the owner's `actual` directory: ROM-protected globals and the HALT oracle were fixture issues, while S/L output exposed the repaired ABI defect. This is ordinary native emulator smoke; no successful full CP/M main or closed-host/PGO certificate is claimed.
