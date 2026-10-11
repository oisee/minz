# Full dynamic shift count and original impl methods

Root integrated full shift-count correction as d0fe5ab0. Dynamic counts
are inspected across all bytes before scratch-counter writes and saturated
at result width; logical shifts zero, arithmetic right shifts preserve sign.
This preserves existing raw/final VM semantics; Book v7 does not explicitly
define out-of-width counts. Root 134064 ordinary source cases plus original
SHA split helpers PASS8.525s; full MIR2 including2184 forced shift cases
PASS19.595s. No constant non-HL shift claim; separate audit continues.

Original50_impl_showcase: eight source functions and four assertions unchanged.
4612 calls in each mode (13836 native total), raw/final VM/native independent
area/perimeter models, receiver fields, actual ABI, local frames, full RAM/code
and PC/SP guards. Root PASS3.206s. Rect widths exhaustive but heights bounded;
Circle radii exhaustive. No whole OOP or arbitrary method call claim.

28 impl and57 shift packet payload SHA256 values verified and copied to the
share. Generated catalog48 bounded VM/native paths,89 pending,194 explicit
paths. Source assertion presence never substituted for execution proof.

Original14 @error/@propagate audit found native wrong error-code A and
stale carry after optimization; VM reports unknown intrinsics. That gap is
under investigation separately, not counted as successful corpus evidence.
Goal remains active.
