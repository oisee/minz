# Six exact assert_test sources: independent native proof

Six separately frozen paths: examples/lizp/assert_test.lanz and .lizp; examples/nanz/assert_test.nanz and .plm; examples/plm/assert_test.plm; examples/pascal/assert_test.pas. Exact source hashes pinned before execution in8569bbf4; no source edits. The two PL/M files have identical bytes, but each exact path is compiled/run and recorded separately. Other same-name files are not treated as interchangeable evidence.

Lanz/Lizp/Nanz and both PL/M sources each declare byte double/add/max. Every256u8unaryinput and every65536ordered pair for add/max tested against independent2*x mod256, a+b mod256, max(a,b). All9source asserts per file explicitly invoked and checked separately. PL/M `$` is an ignored identifier separator (`plm/lex.go:222`), so source ADD$BYTES/MAX$BYTE lower to ADDBYTES/MAXBYTE; source unchanged.

Pascal file is a different program: Double(Integer) and AddOne(Byte). Complete65536signed16patterns for Double, all256Byte inputs for AddOne, both original asserts. VM raw I storage and native actual return word are explicitly decoded as int16 only for declaredTyI16 Double; frozen signed expected remains signed. Byte results remain unsigned. This representation matches Pascal frontend types/Book v7 §3.1 widths, not a new signed-overflow language policy.

Actual declared mains are executed where present, with real native termination semantics. Lanz/Lizp/Nanz have no main, and no artificial main is added. Pascal original main lowers to DI/HALT status0/SPeff0. PL/M module has no executable main for declarations/asserts only; assertions are metadata and each is invoked. No source trimming, replacement algorithm or host computational substitute.

Ordinary/GraceSize/GraceSpeed each perform131337calls for each of five byte sources (256unary+131072pairs+9asserts),65795for Pascal (65536Integer+256Byte+2asserts+main):2167440totalactual assembled native calls and corresponding raw/final VM. Every declared callable retains actual ABI and returns to PC7000 with SPeff2. Full64KiB RAM compares after every call; only exact observed private stack extent and declared native spill homes masked. Code/image/nonprivateRAM/canaries protected, no output allowed. Native registers reset/poisoned each invocation; VM/native reuse avoids setup overhead without reducing domains.

Final proof PASS22.366s. Full Lanz PASS0.014s, Lizp PASS2.199s, PL/M PASS0.029s, Pascal PASS0.022s. No production compiler edits/fullslowpipeline restart. Initial probe log contains test invocation name correction (PL/M separator), not a claimed compiler miscompile.

All exact sources, per-path per-mode actual ASM/BIN/MIR/allocation/costs and terminal logs tracked with manifest hashes. Root owns catalog projection; this proves these six exact bounded programs, not every frontend or arbitrary input structure. Plain native Z80 target, not CP/M runtime verification.
