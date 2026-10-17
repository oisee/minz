# ABAP TYPE i now occupies four bytes on Z80

The previous frontend mapped ABAP TYPE i to unsigned 16-bit storage. It now
uses signed i32 in globals, local DATA, literals, assignments and reference
FORM parameters. Global initializers write all four bytes. Signed boundary
literals are preserved, including -2147483648; out-of-range literal
initialization and assignment are explicit admission errors. Negative DATA
VALUE and PARAMETERS DEFAULT literals are parsed as complete literals rather
than dropping their sign. Unsupported initialization expressions are refused.

Numeric CLEAR writes the complete i32 object. The earlier reference-parameter
and expression probes now compare all four bytes, rather than only the low
word. This is storage and execution progress, not complete ABAP arithmetic
conformance.

## Independent wide-value evidence

Two unchanged [integer-reference sources and results](../../minzc/pkg/abap/testdata/integer-reference/)
were executed with the pinned independent @abaplint/transpiler/runtime 2.14.8:

| Probe | Observable globals after the explicit FORM call |
|---|---|
| wide-reference | x=210001, y=-20000, seen=-2147483648 |
| boundaries | x=-2147483647, y=2147418111, seen=0 |

They cover wide/negative initialization, local INT_MIN, four-byte USING and
CHANGING, signed comparisons, addition/subtraction/multiplication and CLEAR.
Raw VM, final VM and assembled Z80 match those globals. Native execution
returns at PC=F100 with SP=F000. The test invokes FORM probe; declaration
alone does not invoke it, and these are not yet the full report acceptance gate.

## Decimal output and an additional backend correction

The new abap_write_i32 uses ordinary typed
[Nanz code](../../minzc/pkg/abap/integer32.go) for unsigned magnitude conversion,
decimal division/remainder and reversed digits. Only character output uses
the CP/M BDOS or Spectrum character primitive. Converting INT_MIN through
unsigned magnitude avoids signed negation overflow. The existing compact
CP/M convention of a trailing space is retained; full ABAP list formatting
is not claimed.

The negative-output probe found a backend defect: OpNeg of width 24/32 went
through the byte NEG path. For example, -1 printed as "-0" on native Z80
while both VMs printed correctly. Wide NEG now uses the existing private
workspace/register-save machinery, with bytewise borrow propagation and
publication to the actual allocated destination. Allocation checks cover
both register banks, spills and aliases, and preserve unrelated registers,
flags, interrupt mode and stack.

Decimal outputs for 0, 1, -1, 65536, -65537, INT_MIN and INT_MAX match on
raw/final VM and native Z80. VM tests model only the character-output boundary;
native tests capture actual BDOS calls and check PC/SP.

Validation: the complete ABAP and native parser suites pass. The full native
wide arithmetic allocation matrix also passes after the NEG change. A freshly
built CLI compiles wide-reference.abap with PATH=/nonexistent, confirming this
path needs no Node executable.

## Still required

Runtime overflow checks, ABAP `/` rounding, signed DIV/MOD conventions,
full-width selection-screen input, exact admission for other numeric types,
strings/character copies and remaining system-field/control semantics still
need work. Existing TYPE int8/f/p mappings are not certified by this checkpoint.
Main integration and the final unchanged-program gate remain open.

The independent runtime's Integer.set overflow check is commented out in
2.14.8. It is useful for these in-range cases, but cannot serve as the sole
oracle for ABAP integer overflow. Overflow requires a separate specification-
derived oracle and failure-path checks.
