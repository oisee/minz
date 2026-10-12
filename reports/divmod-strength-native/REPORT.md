# Original divmod10 and mul8 strength-reduction corpus

Exact unchanged sources `examples/tests/divmod10_test.nanz` and `mul8_strength_reduction.nanz`; source hashes/domain models frozen before execution in149e1075. Book v7 §3.1 defines u8 width and wrap; unsigned quotient/remainder use independent host n/10 and n%10, multiplication uses x*k modulo256, board_idx uses (10*y+x) modulo256. No compiler output is used as expected-value oracle.

Every original callable retained: div10/mod10/main;14constant multipliers0,1,2,3,4,5,6,7,8,9,10,12,15,16 plus board_idx/main. All256u8inputs for each unary function and all65536orderedu8(x,y)pairs for board_idx. All20original source assertions explicitly executed as native calls and raw/final VM, with source assertion bodies unchanged. Division module has no asserts. Each original main is separately executed; divmod main contains pure unused q/r locals, so legal DCE/comptime folding does not imply that main proves those return values—independent direct function domain covers them.

Three ordinary/Grace Size/Grace Speed modes:513divmod calls and69141strength calls per mode; total208962actual assembled native invocations and matching raw/final VM. Every callable uses actual parameter and return ABI, returns to sentinelPC7000 withSPeff2. Main production termination is XOR A/DI/HALT status0 withSPeff0, exactly source/native main lowering. This is plain Z80 execution, not CP/M host runtime substitution.

Full64KiB RAM compared on every call; VM heap must remain unchanged. Native register inputs poisoned/reset each call, code/image and all nonprivate RAM checked; only exact observed stack extent and declared spill homes masked. Unexpected output denied. Shared reusable pure instances avoid allocation overhead without changing the workload or arithmetic oracle.

Terminal proof PASS2.236s. No production compiler changes; historical comments about GPU-optimal sequences remain unchanged and do not imply a particular table implementation. Full actual ASM/BIN/MIR/allocation/cost evidence retained for each module/mode. Root owns corpus status/catalog projection. No full slow pipeline or broad suite restarted for this test-only addition.

All packet files are tracked, including otherwise ignored ASM/BIN/MIR, and verified against manifest.json before share copy. Source freeze remains unchanged.
