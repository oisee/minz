# Итерация 5: scalar Crystal → HIR → MIR2 → Go

В MinZ уже существует `minzc/pkg/mir2go`. Небольшой research emitter
[emit_go_probe.go](emit_go_probe.go) переиспользует его для scalar/meta Lanz,
полученного native Crystal frontend в итерации 2.

```sh
python3 research/crystal-hir/verify_go.py
```

Runner использует сохранённый typed Lanz fixture из новой статьи, генерирует
самостоятельный Go program и проверяет 7 функций × все 256 byte inputs:
**1792 набора на generated Go и повторно на native Crystal**. Исходный corpus
также ранее прошёл raw/optimized MIR2 и Z80. Oracle исходник — `meta_showcase.cr`;
fixture и source hashes записаны в `evidence/iteration5-results.json`.

Probe удаляет Lanz signature stubs, проверяет HIR, понижает raw HIR в raw MIR2 и
вызывает существующий Go emitter. Временный пустой main нужен контракту emitter;
саму проверку выполняет Go test harness. Вывод с `// TODO:` от неподдержанного
opcode отклоняется до записи готового результата.

Это не перенос compiler Crystal на Go. Его semantic frontend остаётся Crystal
host executable. Generated Go представляет виртуальные регистры как int64,
арифметику по разрядности масками, memory как flat []byte heap, blocks как goto.
Go host program использует обычный Go runtime; обещание «без GC» для Z80 к нему
не переносится. Records, arena, exceptions и произвольная stdlib этим прогоном
не проверялись. Backend имеет дополнительные ограничения; unsigned scalar corpus
не доказывает правильность signed comparisons или остальных MIR2 operations.

CLI mzc ещё не содержит production .cr pipeline. Это проверяемая возможность
ещё одного backend на существующем frontend/HIR пути, без ABAPiti и без нового
language lowering. Более богатое Crystal → idiomatic Go потребует другой работы.
