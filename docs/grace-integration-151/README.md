# Интеграция151: tables/PFCCO/Grace current refresh

Root82da91c7 повторно проверен без solver: ordinary checked-local add5 all256input **8→6moduleB,56→49T fixture**, result/SP; tamperfallback и profile refusal **PASS0,076 с** [лог](no-solver.txt). Обычная tableapplication не вызываетZ3.

Offlinegenerated weightedcandidate × off/greedy/PBQP ×Graceoff/on ×tableoff/on,12combinations: **fullBIN44→20B**, helper256inputs46592→22016T, caller60928→36352T (средние238→142T), actualCALLmoves/saves учтены. **PASS5,321с** [лог](combined.txt). ExplicitVIR_TEST_Z3=/home/alice/dev/minz-artifacts/grace-tools/z3-python/bin/z3 используется тольков этомresearchtest generation. Исходный запускбезenvбылSKIP, не выдаётсязаPASSmatrix. Нового общегоfamilyoptimizationclaim или globaloptimality здесь нет.

Structcallerownedbuffers/knowncallbacksignature исправления продолжаются. Новые declaredcallcontracts должны пройти actualindirect/recursive/valueownership проверкидоcurrentfullPASS. Oldpipelinea7954042live, fullCLI/core4587e753PASS. Общая цель открыта.
