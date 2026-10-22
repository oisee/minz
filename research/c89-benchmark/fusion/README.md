# Frozen iterator-fusion suite

This suite measures the production compiler without modifying it. Every case has
C89 plain loops in `cases/`, a Nanz consumer in `nanz-idiomatic/`, a complete
hand Z80 module in `hand-asm/`, and explicit input/output samples in
`workloads/<case>.json`. Each language manifest pins both source bytes and the
same workload bytes with SHA256. `run.py --suite fusion` passes that workload to
`measure.go -workload`; `check_host.py` replays the saved samples with GCC.

The common contract is disjoint byte buffers, `0 <= n <= 256`, no alias between
inputs and outputs, no interrupts, and little-endian 16-bit control/result
fields. Read-only globals are checked after every call. All arithmetic results
are unsigned; byte transforms wrap modulo 256 and word sums modulo 65536.
Hand code exploits the declared upper bound with an initial **16-bit empty
check**, then `DJNZ` with B=0 representing exactly 256 elements. It does not
specialize by dataset or assume nonzero input. Count/loop semantics are part of
the workload contract, not an optimization of arbitrary 16-bit counts.

| Case | Operation | Nanz spelling / adaptation |
|---|---|---|
| fusion_map_filter_sum | y=u8(x*2); sum y where 10<y<200 | map → filter → filter → fold(u16(0), add) |
| fusion_filter_count | count x==k | filter → forEach with captured u16 count |
| fusion_map_store | target[i]=source[i] xor 255; result=n | map → forEach with captured destination pointer |
| fusion_window_sum | sum first limit elements after skipping four, clamped to n | clamped pointer + count → fold |
| fusion_fold_min | minimum byte, empty=255 | fold(u8(255), smaller) |
| fusion_fold_max | maximum byte, empty=0 | fold(u8(0), larger) |
| fusion_dot_product | sum source[i]*other[i] mod 65536 | forEach, captured second pointer and u16 sum |
| fusion_filter_map_sum | sum u8(x xor 255) for x>=128 | filter → map → fold(u16(0), add) |

The [G0 audit](../../../docs/Nanz_Iteration_Audit.md) takes precedence over the
Nanz book's historical iterator claims. `iter()`, `skip`, `take`, and `enumerate`
are G0-5 unsupported methods. There is no implemented native `zip` method or
`count` terminal in the audited memory-chain interface. This suite uses the
closest supported consumers above. It does not claim zip/skip/take fusion or
predicate-driven early exit. `any`/`all`/`find` are absent from that interface,
so filter→map→fold consumes all n elements. The optional enumerate case is
omitted due to G0-5. Range countdown sources are deliberately avoided, including
G0-9 at 256; memory chains use u16 lengths and preserve forward order.

Min/max use named conditional callbacks; current lowering calls these helpers
per element rather than inlining their control flow. The scalar fold result is
bound to a typed local before storing `result`. Direct assignment / nested-cast
fold spellings failed initial probes on this revision; those failures and their
artifact paths are recorded in the report. No compiler workaround or inline
assembly is hidden in the Nanz sources.

Each case includes lengths 0,1,2,3,4,5,16,127,128,254,255,256 over five datasets:
repeated threshold/overflow boundary bytes, all 6, all 0, all 255, and a byte
ramp. Filter/count additionally uses k=0,6,255; window uses limit=0,7,256.
Thus every filtered case includes all-pass and none-pass inputs. Each case has
60 calls, except count/window with 180, totalling 720 distinct case samples.
The store case checks all 256 destination bytes, including its untouched tail.
The dot case includes 255*255 products and accumulated word wraparound.

JSON `sizes` declares global region lengths. Each `samples` entry contains a
unique `id`, base64 full-region `inputs`, scalar `expected`, and base64 full-region
`after`. Expectations were computed independently with byte/word masks, min/max,
clamped slices and scalar zip products, before measurement; GCC validates them
against the frozen C. Changing a workload or fixture requires an explicit
manifest update and a fresh matrix plus host replay. Do not silently refreeze
a failing expectation or select a faster dataset after observing results.
