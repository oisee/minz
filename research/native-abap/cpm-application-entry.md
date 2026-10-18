# Ordinary ABAP CP/M application entry

Native ABAP CLI products for `-t cpm` use an application wrapper in both `.a80`
and `.com`. For example, `mz -t cpm report.abap -o report.com` produces a CP/M
application which calls `main` on a resident 1024-byte stack, restores incoming
SP, and returns through the caller's saved return word. Explicit `-f com`
selects the same application bytes as the inferred COM format. Explicit raw
`-f code` output retains its existing payload entry contract. Terminal ABAP faults
preserve `_abap_error_code` and A, print a bounded BDOS2 diagnostic such as
`ABAP runtime error 0x07` followed by CR/LF, and then jump to the wrapper exit,
abandoning nested application frames and all remaining report code. CP/M RET to
the OS has no process error-status convention: MZE reports shell status 0 for
this clean OS return, including terminal ABAP faults. The diagnostic and saved
fault byte expose that failure; arbitrary A values from normal CP/M returns are
not interpreted as shell errors.

The static image, including code, data, runtime storage, canary, and stack, must
end below `0xFE00`. This is the initial ordinary application budget, independent
of the guarded runtime's `0x8000` limit. Before writing application storage or
switching stacks, entry reads the CP/M TPA top from the word at address 6. It
returns immediately if that TPA cannot cover the complete image, or if the
caller's two-byte return word wraps, exceeds the TPA, or overlaps the image.
Loading the program into a suitable TPA and supplying a valid return path are
still loader/OS requirements. The resident stack is a measured workload budget,
not a proof that arbitrary recursion or BDOS implementations fit it.

`--runtime=cpm` remains a separate, explicit guarded certification contract.
There is no fallback from its refusals to ordinary entry. `--runtime-stack`
continues to require that explicit runtime. Intermediate output, assertion and
measured/profile workloads, other frontends, and other targets/backends do not
automatically acquire ordinary application entry. Bare/called semantic harnesses
keep their existing entry contracts.

The tests run frozen independent `fleet` and `ledger` references as ordinary CLI
products, compare independently assembled `.a80` bytes with `.com`, and measure
native stack/canary and OS return. They also run the real MZE executable with its
normal CP/M loader and BDOS. This is emulator verification; physical CP/M machine
deployment and complete F4/file/SALV integration are separate work.
