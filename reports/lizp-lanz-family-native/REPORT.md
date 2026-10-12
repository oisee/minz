# Four original Lizp/Lanz sources: three behavior proofs and one return-policy gap

Allfour exact sources frozen before execution in1bbb1fd3; unchanged source bytes included. Sources: examples/lizp/cond_test.lizp, factorial.lizp, hello.lizp and reports/showcase-src/2026-03-15/ex29_lanz.lanz. No source rewriting, external computational substitution or compiler production changes.

## Three proven source programs

factorial.lizp: complete256Byte inputs, iterative independent host product1..n modulo65536; original main calls factorial5 and writes global result little-endian120. Despite file title there is no recursive factorial: dotimes desugars to Lanz `(for i 0 n)` and HIR ForRange ascending0..n-1, so body multiplies1..n. Book v7 chapter6 Nanz `range` countdown is a different iterator construct and is not applied to this ForRange. Original all callable bodies retained.

hello.lizp: original void main simply returns, emits no hello output and touches no program RAM. File name is not an output oracle.

ex29_lanz.lanz: complete256double inputs modulo256; all65536orderedByte abs_diff pairs; sum_to inputs0..254 independent triangular sum modulo256. Original source uses explicit while, not range. For n255, i is Byte, advances255→0, and unsigned i<=255 stays true: defined source nontermination. Exact n255 is included as bounded negative execution: raw/final VM gas limit and native10000op budget with no return/HALT and stableSP. Not silently omitted or given triangular terminating expectation.

Ordinary/GraceSize/GraceSpeed:257factorialcalls,1hellomain,66047ex29calls per mode=198915native completed invocations plus matching raw/final VM. Additional nontermination probes and cond diagnostics are separate. All original callable parameters/returns use actual ABI. Completed calls require exactPC7000/SPeff2; native originalmain may validlyRET orHALT according to emitted form. Main declared global effects are independently normalized to VM/native symbol addresses: result=120 only. Full64KiB memory/code/nonprivateRAM compares every completedcall, only exact native observed private stack extent and declared spills masked. No output allowed, source images and unused globals protected.

## cond_test missing-result observation, not a behavior success

Declared classify(n:u8)->u8 ends with bare cond and no explicit return. Raw/final MIR returns void; classify42 native leaves staleA42. Expected categorical2 is not produced. Exact before-cond-return log and full source/MIR/ASM preserved; diagnostic regression records actual missing-result observation explicitly labeled NOTbehaviorPASS.

Authority audit: Bookv7 §21.2 describes Lizp desugaring/cond but does not specify implicit function last-value returns. Eight_Languages_One_Binary.md:367 examples use explicit return. lizp.go:166 desugarDefun emits statements, and TestCompileCond only verifies compilation of a bare-cond function, not execution. Lisp conventions alone do not justify implementing a new language rule or rewriting original source. Coordinator agreed retain unresolved source-language gap. cond_test must remain uncovered/diagnosed in behavioral catalog, not be counted as a fourth successful source.

The diagnostic raw/final/native proof uses complete original function and source module, with no manufactured corrected derivative. Once implicit-return policy or explicit-source requirement is formally chosen, its observation test should be replaced by the chosen semantics and original-source provenance revisited. This packet does not make a compiler miscompile claim for undefined/missing return policy.

Final focused proof+diagnostic PASS6.056s (test success includes the explicitly failing-behavior diagnostic). Source names/model/frozen scope distinguish these claims; none is a full general Lizp or Lanz guarantee. Plain Z80 execution, no CP/M OS claim. All artifact files, including ignored ASM/BIN/MIR, tracked and hash-verified; root owns catalog projection. No broad/full pipeline restart.
