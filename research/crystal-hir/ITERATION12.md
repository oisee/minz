# Итерация 12: точный frame liveness и итоговая веха

Общий HIR generator lowering теперь строит маленький CFG для поддержанных
structured blocks, while/if/switch, return, break/continue и yield. Стандартный
backward dataflow использует use/def, включая definition kills, до неподвижной
точки без произвольного лимита итераций. В frame попадают только locals,
которые читаются в продолжении yield до нового определения. Параметры остаются
в frame по существующему creation protocol.

Новый анализ не меняет admission и runtime protocol. Для адресных выражений,
let-in и пока не моделируемых statements (например, for/goto/asm) используется
прежний консервативный путь. Поэтому byte_stream остаётся 33 B, RLE — 47 B;
их pointer arithmetic также представлено AddrOfExpr. Ring generator не имеет
таких адресных expressions внутри producer, его frame уменьшается 53 → 13 B.
Бесконечный counter уменьшается 6 → 3 B. Slot coalescing, удаление неиспользуемых
параметров и универсальная минимальность frames не реализованы.

## Семантика и проверка

`TestGeneratorFrameDefinitionLiveness` проверяет definition kills, две фазы
цикла, обе ветки перезаписи, break и continue при изменении внешнего gate.
Nanz frame двухфазного ANF-примера уменьшается 8 → 3 B, overwritten-local
пример — 3 → 2 B. Runtime results совпадают на raw/optimized MIR2 и Z80.
Сохранён before run с отключённым новым анализом, который воспроизводит именно
старые frame sizes; затем hook восстановлен и регрессии выполнены заново.

Команды воспроизведения:

```sh
GOCACHE=/tmp/minz-crystal-go-cache go -C minzc test ./pkg/hir ./pkg/nanz -run '^(TestGeneratorFrameDefinitionLiveness|TestGeneratorExplicitCompletionDone|TestGeneratorForDerivedFrame|TestG1GenYield|TestG1GenYieldNegative|TestG1BoolComparisonYield|TestByteBufferImportedNative|TestByteRingImportedFIFO)$' -count=1
python3 research/crystal-hir/verify_generators.py
python3 research/crystal-hir/verify_ring.py
```

Generator corpus уже прошёл 2048 наборов на native Crystal, raw/optimized MIR2
и Z80, плюс 5 negative controls. Nanz/HIR regression gates и go vet ./pkg/hir
прошли. Финальный ring-прогон прошёл 525 наборов на четырёх engine, 6 отказов,
symlink gate, 7 legacy buffer-наборов и 10 независимых FIFO-трасс.
Receipt со сравнением layout: [evidence/iteration12-results.json](evidence/iteration12-results.json). Время harness не используется как benchmark generated program.

## Итог дня

Итоговая статья отделяет новые общие API/исправления от ранее существовавшей
основы HIR/MIR2/Nanz; содержит размеры descriptors и frames, реальные ASM
листинги, границы профиля и доказательства. Она создаётся в
reports/crystal-day-2026-10-25, standalone копия — в основном reports/.
