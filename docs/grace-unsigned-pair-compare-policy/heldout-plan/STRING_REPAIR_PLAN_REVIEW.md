# String repair and CP/M wrapper: bounded plan review

Read cost plan `a34c03ec` and the separately assembled diagnostic wrapper `171acff4`. No compiler edit or target execution was performed.

The string repair scope is sound: VM literal storage must preserve C/S/L encoding and cached symbol identity; direct literal `@print` can dispatch S/L into ordinary counted HIR loops while opaque pointers retain the existing CString convention. S/L output must include embedded NUL and stop after the declared byte count. Empty strings must not read data. Reject S length >255 and L length >65535 before encoding instead of truncating; compare byte lengths, not Unicode character counts. Source main's plain-to-NUL pointer misuse requires a separate explicit CString source correction, with a new SHA.

Prefer normal HIR allocation/liveness for counted printing. Test a value live across print, a non-tail print followed by an observable return/store, and neighboring literal bytes to expose overread. `elimJrToRet` currently recognizes only a jump to a bare RET; a conditional return in a genuine tail print is legal. Do not change that peephole on the basis of seeing `RET Z` alone. A non-tail fixture is needed to establish any erroneous loss of continuation.

The CP/M wrapper binds the exact ordinary ASM SHA, checks HL→A and HL/C extern contracts, supplies genuine byte adapters, replaces sixteen known output operations with a BDOS-2 bridge, and explicitly adapts the unique main HALT terminal path. The bridge preserves AF/BC/DE/HL/IX/IY around CALL 5. Strings and print loops remain unchanged, and the known defects are reported rather than hidden.

Execution is still blocked: the assembler's COM is 6157 bytes while the declared multi-ORG logical image spans 0x0100 through 0x7004. A flat COM load cannot silently assume those declared addresses are populated. Audit the file/address mapping or derive a reviewed contiguous image before loading it. The fixed regions also require explicit TPA checks; a modeled BDOS boundary is not a universal CP/M memory guarantee. Any later run under mze demonstrates BDOS emulation, not a physical CP/M installation.
