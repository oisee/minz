# Live-across-call TSMC exclusion review

Reviewed cff26ed7 without native execution. The backward order is correct: remove Dst and ExtraRets definitions from the live-after set, record remaining values at a call, then add instruction uses. This excludes newly defined results and does not mistake a call-only argument for a live-across value. CFG live-out and every terminator use seed the scan, covering successors, joins and loop-carried values.

One concrete blocker was found and sent to the backend: OpCallCond invokes the same genCall path but is missing from the new call classification. A value used after a conditional recursive/reentrant call can remain TSMC-eligible; callerSaveSpillHomes deliberately skips its patched storage, so neither path protects it. Include OpCallCond and a focused eligibility regression with the value live after that instruction. No target replay is needed to identify this omission.

Once corrected, the new exclusion makes live-across-call candidates ordinary data-backed homes and the existing stack snapshots can preserve mutual/indirect activation values without globally disabling TSMC. This statement covers structured compiler-generated calls, not an inline-assembly template that secretly invokes a function.
