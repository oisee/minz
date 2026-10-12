# PFCCO arena investigation checkpoint

Base: 1abea5b1. No emulator runs in this checkpoint. Historical source/compiler probe: 9b1637ef. Original native negative retained unchanged.

Current greedy and PBQP both assemble to 1130 bytes and retain the old greedy arena_split ABI (a=HL,start=IX,size=DE). Historical PBQP selected a=HL,start=DE,size=BC, measured 190 T versus greedy 220 T. The smaller whole module therefore does not establish absorption of that function gain.

The isolated compile-only diagnostic temporarily reverted only typed integer literal call lowering from 1f87d7b7, then restored it. It restored PFCCO index/index choices and 1115→1106 bytes. This is causal evidence, not a proposed fix: correct typed literal lowering must remain. The new typed u16/index call-only constants change PFCCO edge costs. Investigate producer placement jointly with candidate ABI and materialize that placement before allocation; never claim zero adapter cost without an emitter/placement change.

Strict native benefit assertion is unchanged. Current semantics passed in the supplied negative log. No new native proof yet.
