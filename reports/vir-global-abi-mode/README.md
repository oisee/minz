# Global parameter ABI participates in mode selection

Mode selection previously recognized caller constraints only through FuncParamLocs, while SolveCFGFull accepted both ParamLocs and FuncParamLocs. It could therefore treat an explicitly constrained global caller ABI as unconstrained. Combine both maps for selection/adapters, preserving function-specific precedence. No change to the source language or default ABI.

Permanent CodegenFunc regression passes incoming word in DE (global ParamLocs), independently verifies HL=2*x modulo65536 for all65536 inputs, exactPC/SP. The observed constrained solve is unsat and standalone+adapter succeeds with one adapter: this covers the actual mode transition, not merely a direct helper. Broad native assertions/adapter/island/index family PASS7.234s. Core packages still running separately at immutable97266570; full VIR ced33bd1 and production cbf01110 also remain earlier running checkpoints.
