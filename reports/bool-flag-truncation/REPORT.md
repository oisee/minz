# Predicate truncation materialization

Independent accepted MIR reduction: compare two bytes to a ClassFlag Bool, truncate to Bool in a general byte home, and consume an unrelated live A=103 afterward. Before, physical-F placements emit invalid LD C,F/LD E,F/LD H,F/LD IYH,F (before.txt); pending predicates assigned another register copy stale raw data. F represents a predicate, not integer F bits.

OpTrunc now routes actual F/pending-predicate sources through the existing full-condition boolean materializer used by OpExt. It produces canonical0/1 for all six unsigned Eq/Ne/order conditions and preserves the unrelated live accumulator. Destination Bool width1 is explicitly handled. Ordinary nonpredicate truncations retain their previous masking path. No new implicit carry convention or emitMov F fallback is used.

The 300 native cases cover 6comparators×5boundarypairs×2flagplacements×5destinations including spill and IYH. Existing broad Bool extensions also remain part of full MIR2 verification. This is separate from the C89 short-circuit frontend fix.
