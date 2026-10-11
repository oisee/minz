# Independent saved decimal-printer audit

Frozen source checkpoint `4d6522d1`; retained MIR ELF SHA256 `1b42a7b079ce647e5bfe0786f7127e579cdbc68582c863bef6fab3c9d0361555`, pipeline ELF `75e980b563ef52ce03230d5dc0c08e0b2b37af1bd26f8d7b3fc2230445fe8a02`. The executor's once-only 198 physical ABI cases and two parsed source routes passed. This audit performs no CPU execution.

All 198 output byte strings were independently recomputed as decimal ASCII of `value & 255` for both intrinsics, eleven allocated locations and nine boundaries. The expected initial register patterns were independently reconstructed from each case identity, then all saved A/F/BC/DE/HL/IX/IY/SP values and terminal PC were checked. Recorded VM output was checked against the independent bytes, not used as the sole oracle. All two source files exactly match the reviewed fixture: x=256 prints 0, x+99 prints 99, x+17 returns 273. Saved output `099`, HL/answer273, stack and post-HALT PC match for Steps and legacy; legacy Steps is explicitly null.

Offline assembly of every one of the 200 saved ASM files reproduces its saved BIN exactly. The tool imports only the assembler and contains no emulator. Executable identities and per-case ASM/BIN SHA256 values are in `result.json`; all reassembly results are in `offline-reassembly.txt`.

Limits: the physical fixture's final spilled RAM byte is not exported in execution JSON. Its unchanged-memory assertion passed in the preserved executor, but this auditor does not invent an independent final RAM attestation. The MIR JSON omits type internals, so the valid typed fixture contract is established by review of the frozen sources rather than re-derived from that diagnostic JSON. Pending flag/accumulator controls are static tests, not additional native cases. No CP/M or generic host permission claim follows from this ordinary output-port test.

Portable saved-only commands (helpers accept compressed files when the plain path is absent):

```sh
python3 tools/audit_decimal.py EVIDENCE_ROOT --report result.json
# From a minzc Go module cwd:
go run tools/reassemble_decimal.go EVIDENCE_ROOT/actual
```

The first local reassembly command used a wrong relative tool path and failed before assembly or CPU execution; the corrected absolute-path command successfully assembled all 200. No native case was rerun.
