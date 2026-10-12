# Production table replay with exact harness completion

Saved-table readmission needs no Z3: seven test cases (one standalone plus six off/greedy/PBQP×Grace configurations) independently execute every256 byte caller inputs. Baseline and selected module remain44→20B, total caller60928→36352T (238→142 per call). Complete source, readmitted checked table, baseline/selected ASM/BIN and per-case measurements retained under emitted/. This repeats a known gain, not new heldout or new application evidence.

The shared native helper now also requires emulator halted at the exact terminal HALT in the caller bootstrap, with restoredSP, correct return value and unchanged compiled code. Previously another clean emulator exit could satisfy only value/SP. This keeps existing timing setup unchanged; not universal arbitrary-control-flow or all-memory proof.

Final no-Z3 production replay PASS3.239s. Offline real generation and already-optimal leaf checks with actualZ3 PASS1.612s. Other consumers PASS6.264s but four environment-dependent tests skipped in that initial run (retained). Actual indexed inputs then ran: three indexed consumers PASS; legacy initially failed because its bounded sample file was missing from the chosen export directory. Repeated legacy with the retained original sample PASS1.018s. These environment failures/skips are disclosed and not compiler miscompiles. Indexed output includes exact source/constant remap and native triple/weighted coverage. Default-size clone and historical mappings do not imply every historical entry helps.

Full production cbf01110 and full research7bfe4476 continue as separate older immutable checkpoints. No current fullsuite PASS claimed. Speed remains without byte penalty; this replay uses Size configurations.
