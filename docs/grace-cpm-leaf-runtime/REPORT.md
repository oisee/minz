# Explicit CP/M runtime: first public leaf-console COM

Ordinary CLI now supports an explicit application runtime:

```sh
mz examples/nanz/cpm_leaf_console.nanz -t cpm --runtime cpm -o demo.com
```

The resulting **524-byte COM** starts at 0100 with production startup code, uses
BDOS function 2 for console output, calls the original void main, restores the
incoming SP and returns to CP/M's caller. There is no ASM text surgery or test-only
runtime linking in this workflow. `-o demo.a80` saves the complete linked assembly.
Without the option, default port23/main-HALT behavior and profile option IDs stay
unchanged. Runtime is an omitted JSON field when unset.

The resident image includes a 256-byte application stack and a guard byte.
Startup saves incoming SP and selects the resident stack. A generated BDOS wrapper
preserves AF/BC/DE/HL/IX/IY and passes C=2/E=character to CALL5. The application's
main terminal path emits RET. Printing LF retains the existing byte stream; no
implicit CR is added.

The first slice deliberately admits only one leaf, void, no-argument main with
console intrinsics, arithmetic/control flow and normal module globals. It rejects
extern/user/indirect/conditional calls, recursion, ASM/ports/SMC, fixed-address
globals, allocator absolute spill slots and explicit stack operations. Stores
must target direct module globals. Generic pointer reads include compiler-emitted
string loops; this is not a general pointer isolation policy. The whole linked
image, stack included, must fit below 8000. Analysis of actual assembled reachable
main paths checks PUSH/POP balance, bounded depth and known generated calls;
unsupported or unbalanced paths are refused. BDOS internals' stack consumption is
outside the application bound and depends on the CP/M system.

Measured/profile modes cannot select this runtime: their origins/ABI/host models
remain separate. CP/M runtime source assertions require a separate verification
workload (`--asserts none` if the source contains assertions). The option applies
through both public pipeline compilation entry points.

The demo reads persistent `result=17`, prints, computes `x=3*17+2=53`, prints x,
branches on the still-live value, saves result54 and prints LF. Actual ordinary
CLI COM and linked ASM assemble to exactly the same bytes. A native Z80 run with
a CALL5 BDOS handler prints **`CP/M 53 OK\n`**, retains result54, exits at PC0 with
restored incoming stack, and leaves the resident stack guard intact. The handler
intentionally corrupts every primary register and flags; a separate direct wrapper
call verifies exact AF/BC/DE/HL/IX/IY preservation. The stack touched 19 bytes in
the test run. CLI checks pass in 0.035 seconds.

This is an assembled native CALL5-hook emulator proof. An actual CP/M OS execution
has not been claimed. Applications still depend on a CP/M loader/BDOS convention
and available TPA. This leaf slice does not yet admit recursive selfLanz or the
whole tokenizer: real byte-memory extern bindings, additional user calls and
proved/configurable stack support are following steps.

Frozen CLI: `/home/alice/dev/minz-artifacts/grace-checkpoints/mz-cpm-leaf-final`,
SHA256 `e9c30bb80e3dec44d898164c9d489fdaf0ce5ac744150e120df55bdec2e61881`.
