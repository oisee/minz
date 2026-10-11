# Original signed source and full pipeline failure audit

Original signed_arithmetic.nanz all nine functions pass266240 independent
raw/final VM/native cases; root PASS0.884s. Actual ABI, return PC/SP and
code/literal immutability checked. Byte inputs/counts and nonzero byte
divisors exhaustive; sar16 bounded. One ordinary ContractOpt mode, not
three modes. Division-by-zero result deliberately not asserted.
Current catalog49 bounded VM/native paths,88 pending,194 explicit paths.

Frozen fullpipeline8bf75e92 terminated FAIL2865.894s: five top-level
tests failed (seven subfailures). This historical binary predates several
subsequent fixes and is retained exactly. Root reran all five on current
b0e1b456: sum and loop-cost pass, while LUT globals and mutation diagnosis
still fail. Full current replay21.243s; not a fullsuite PASS.

Mutation rejection was already correct but lost its diagnostic context
when a renamed retained function made context validation fail first.
d7c06307 wraps those revalidation errors as prior factory mutation;
focused mutation/forged-profile controls PASS0.208s. Guards unchanged.

Two genuine valid-workload refusals remain: compiler-generated readonly
LUT globals differ from the unoptimized source-global list. Exact source
LUT regeneration admission is under repair, with tamper refusals retained.
Original14 error baseline also retained: wrong native A codes, stale
Carry affects main output, VM unknown intrinsics. Semantic fix in progress.

All signed payload SHA256 checked. Error baseline and historical full
pipeline logs saved on the share. Goal remains active.

All TestMeasureProfileVariantSets tests after the diagnostic fix: root
PASS1.718s, including caller, fresh-input/mutation refusals, site-cap and
forged-count checks. No admission guard was loosened.
