# Static CP/M wrapper and source oracle

The saved guarded wrapper from `self-lanz-cpm-main` 9a5f7ae3 independently
assembles to all 2344 contiguous bytes at origin 0100, SHA
`eb4032ada45787371a057dde78c3e193f67d239f68a7ed740b3253669e9925f6`.
Every structured listing byte equals COM[address−0100]; code/image end equals
the file end and remains below 6000. Single ORG avoids the former multi-ORG
serialization hole. `flat-map.json` is a fresh assembly-only check, not CPU run.

The wrapper saves incoming SP before switching stacks, rejects BDOS top-memory
word at 0006 or incoming SP below D000, initializes canaries at 6000/7002,
uses stack [6002,7002), calls main, checks both canaries, restores system SP and
jumps 0000. Code/data, node arena 8000, string arena 9000 and input C000 are
disjoint for this fixed input. These static guards do not prove arbitrary
recursive stack depth or arbitrary CP/M TPA sizes. The BDOS character bridge
preserves AF/BC/DE/HL/IX/IY. Peek/poke wrappers implement real byte loads/stores
at HL with A result / C byte argument. This is a planned emulator smoke adapter,
not physical CP/M execution or a PGO result.

The original source SHA 0e92689c… is preserved. The explicitly derived source
574cef71… changes only main's NUL-scanned input pointer to CString. This wrapper
was emitted by historical compiler 112ae772…, before the final accumulator and
native-certificate repair; it must be rebuilt from the new compiler before
runtime validation.

Independent fixed-input parsing (`oracle.py`, no compiler/CPU) produces six
lists and eleven atom occurrences: 17 nodes, node_bump 8088. Seven distinct
strings are fun/add/a/u8/b/return/+; their length-prefixed records occupy 24
bytes, str_bump 9018. Parsing consumes all 45 input bytes. The pretty tree is
exactly the input expression.

Concrete output blocker: three current `@print_u8` sites call the character
bridge without loading the actual argument from HL. Moreover the intrinsic's
historical implementation emits one raw byte, not a decimal number. The saved
wrapper therefore cannot establish the desired textual final line
`Nodes: 17 Strings: 24 bytes`. Fix or explicitly specify that representation
before treating stdout as the desired source-level result. Input/Parsed labels
and tree have a separate exact byte oracle; they must not be inferred from
possibly wrong output. No CPU execution was performed in this review.
