# Bool→u24/u32 native widening

Base a5c1eaf0. Explicit cast input==0→wide gave raw/finalVM1 but native0x803b/0x803f. genExt only recognized bool for8/16bit destinations, leaving wide TODO and stale pair bytes. [Original source FAIL/ASM](before.txt).

Bool handling now materializes a flag predicate once and widens its canonical byte through the existing overlapping-safe unsigned snapshot emitter. Materialized bool also follows that byte path. Live source predicate remains available after an intervening flag writer; A/F preservation and exact destination byte writes retain existing emitter contracts. This is a correctness fix, no speed claim.

Independent source model uses Bookv7 §§2.9/3 bool0/1 explicit conversion. Two widths ×three modes ×all256inputs =1536cases, raw/finalVM/native. Result and repeated predicate after input+1 must match; all64KiBRAM except declared output bytes, typedspill ranges and boundedactualstack window remains unchanged, PC/SP return checked. Existing narrow bool extension tests also pass. [After log](after.txt). Exact six source/ASM/BIN/MIR packets retained in after/. RAM stack bound is64bytes rather than an instruction-traced per-write proof; no arbitraryallocation/fullCPU preservation claim.
