# Legacy compiler accumulator diagnosis

The module-register repair fixes four assembler failures. The first actual legacy-route 15-case run then found four SString output failures: only the first byte was printed. Eleven cases passed. The complete failing evidence and exact a623 executable are retained here; this is not a successful legacy output certificate.

The original pass selected D while DE held a live pointer and a later definition overwrote the saved count. The separate lifetime/spill repair remains isolated pending activation-safe recursive spill preservation and reviewed native validation. No further native execution has been performed.

Steps-only MIR/allocation metadata is unavailable through CompileHIR; saved legacy allocation.json is null and final.mir2 empty. Exact assembled wrapper, binary, output, expected bytes and final state are available.
