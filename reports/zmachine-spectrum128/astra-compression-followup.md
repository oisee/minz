# Astra: compression after native Zork startup

2026-10-13. Read-only gpt-6-astra review requested by Alice. Two attempts hit
rate limits; the final followup completed. Budget below is arithmetic from the
reported build, not an independent rebuild.

Recommendation: finish raw interactive story first; compress immutable blocks
as a later option; keep writable dynamic memory raw.

- Six banks: 98304 - 86928 = 11376 bytes free.
- Fixed RAM after screen: 25856 - 24037 = 1819 bytes for external stack and console.
- Current resident includes a 1024-byte capture buffer; replacing it with a real
  console requires a fresh size measurement. Two 1-KB cache slots do not fit now.
- Independent immutable blocks with bounded LZSS or ZX0 decoding are feasible
  candidates. No decoder size/speed measurement exists yet. Require raw fallback,
  an exact index, bank-padding accounting and bounded backward references.
- The zlib redundancy probe yields 65797 bytes including a provisional index
  for 1-KB blocks, just over four banks. 2-KB blocks yield 62148 bytes before bank
  padding, but two output cache slots need 4 KB fixed RAM. These are not predicted
  LZSS/ZX0 sizes.
- Compressing dirty dynamic pages adds recompression on eviction, possible growth
  and free-space management; worst-case raw capacity must still be available.
  Keep the 11288-byte writable region raw for the first runnable version.
- Save compression is separate: streamed saves, then optionally XOR with original
  dynamic bytes and zero-run encoding. Restart/delta saves require the original
  image or reloading. Do not reintroduce a permanent full snapshot.

Next experiment: execute a short raw interactive transcript, measure native stack
high-water use and log logical reads. Replay this trace for 512/1024/2048-byte
blocks and one/two cache slots. Then test one bounded LZSS decoder on Z80 against
original bytes, block/bank boundaries and restored paging. Decide using complete
resident/archive size and cache-miss cost.
