# Historical proof correction

A compile-only diagnostic on exact9b1637ef added a print at the whole-set infeasible-assignment fallback branch. Historical PBQP printed `DEBUG FALLBACK`, then emitted1184B with the original lowered pointer/index/index conventions. Current typed input selects a feasible pair/index convention instead. Thus the historical190T/−9B binary gain is real, but its attribution to a successful PBQP convention assignment was incorrect. It came from the fallback retaining the original lowered ABI.

The underlying literal-width correction1f87d7b7 removed narrow u8/general producers for u16 call parameters. This changed graph feasibility and costs. The current baseline did not absorb the190T split benefit; currentboth are220T. The old ABI is still expressible and emits exacthistoricalsplit instructions with typed arguments.

A second structural issue is visible directly in candidateParamOnlyChoices: pointer/index/index is pruned because both soft ClassIndex parameters have DE as their unique zero-cost home. They are not hard ABI pins: allocation can put the second intoBC (as the historicalfallback does). Treating preferred zero-cost equality as an infeasible physical contract forbids a valid candidate. A blanket relaxation also changes greedy and is not proposed as an unreviewed fix. Correct target-aware feasible placement costs need to replace the hard-conflict proxy.

No native emulator was run for these diagnostics. Existing failednativehistory andoldproof are retained. Strict gain assertion remains.
