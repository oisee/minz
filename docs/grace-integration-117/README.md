# Original enum match: native integration

All three original functions on all three declared enum values pass raw/final
VM and assembled Z80 in default/GraceSize/GraceSpeed (27 native calls).
Root replay PASS 0.042s. Book v7 sections16.1/16.3 define variant numbering
and match behavior; invalid tags are outside the declared-value proof.
Nine original assertions retained; actual ABI, return PC/SP and full RAM checked.

All 23 payload SHA256 values verified and copied to the share. Current generated
registry includes previous bounded saved packets directly, avoiding the old
manual projection: 47 bounded independent VM/native paths, 90 pending oracle,
194 explicit paths total. This is not whole-corpus execution proof.

U32 logic full MIR2 root check PASS16.882s. Dynamic wide-count shift is
still being repaired separately; the full goal remains active.
