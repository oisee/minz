# Adapter copies preserve other parameters

Dedicated adapter copies use PUSH/POP for pair→pair instead of EX DE,HL, which swaps and destroys the source. Prefix-conflicting byte transfers preserve AF in both directions (H/L↔IX/IY halves and cross-index halves). Cycle snapshots use these copies; restoring a word destination now pops the complete pair instead of truncating through a scratch byte.

Native tests cover H→IXH,IXH→L,IYL→IXH,HL→DE,65536 inputs each; exact full register state incl AF and PC/SP compared against independent copy expectations. Existing all-permutation byte tests, word cycles and VM/native call/index assertions also PASS6.748s. These four domains are not all possible mixed-width adapter maps. No-free-scratch byte cycles still reject explicitly; dedicated stack-slot lowering remains open.
