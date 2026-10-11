# Original Pascal LogicTest and MathTest: finite domains

Both source files are unchanged and frozen before execution in871b03ea. `logic_test.pas` and `math_test.pas` contain all13declared functions; original main and every original module assertion remain compiled. Pascal Byte maps to TyU8 and Integer to TyI16 (`pkg/pascal/lower.go`, Turbo Pascal subset); Book v7 §3.1 gives the underlying widths and integer representation.

Independent host models follow each original expression: byte arithmetic modulo256, division by2, unsigned min/max/absolute difference/parity; signed16 Double uses modulo65536 then signed interpretation. Complete unary Byte inputs0..255; every ordered Byte pair0..255×0..255 for Diff,Max,Min,AbsDiff. Add3 executes every ordered first/second pair and16 frozen third argument boundaries0,1,2,3,7,15,16,31,63,64,127,128,129,200,254,255:1048576triples, explicitly not all16777216triples. Double executes all65536signed16bit patterns, preserving signed expected results.

MIR VM Value.I stores raw masked bits even for signed arithmetic. First run exposed DOUBLE(16384) represented32768 rather than semantic-32768; exact log before-signed-return.txt retained. This is representation, not a compiler arithmetic miscompile: source contract is TyI16, native ADD HL,HL yields8000. Final harness explicitly decodes only Double VM results int16(uint16(I)) and native actual ABI return word int16(word); unsigned Byte results are not sign-normalized. Freeze and independent expected model are unchanged. Final native signed check asserts exact declared TyI16 and reads the actual ReturnLocation.

Each mode ordinary/GraceSize/GraceSpeed runs1114895 LogicTest calls (768unary+65536Diff+1048576Add3+14sourceasserts+main) and263193 MathTest calls (1024unary+65536Double+196608pairs+24sourceasserts+main). Total4134264actual assembled native invocations, matching raw and final VM calls. No rewritten algorithm, host arithmetic execution replacement, extern binding or source trimming.

Reusable VM/native instances avoid setup overhead; native CPU registers are reset to poisoned values for each independent call. VM semantic heap remains byte-for-byte unchanged on every call. Native full64KiB comparison checks every call, with only actual observed stack extent and declared spill homes masked. This protects all code, nonprivate RAM and canaries; outputs must remain empty. Every source callable returns to exactPC7000 withSPeff2. Original main genuinely executes generated XOR A/DI/HALT and leavesSPeff0/status0; assertions lower into compiler metadata and are each executed separately. This main HALT is not represented as a callable RET.

Terminal logs, full per-module per-mode source-derived MIR/ASM/BIN/allocation/cost evidence and hash manifest are tracked. Initial full proof PASS52.513s; final signed-return-location proof has its own terminal log. Full Pascal tests PASS0.022s. No compiler production changes/fullpipeline duplicate. Runtime is plain native Z80, not CP/M environment.

This test intentionally adds substantial finite-domain work (~50s on this snapshot/load) to full pipeline testing; it is a real-source correctness proof, not a timing benchmark or general claim of all Pascal correctness. Root owns catalog projection and product status.

Final exact signed-decoded code PASS44.867s; all4134264native calls and raw/final VM passed.
