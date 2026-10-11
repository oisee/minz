# Current core и source delta после frontend исправлений

Полный HIR/MIR2/Nanz/VIR запущен на40041bd4: GOCACHE=/tmp/minz-grace-gocache Go1.26.8, go test ./pkg/hir ./pkg/mir2 ./pkg/nanz ./pkg/vir -count=1. Log grace-checkpoints/core-40041bd4.txt, handle96653; terminal outcome pending. Это после C89/PLMfix и до возможногоzeroidentity переноса.

На шаре сохранён minzc delta08be2950→40041bd4: все10 изменённых code/test файлов SHA verified, включая C89 switch/PLM module literal regression и оригинальную evidence тестовую инфраструктуру. Exactbase snapshot имеет4524files и собственныйmanifest; delta не выдается за ещё один fullsnapshot.

CLI31d6abd0 и pipeline29df08bb подтверждены live по своим handles; их epochs сохраняются отдельно. Общаяцель incomplete: unresolvedLizp policy/runtime/corpus ограничения, usefulLIRidentity transfer и свежиеterminal gates.

Terminal core40041bd4: HIR28,651с/MIR2 70,579с/VIR2,470с PASS; NanzFAIL16,176с из-за stale evidenceSHA original_strings_pointer_plm_native_test послеverifiedPLMfollowup. ПолныйcorePASS не заявляется. Catalogrefresh025fe495 проверенfocusedPASS1,885с; полныйNanz повтор отдельноlive. Log сохранёнв212/core-before-catalog-refresh.txt.
