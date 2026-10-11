# Separate CP/M smoke adapter

Read-only plan; no implementation or target execution. Original `examples/nanz/self_lanz_parser.nanz` stays byte-exact (`0e92689c...`). This is an ordinary CP/M smoke artifact, separate from PGO results and closed memory-host evidence.

The source declares external `peek(u16)->u8` and `poke(u16,u8)->void`. `node_tag EQU peek` is a legitimate tail-call alias, but the ordinary emitted module has no external adapter body. Linking an alias does not supply its target. The current MIR2 print intrinsic emitter (`z80codegen_call.go`) also emits `OUT (0x23),A`; the `-t cpm` option does not turn those instructions into BDOS output on this route.

Build a separate derived assembly wrapper around the unmodified ordinary compiler output:

1. Use `ORG 0x0100`, a COM entry that saves the incoming SP, installs an explicitly reserved private stack, calls the real `main`, restores SP, and exits via `JP 0`. Audit final symbol and image bounds after assembly.
2. Supply `peek: LD A,(HL); RET` and `poke: LD (HL),C; RET` only after checking actual allocated extern contracts are HL→A and HL/C. Refuse conflicting definitions or changed contracts. These are real target memory accesses, not emulator callbacks.
3. Replace only known compiler-emitted `OUT (0x23),A` print operations in this derived artifact with a named BDOS bridge. The bridge passes the original A as E, sets C=2, calls address 5, and preserves all surrounding register pairs and AF; this also supports the compiler's inline decimal and string loops. Verify no output-port instructions remain. Do not blindly replace arbitrary source assembly or pretend the replacement is byte-identical to the normal compiler image.
4. Validate the CP/M TPA before executing the source's fixed node/string/source addresses (0x8000, 0x9000, 0xC000). Its source buffer extent must remain below the actual BDOS/OS boundary. Reserve code, globals, and the private stack disjointly below the node arena; reject insufficient TPA or stack space. A generic CP/M machine is not guaranteed to provide this fixed memory layout.
5. Assemble the wrapper and execute it under an actual CP/M environment with normal CALL 5 console behavior. Compare captured console text and parser state against the independently derived source result, and check stack canaries and final memory ranges. Record this new image's own SHA and code placement. Do not reuse a closed-host binary, its timings, or a prior PGO attestation.

The minimal fix is therefore linking real memory adapters plus a CP/M startup/output bridge. Merely suppressing the undefined `node_tag` warning or providing an emulator port hook would not establish that the COM program runs correctly on CP/M.
