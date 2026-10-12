# PL/M console output on CP/M

The compiler supports a narrow console intrinsic in statement context:

```plm
PRINT$CHAR: PROCEDURE (C);
    DECLARE C BYTE;
    CALL BDOS(2, C);
END PRINT$CHAR;
```

An undeclared `BDOS` name with literal function number `2` and one `BYTE`
argument means output exactly that byte. Other function numbers, argument
counts, and wider values produce a diagnostic. A declared procedure named
`BDOS` retains ordinary procedure-call semantics.

This is a compiler extension, rather than a general PL/M external-procedure
ABI. It lowers to the existing effectful console primitive. With
`--target=cpm --runtime=cpm`, the public guarded runtime emits an actual
`CALL 5` with `C=2` and `E=character`; it protects application values against
BDOS register clobbers. The MIR VM host models the corresponding byte output.
Without the CP/M runtime, the normal emulator console output remains in use.

Addressed string literals contain exactly their source bytes. Append an
explicit zero for a loop that reads until zero:

```plm
CALL PRINT$MSG(.('Hello from PL/M-80!', 0));
```

The addressed list accepts byte literals after its initial string, including
CR/LF. `.'hello'` still contains five bytes; `.('hello', 0)` contains six.
There is no implicit C-string terminator.

The complete [Hello example](../examples/plm/hello.plm) compiles to a CP/M COM
image using:

```sh
mz examples/plm/hello.plm --target=cpm --runtime=cpm --runtime-stack=1024 -o hello.a80
mza hello.a80 -o HELLO.COM
```

Its whole-program regression executes the original main and the emitted
runtime in ordinary, Grace Size, and Grace Speed modes. Raw and optimized MIR
must emit `Hello from PL/M-80!\r\n`; assembled Z80 must do the same with and
without BDOS clobbering all primary registers and both index registers.
The native gate checks the actual `CALL 5` instruction, exact return PC/SP,
stack guard/status, and all memory outside declared runtime stack, status,
and allocator spill areas. This verifies emulated CP/M services; physical
CP/M remains a separate deployment check.
