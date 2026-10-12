# Rejected zero-cost same-pair VIR move probe

Model lacks a zero-cost16-bit copy whose source/destination are the same pair. An experimental same_pair OpMove pattern with TiedDstSrc=true reduced double_dec from6 to3 bytes and passed both the existing DEC HL quality test and all65536 native word inputs. Direct physical move lookup was adjusted to reject a tied pattern for unequal locations.

Broader assert/module/solver/table run nevertheless failed IndexedArrayBaseRegression: scenario returned0 instead16 after solver timeout and PBQP fallback. Reverting all three code modifications restored the original unchanged assertion to PASS5.499s, through the same timeout/fallback path. The new pattern is NOT retained in compiler code. No claim yet about the precise cause in fallback or pattern consumers; this difference is a concrete next investigation, not permission to weaken assertions.

Current checked source remains cc8023a2 semantics. Both immutable full runs are live: production pipeline b6b4e0c7 and research VIR c86bfcd5. No restart performed.
