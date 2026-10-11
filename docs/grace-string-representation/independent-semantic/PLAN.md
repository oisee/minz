# String representation and printing triage

Read-only source review at7a4820ad. No compiler change, parse/VM test or native CPU was run. The fixture files and EXPECTED.json define independent specification-derived byte/output oracles for a later bounded test.

## Three distinct issues

1. LanguageBookv7§18.1 defines plain literals as SString `[len:u8][data]`, `l` literals as LString `[len:u16le][data]`, and `c` literals as CString `[data][0]`. Parser parse.go5290 and HIRLower52 preserve that kind; Z80data emitter328 follows it. Original SelfLanz main231 assigns a plain literal to a raw pointer and scans untilNUL. That is a source representation error: first byte45 is the string length, not `'('`; no terminator belongs to this literal. Changing the source to `c"..."` is an independent source correction, changes its SHA and must not be relabelled as the previously proved unchanged-source workload. Function-level parser workloads supply their own correctly bounded source resources and are unaffected.

2. MIRVM ResolveSymbol953 always allocates data+NUL and ignores StringPool.Kind. Native data follows the representation. Direct peek of plain `"AB"` should return2, but the present VM resolves65. This is a VM storage semantics bug that can conceal the source bug. It is distinct from register allocation or Z80 instruction code generation.

3. Parser @print5343 lowers all three formats to the same @mir.io.print.str intrinsic. That intrinsic's VM and Z80 implementations scanNUL. The language book explicitly permits @print on S/L literals; therefore native plain printing can output its length and continue beyond the literal, while native LString stops on its zero high-length byte. VM storage normalization can mask these errors. Original custom SelfLanz print_str reads a Pascal length and is correct for interned strings; it is not the @print intrinsic.

## Smallest bounded repair proposal

Repair VM symbol allocation to preserve the existing StringPool encoding, including length prefix and no invented S/L terminator. Keep CString behavior and cached-address identity. Validate byte lengths against representable S/L limits; the current emitter uses byte counts and should not silently rely on assembler truncation for unsupported long SStrings. That limit issue can be separately refused/documented if beyond this slice.

For direct @print literals, select representation-specific HIR intrinsic names at parse time using the just-interned AddrOfExpr and module StrKinds: existing @mir.io.print.str stays CString for compatibility; new print.sstr and print.lstr carry explicit known representation. No string is reencoded solely for printing, so passing the same literal to peek and @print has one storage contract. Arbitrary pointer expressions lack representation type metadata today; retain their documented CString behavior rather than guess. Typed String/SString/LString/CString parameters need a separate tracked representation contract if later supported; do not claim this initial literal dispatch solves arbitrary dynamic strings.

Implement S/L counted printing as ordinary HIR/MIR load-and-output loops or small equivalent explicit VM/native intrinsic emitters. Prefer ordinary lowered loops when they preserve normal allocator/call liveness treatment; handwritten Z80 intrinsics must declare/follow actual HL pointer and A/F/count-pair clobbers and materialization/call-save rules. Counted S uses its u8 length; L uses u16 little-endian length; output exactly that many bytes including embeddedNUL, with zero length exiting before data read. CString remains firstNUL termination. Do not add generic I/O or memory-host permissions to existing closed evidence harness.

## Bounded independent gates

repro/representation.nanz has three first-byte returns: S2,C65,L2. Check rawVM, optimizedVM and assembled actualtarget against EXPECTED.json. Inspect exact data bytes2,65,66 /65,66,0 /2,0,65,66 and repeated pointer identity. No expected value comes from existing VM execution.

repro/print_formats.nanz has directliteral S/C/L, empty and embeddedNUL cases. Capture raw stdout bytes and actual OUT0x23 events, not decoded text or substring checks. S/L `A\0B` must emit65,0,66; C emits65. Empty literals emit nothing. Add a neighboring literal with distinct bytes to detect overread, and a live value across @print to catch clobber errors. Add byte-length boundaries255 S and256 L, subject to explicit length admission. Existing nanz @print test only searches ASM substrings; it does not prove these outputs.

repro/main_source_boundary.nanz demonstrates the source distinction without unsafe unterminated scanning: plain `(a)` begins3, CString begins40. A later corrected whole main smoke must be named a changed-source experiment and independently checked, preserving original source and known-failure history.

Require frozen minimal source inventory, independent backend/cache/ABI review and root release before actual CPU. No broad suite or optimizer rollout is needed for diagnosis. Existing transfer and source-bound certificates remain attached to their original source SHA.
