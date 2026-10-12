# Свежий полный core905b33e2

Все13packages PASS: HIR25,220с,MIR2 83,094с,Nanz18,786с,Lanz0,022с,Lizp2,913с,PLM0,070с,Pascal0,047с,C89 5,699с,Frill0,185с,VIR2,511с,Z80asm0,150с,Z80validate0,020с,importer2,028с. Команда go test ./pkg/hir ./pkg/mir2 ./pkg/nanz ./pkg/lanz ./pkg/lizp ./pkg/plm ./pkg/pascal ./pkg/c89 ./pkg/frill ./pkg/vir ./pkg/z80asm ./pkg/z80validate ./cmd/import-enrt -count=1 изminzc, Go1.26.8; offlineVIR solver задан черезVIR_TEST_Z3.

Этот core включает новые readmissionAPI/CLI, исправление obsoleteZXoracle иTetrisruntimeexpectation. Отдельный rawJSONbudget9d041c80 появился позже и имеет собственный fullimporter gate. Длительный pipeline dd100284 ещё работает на своём точном бинарнике и содержит исправленные позже parserreference failures. Core не заменяет fullpipeline или аппаратную проверку. Общая цель активна.

Отдельный parser/hashmap frozenheldout replay9d041c80 завершился PASS; точные результаты сохранены в heldout-9d041c80.txt. Это повтор заранее сохранённых входов, не новые heldout.
