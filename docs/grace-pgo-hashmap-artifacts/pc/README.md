# Read-only hash map exact PC evidence

Compiler/reference worktree ef270333, measured pack /tmp/minz-hashmap-baseline-evidence. This pack overlays ONLY pipeline/profile_workload.go with instruction-startPC/cycle counters; all current production guards are retained. Instrumented host/tool executableidentity differs deliberately, but emitted targetASM/binary and completeactualtiming/state/traffic are compared exactly to uninstrumented saved evidence before success. No production or source files are edited.

```sh
GOCACHE=/tmp/minz-grace-gocache bash reproduce.sh /tmp/minz-hashmap-program-baseline /tmp/minz-hashmap-baseline-evidence
python3 attribute.py reproduced-pc-records.json /tmp/minz-hashmap-baseline-evidence/baseline.a80 reproduced-function-costs.json
```

Use Go1.26.8. reproduce.sh uses adjustable repository/baseline arguments and packrelativehelper/overlay; outputJSON can be deterministicallygzipped after run. attribute.py accepts rawJSON or gzip. report.md explains exclusive ranges/function/blockcosts and one proposed independentoptimizationexperiment. Callbootstrapcosts excluded; generatedhelpers/trampolines inside physicalfunctionrange counted to thatrange, no caller-inclusive functionclaim.

Rawrecords gzip mtime0, emptyfilename. Allpackfiles except manifest bound by SHA. Exactsum138440T and samebinary aefde17ab12e824e4d790cc7147f0974329faf5dded51f032643eb316d56ba86.
