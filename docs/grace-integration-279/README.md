# Полный HIR после ASM input binding

Source841ca5e0: `go test ./pkg/hir -count=1 -timeout=10m` завершился PASS, exit0,33,044с. Включает variable-first ASM inputs, explicit register aliases и unknown-name refusal. Во время gate HIR source не менялся.

Это полный HIR package; MIR2 имеет отдельный fullPASS88,024с наbb0fdc02. Full pipeline1f2365be продолжает работу со старым parserreference; три knownnumerical failures сохранены и сверены отдельным rootrepeat267.

При проверке prospective literal теста root обнаружил required MINZ_CALL_LITERAL_TOKENIZER_EXPORT: совместный runtimegate276 без него не выполнял этот один тест. Donor prospective proof сохранён, но текущий rootexported replay запущен отдельно. Не считать skipped тест текущим runtime proof.
