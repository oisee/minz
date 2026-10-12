# C89 struct-return: согласованный tuple-контракт

Неизменённый examples/c89/c99_designated.c ранее не проходил hir.Verify: promotion менял callee RetTy на void и добавлял RetTys, но tuple call оставался ptr. После изменения scalar call type на void выявилась вторая ошибка: verifier проверял tuple как scalar и требовал первый return u8.

Promotion теперь устанавливает scalar placeholder void. Verifier распознаёт именно CallExpr внутри TupleLetStmt, проверяет аргументы/захваты и полный список типов/arity результата; вложенные scalar calls продолжают проверяться отдельно. Отрицательные тесты отвергают неверную вторую позицию, число результатов и аргументов.

Девять field wrappers исходного файла проверены по буквальным значениям именованных полей: basic10/20, reverse3/7, mixed5/15, triple1/50/100. Raw MIR VM, final MIR VM и native Z80 assertions проходят в ordinary, Grace Speed и Grace Size. Экспорт input HIR, final MIR и actual generated ASM сохранён рядом. Это девять конкретных wrappers, не доказательство произвольных C struct calls. Assert runner проверяет собранный машинный код; отдельный BIN в этом пакете не экспортирован.

Полные HIR, C89, Nanz packages PASS (30.149s/4.849s/18.440s); focused tuple/owned struct ABI boundaries PASS. Экспортный повтор PASS. Старый full VIR panic теперь устранён этим входным тестом; новый полный VIR gate ещё не запускался. Текущий immutable production pipeline b6b4e0c7 предшествует этой правке.
