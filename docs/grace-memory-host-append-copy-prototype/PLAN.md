# Isolated append-copy experiment

Base: frozen combined selector branch0bfe99a6. Parent production is untouched.

Candidate under review: exact intern_region append loop, preserve peek(source) before poke(destination), preserve the u8 loop counter and u16 modular address arithmetic. Proposed local bases are src_buf+start and str_bump+1. Closed memory hosts access only declared regions disjoint from global storage; no arbitrary callbacks or host writes to globals. The exact loop does not assign either global, so removing repeated global reads requires this explicit invariant scope. It does not claim equivalent ordered global traffic.

First measure the smallest approved candidate against unchanged original SelfLanz30 through the public closed baseline API. Keep original/variant distinct typed HIR keys and the original source dependency SHA. Archive raw observations, native results/full resources/events, ASM/BIN, allocation/MIR/callframe/sidecar identities and independent host audit. No registration, default activation or production proof claim. Preserve strict native infrastructure refusals instead of widening permissions.
