# Combined source review at 5cc02d44

Verified every SHA in the ten-path combined-source inventory against the actual owner worktree. The four imported backend files (call lowering, caller-home helper, TSMC scan and scanner test) are byte-identical to the independently reviewed 272814d1 version. This reviewer ran only file/hash comparison, not target CPU.

The accumulator pass removes only the recursive no-free-scratch refusal, retaining the named memory spill fallback, module-aware fresh register allocation, full scratch lifetime checks and operand renaming safeguards. OpCallCond is now recognized as an accumulator clobber, consistent with its actual genCall lowering. The existing no-free-byte fixture requires both recursive and nonrecursive forms to receive a spill rather than silently omitting the save.

No remaining blocking source issue was found for the authorized corrected legacy15 retry. Compile-only showcase acceptance and MIR tests do not independently prove recursive output correctness; backend forced recursion execution and any actual source recursive caller remain separate evidence. Unsupported native-host EX(SP),HL authority is unchanged. Preserve the original invalid-ASM, first-byte output and recursive-refusal histories.
