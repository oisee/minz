# Constant byte multiplication under full scratch pressure

Base 0041d9e4. All eight legacy inline coefficients 3/5/6/7/9/10/12/15 used
pickScratch8, whose exhausted fallback is D. With BC/DE/HL/IX/IY live, LD D,A
corrupted DE from 1234 to 0034 for x=0, despite correct product and restored SP.
The independent native failures are retained in native-before.txt, including
both result in A and live input A relocated to its named spill before output.

Byte genMul now wraps its complete selected emitted sequence with the existing
saveWrittenScratch mechanism. Actual physical writes determine required saves;
DE is saved before the sequence and restored after result pickup. An available
scratch that writes no live pair requires no new save. The rule covers constant
and runtime byte paths without widening the multiplication domain. No illegal
memory ALU operand is introduced. Input A exhaustion uses the existing explicit
spill routing from the preceding fix.

Validation uses the u8 width oracle from Nanz Language Book v7 §3.1: result is
x*coefficient modulo256. Each coefficient and result mode is executed for all
256 x and four incoming full flags bytes (00/01/40/ff). This is 16,384 native
executions. BC/DE/HL/IX/IY remain exact, live A spill remains exact and SP restores
f000. Full output F is independently compared with the same arithmetic sequence
with scratch-save instructions omitted: surrounding PUSH/POP does not overwrite
the flags produced by multiplication. Arithmetic is allowed to change F; this
step does not promise preservation of an earlier MIR predicate across MUL.

An ordinary source separately loads three u16 globals, multiplies its byte
input by each coefficient and consumes all three words afterward. Eight×256
raw-independent expected results match MIR VM and native Z80 with balanced SP.
This confirms integration through normal compilation, while forced allocations
supply the otherwise difficult full-pair/index exhaustion boundary.

Full MIR2 PASS and pkg/... cmd/... compile-only PASS. Saved chosen sequences,
before/after failures and validation logs have SHA256. This is a correctness
repair: forced exhausted sequences pay their required preservation overhead;
no speed or size improvement is claimed for those cases.
