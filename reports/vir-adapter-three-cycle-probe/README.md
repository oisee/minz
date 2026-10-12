# Adapter three-cycle native counterexample

Existing resolveParallelMoves loses values in A→B→C→A. Independent native test initializes A=x, B=y, C=3*x+7*y modulo256 and verifies all three outputs, unchanged DE/HL/IX/IY and exactPC/SP. Baseline fails x0,y1 (C7). Candidate with alias-aware dependencies and a saved free scratch pair passes65536 x/y combinations; this is not all256³ triples.

Candidate NOT admitted. Its no-free-scratch fallback remains invalid for byte-only cycles (PUSH byte register is not encodable), and mixed pair/half cycles need explicit tests. Original compiler restored; executable reproduction retained outside passing suite. Next step is a total cycle resolver with stack snapshots or checked scratch selection, followed by all permutations and mixed-width overlaps. This research issue does not invalidate independent production table measurements.
