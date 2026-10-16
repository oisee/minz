# Nanz + C for Z80 system code

Implementation checkpoint: 16 October 2026, PR [#162](https://github.com/oisee/minz/pull/162).
Implementation and integration evidence is tracked in that PR and the linked
reports. This is a supported subset with executable examples, not a claim of
full C89/C99/C17 conformance.

## Import a C source module

Nanz resolves a local `.c` module and compiles it through the same HIR → MIR2
→ Z80 pipeline. No host C compiler or object-file linker is involved in this
import. Local quoted headers are resolved relative to that C source.

[The minimal example](../examples/system-c/interop/main.nanz) has three files:

```c
/* arith.h */
unsigned long step(unsigned long value);
```

```c
/* arith.c */
#include "arith.h"
unsigned long step(unsigned long value) { return value + 1UL; }
```

```nanz
import arith { step }

fun main() -> u32 {
    return step(65535)
}

assert main() == 65536 via z80
assert step(4294967295) == 0 via z80
```

From the repository root:

```sh
go -C minzc run ./cmd/minzc ../examples/system-c/interop/main.nanz --asserts=z80 -o /tmp/system-c.a80
```

Compilation executes both assertions on assembled Z80. Imports can also use
`import arith { step as next }`, `import arith {*}` or qualified `arith.step`.
The same resolution applies to assertions. Imported functions, typed globals,
struct fields and local headers have separate native and raw/final VM tests.

C can import source modules with `#import "block_device.nanz"` or
`#import "helpers.c"`. The CLI supplies the mixed C/Nanz resolver for `.c`
entries too. Nested headers and imports resolve from their own file's directory.
Declare called functions and shared globals explicitly in C prototypes or
headers; importing a module does not generate those declarations. The FAT12
example demonstrates a C → Nanz callback.

The low-level Go API takes `nanz.CSourceResolver(sourceFile, parseOpts)` in
`c89.CompileOpts.Resolver`. A `#import` without a resolver returns an explicit
error. The standard loader isolates file-static functions and globals in each
imported C translation unit, including references through pointers. `extern`
object declarations refer to linked storage rather than allocate another copy.
Conflicting public function definitions return a link error; identical repeated
functions are coalesced. This is a source-module linker, not a complete host C
linker.

## Target model

| C type | Z80 representation |
|---|---|
| `_Bool` | One byte, normalized to 0 or 1 |
| `char`, `signed char` | Signed 8 bits |
| `unsigned char` | Unsigned 8 bits |
| `short`, `int` | Signed 16 bits |
| `unsigned short`, `unsigned int` | Unsigned 16 bits |
| `long`, `unsigned long` | Signed/unsigned 32 bits |
| Object/function pointer, `size_t` | 16 bits |
| `ptrdiff_t` | Signed 16 bits |

Storage is little endian and byte aligned. Structs are packed; inline arrays
occupy their full size, union members overlap, and array strides use the actual
object size. Automatic objects have separate storage per activation when
addressed. Aggregate parameters copy by value into private callee storage;
physical argument placement uses the compiler's own register ABI.
This is source interoperability, not binary compatibility with SDCC objects.

Integer promotions use this target model: an unsigned 16-bit short promotes to
unsigned int because signed int cannot represent its range. Conversions and
unsigned wrap retain the declared width. Pointer arithmetic scales by element
size. C rules still constrain valid pointer arithmetic and signed overflow;
the machine's 16-bit address wrap does not make out-of-bounds C accesses valid.

Enum storage chooses a compatible integer type from the target range.
Enumerators wider than int are an accepted frontend extension. They do not
establish full C23 enum support. Used 64-bit integer values, floating values,
variable-length arrays and executable bit-field storage/access currently
receive explicit errors. Compile-time `sizeof(long long)` can still return 8.

Existing C99 syntax includes mixed declarations, `for` declarations, designated
initializers and compound literals. Runtime and library support are checked
separately from parsing. Bundled minimal libc headers do not imply a complete
hosted C library on Z80: provide implementations for external functions and
inspect diagnostics. Predefined compatibility macros are not a conformance
certificate.

## Observable memory and ASM

`volatile` accesses preserve observable reads and writes through optimizations.
It applies to scalar objects, fields, arrays and whole aggregate copies;
source qualification survives by-value calls and returns. Imported Nanz fields
retain their C qualifier. Multi-byte scalar accesses visit low bytes first.
Aggregate copies snapshot source bytes before destination writes, including
when storage overlaps. Volatile does not provide atomicity, locking or banking.

Side-effecting lvalues are evaluated once. For example, `*pick() += 2` calls
`pick` once; assignment expressions yield the converted assigned value without
reading a volatile destination back.

Nanz ASM is the device boundary in the FAT12 example. For functions with local
stack frames, only freshly analyzed straight-line ASM without SP access is
admitted; opaque ASM, jumps, CALL/RET and PUSH/POP are rejected. Explicit DI/EI
retain their behavior. This admission does not silently disable interrupts or
restore their previous state. Arbitrary stack-changing ASM needs a separate
contract and is outside this supported path.

## Run the disk example

[The FAT12 reader](../examples/system-c/fat12/README.md) executes:

```text
Nanz application → C FAT12 → Nanz block driver → Z80 IN/OUT → disk image
```

```sh
python3 examples/system-c/fat12/create_image.py /tmp/demo.img
go -C minzc run ../examples/system-c/fat12/run.go -image /tmp/demo.img -output /tmp/read.txt
cmp /tmp/demo.txt /tmp/read.txt
```

The runner assembles and executes Z80, checks return PC/SP and saves output
only on success. Add `-size` for Grace Size. This is a read-only image-backed
sector-controller model, not a physical FDC driver. It supports one open root
8.3 file on FAT12 with 512-byte sectors and 1–8 sectors per cluster.

Its tests use upstream FatFS compiled by GCC as an independent host oracle.
The Z80 filesystem remains our small C module. On MIR VM, only individual
port-I/O assembler routines are modeled by callbacks; the Nanz sector loop and
C filesystem execute normally. Eight success/error workloads check bytes,
status, sector requests and native PC/SP in default, Grace and Grace Size.

[Evidence and implementation reports](../reports/system-c-z80/objective.md)
cover integer arithmetic, pointer semantics, local storage, aggregate layout,
volatile events and the disk example. Required integration checks and a
post-merge launch remain necessary before declaring the system-code goal done.
