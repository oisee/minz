# VIR word-return: correctness separately from code quality

The existing Double16ThenDec fixture emits ADD HL,HL; LD D,H; LD E,L; DEC DE; EX DE,HL; RET (6 bytes). The existing quality test still FAILs because it expects DEC HL. No expectation or compiler optimization was changed.

A separate independent native check executes that actual assembled BIN on all65536 word inputs and compares HL with (2*x-1) modulo65536, exact PC=f100 and SP=f000. PASS0.057s. This resolves the previously unverified arithmetic correctness of this specific output. Extra moves remain an open quality regression, not a demonstrated arithmetic miscompile. The fixture has the same missing explicit function ABI metadata as the original test.

ASM/BIN and logs retained; full VIR immutable c86bfcd5 run started before this new test, remains running separately. Production pipeline b6b4e0c7 continues separately. Research Z3 only on test process PATH.
