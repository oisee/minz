# Полный MIR2 финального literal-call emitter

Checkpointbb0fdc02: полный MIR2 `go test ./pkg/mir2 -count=1 -timeout=10m` завершился PASS, exit0,88,024с. Включает финальные byte-only и exactcurrentCALL guards literal rematerialization; это закрывает отличие от donorfullMIR, начатого до последнихguards.

HIR ASM input fix появился позже и имеет отдельные HIR/native runtime проверки277. Этот MIR2 gate не подменяет полный pipeline/CLI; retainedpipeline1f2365be продолжает работу со старым parserreference.
