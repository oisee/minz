# Word multiply and fresh frozen replay

Root69566578 bounded checked-local word multiply lowering integrated. Explicit ADD/MOVE chain and BC saves preserve original source/MIR/ABI context; assignment solved by retainedZ3 and independently checked. This is new local lowering plus register allocation, not use of historical multiply recipes or a default backend change.

Root actual originalmul16_gpu_test nativeall65536 perhelper/rawcandidate and real assemblyCALL wrapper PASS1.681s. Original wholemodule112→63B; ×7 1085→104T, ×1001085→148T, ×2551115→214T. ×3/10 raw candidates are worse and baseline is retained. CALL wrapper preservesx acrosscall:1144→163T, wholemodule+wrapper119→70B. The separate caller sourcefixture is inlined, so only assemblywrapper supplies actualCALL proof. FullVIR PASS2.356s. Initial opt-in run skippednative before solverpath was supplied; that log is retained and is not counted asnativePASS. Solver /home/alice/dev/minz-artifacts/grace-tools/z3-python/bin/z3. Exact20archivepayloads SHAverified/shared.

Current frozen parser/HashMap replay rootPASS26.008s. Parserheldout2141572→2093170T,2364→2344B unchangedfrompriorcontext. HashMapSpeed/Sizeheldout258962→237948T,990→970B. Same trainingBIN on heldout, no new heldoutsearch/inputprospective claim. HistoricalHashMap figures remain untouched. All107currentreplaypayloads SHAverified and copied to compiler-reports/current-parser-hashmap; independent Python audits retained.

Sitesv65 native deployment succeeded appgdep_6acbe8f502d481918d5ef849945b21b9: https://minz-grace-hir-mir-report.grittysnipe3.chatgpt.site/current-state.html . Pushedsitecommit6abd7d4d4268e08c681f7be759f221fac3cbe128, compilerpublication9778a214. It includes currentfrozenreplay but predates newwordmultiply integration. Wholeofflinecopy/SHAverification is a separate live operation at this checkpoint; do not claim terminalcopy yet.

Second standalonebackenddraftPR https://github.com/oisee/minz/pull/72 head a3c2def3 fixes unsigned division B/Cworkspace againstmain4743cb7c.130560nonzeronativepairs+4096liveDtests pass; nativezero-divisortrap outsidepatch. Main not merged. Big frozen8bf fullpipeline remains live; overallgoalactive.
