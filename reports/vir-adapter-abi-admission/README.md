# Adapter entry ABI admission

Adapter emission validates every used parameter's caller/standalone locations before comparing them: index bounds, allocatable location, declared width and known caller mapping. Invalid SP/F/negative or word→byte locations are errors, not skipped transfers. A standalone linear solve with caller constraints but no standalone entry map is no longer returned under the caller ABI; it remains a rejected alternative so subsequent solver/fallback handling can proceed.

Negative ABI tests and full current native asserts, adapter permutations/copies, word-entry domain, islands and indexed-call family PASS6.391s. No claim that arbitrary standalone/CFG alternatives are now supported. Previous immutable full VIR ced33bd1 and production cbf01110 processes still run, excluding these later changes.
