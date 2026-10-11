# Exact original hashmap PC/function attribution

Measured36calls **138,440 T /1,013 B**, machine SHA `aefde17ab12e824e4d790cc7147f0974329faf5dded51f032643eb316d56ba86`. Overlay replay reproduces savedASM, machinebinary, completeinvocations/results/state/allglobalbytecounts, bootstrap identities/costs and executedsteps. Instrumentation records instruction-startPC/cycles only outsidebootstrap; physicalfunctionexclusive attribution contains callee cost separately (no doublecount).

| Physical function | T | Share |
|---|---:|---:|
| map_put | 74633 | 53.91% |
| rd16 | 33390 | 24.12% |
| map_get | 14426 | 10.42% |
| wr16 | 12464 | 9.00% |
| map_clear | 3054 | 2.21% |
| map_has | 473 | 0.34% |

Standalone hash16 has0T: calls were inlined into put/get; its machinecode still exists. rd16 executes159times at210T each; wr16 executes38times at328T each. putprobe139/getprobe30 iterations.

| Exclusive physical block | T |
|---|---:|
| rd16 whole | 33390 |
| wr16 whole | 12464 |
| hash16 standalone | 0 |
| clear count/setup | 106 |
| clear loop guard | 420 |
| clear byte store/address/bump | 2528 |
| put hash/setup | 1806 |
| put probe count guard | 3786 |
| put occupancy address/read/test | 22150 |
| put insert caller+count | 7344 |
| put key-read call/compare | 31208 |
| put overwrite caller | 128 |
| put slot/probe bump | 8211 |
| get hash/setup | 1170 |
| get probe count guard | 912 |
| get occupancy address/read/test | 5030 |
| get empty miss return | 40 |
| get key-read call/compare | 5270 |
| get value tailcall | 240 |
| get slot/probe bump | 1764 |
| has caller | 473 |

Concrete next experiment: exact typed rd16 helper currently recomputes two array addresses and zeroextends/recombines bytes. An opt-in HIR helper lowering can keep one address or recognize a littleendian word load while preserving first low-byte read then high-byte read, modularu16address and partial firsttrap. Localoffsetbase=idx*2 is pure; original idx*2/+1 wrap semantics must remain exact. Full caller has159rd16calls and 24.12% exclusivecost, versus map_clear entire2.21%; prioritize independent rd16 address-reuse proof over zero-fill.

Profitability unknown: a widened-wordLoad can have different partialtrap/accessobserver semantics, so simplest boundedcandidate reuses address then performs TWO explicitorderedu8LoadExpr nodes and combines toU16. This needs sourceexact/functionmetadata/nowrap-or-wrapping argument proof and actualfull36caller raw+Z80oracle. Do not replace sourcewithASM or fakehelperpurity, do not infer newgain from24.12% ceiling. Probe loops still54% callerexclusive and generate repeatedmap_used slotaddress with save/restorepair traffic; a later slotmask/u8induction proof requires modulo16 semantics and full-table/early-exitorderedaccess, not genericpointerwalk. No productionrule/source/workload changes were made by this attribution task.
