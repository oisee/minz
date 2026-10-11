# PFCCO R1: incompatible neighbor choices

A two-node regression demonstrated an infeasible assignment: the eliminated
node had no compatible choice for neighbor option0 (InfCost edge), but R1
left that option at zero unary cost. Solver returned[0,0] instead of the
feasible[0,1]. R1 now marks the unsupported neighbor option InfCost.

Regression failed before fix; full MIR2 after fix passes. This addresses
one finite feasibility propagation case; all-infeasible graphs and residual
RN heuristic optimality still require explicit admission/fallback work.
Main greedy remains unchanged. No native speedup is claimed for this test.
