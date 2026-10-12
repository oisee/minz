# Фронтенды после Z80 literal-overwrite

На checkpoint95fb99ec повторены полные HIR/Nanz/Lanz/Lizp/PLM/Pascal/C89/Frill/VIR/assembler/z80validate/import-enrt пакеты: все 12 PASS. MIR2 уже отдельно прошёл после той же production правки5183e540, лог в `literal-overwrite-native`. Новых production изменений между этими checkpoint нет.

Пять отдельных independent native gates прошли за24,542 s: C89 controls (710 случаев на каждый из ordinary/GraceSize/GraceSpeed), C89 expression effects (328704 на режим), PLM module entry, Lizp crypto bounded byte models и Pascal records PointSum/Manhattan modulo256. Проверки используют собственные ожидания, raw/final VM, assembled Z80 и соответствующие state/trace/SP границы. Детали точного объёма каждого family находятся в native.txt и источниках тестов; этот набор не объявляется исполнением всего корпуса.

Команды: `go test ./pkg/hir ./pkg/nanz ./pkg/lanz ./pkg/lizp ./pkg/plm ./pkg/pascal ./pkg/c89 ./pkg/frill ./pkg/vir ./pkg/z80asm ./pkg/z80validate ./cmd/import-enrt -count=1 -timeout=15m` с offline VIR_TEST_Z3; `go test ./pkg/pipeline -run '^(TestC89ControlEffectsNative|TestC89ExpressionEffectsNative|TestPLMModuleEntryNative|TestShowcasePascalRecordsNativeModels|TestShowcaseLizpCryptoNativeByteModels)$' -count=1 -timeout=15m -v`.

Полный CLI3ebed86e пока работает; полный pipeline5d90b7aa работает на более раннем backend. Их итог не подменяется PASS этого пакета.
