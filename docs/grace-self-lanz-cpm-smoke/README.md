# CP/M full-main preparation: blocked string contracts

The unchanged SelfLanz source and exact ordinary assembly are preserved here. No COM execution has been performed, and no PGO image or evidence is relabelled as a CP/M main run. The intended environment is mze CP/M BDOS emulation, not a physical machine or a complete installed operating system.

Static checks confirm extern allocation contracts `peek(HL) -> A` and `poke(HL,C)`. Ordinary extern labels have empty bodies. The main function ends with `XOR A; DI; HALT`; a derived COM startup must explicitly adapt that terminal path, rather than assume main returns. The planned bridge must preserve AF/BC/DE/HL/IX/IY around CALL 5, function 2.

Two independent string problems prevent an honest unchanged-source success claim:

1. The source main assigns a plain literal to `^u8` and reads until NUL. `nanz/parse.go` defaults plain literals to SString; the emitted `_mir2_str_3` begins with length 45 and has no NUL terminator. The pointer therefore addresses the prefix. This source fixture needs an explicitly derived CString form to express its intended memory loop. That change must not be hidden in the assembly wrapper.
2. `@print` lowers all string arguments to `@mir.io.print.str` without preserving their representation. `z80codegen_call.go` consumes that intrinsic as a NUL-terminated loop. Book v7 section 18 explicitly documents SString and LString printing, so this is a separate compiler contract defect. The print lowering must be fixed and independently tested before claiming ordinary main output is correct.

No string pool bytes, pointer references, or print loops were rewritten. Before actual CP/M execution, independently review the derived source (if authorized), wrapper modifications, ORG 0100, image/global bounds below 8000, stack canaries and source buffer below the explicitly modeled BDOS boundary. The fixed-memory layout is not portable to an arbitrary CP/M TPA.

A finite diagnostic wrapper is prepared and assembled only. It replaces exactly 16 known OUT instructions with the preserved-register BDOS bridge, fills the two empty memory adapters, and changes only the unique main terminal block to RET. All original print loops and string bytes remain intact. The static symbol image is below the arenas, but mza produced a 6157-byte COM for an ORG span through 7004; its multi-ORG file mapping needs further audit before any execution. No CP/M run is claimed.
