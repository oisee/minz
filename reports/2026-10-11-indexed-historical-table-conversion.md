# Адресная конвертация настоящих исторических таблиц

Из восстановленной `enriched_5v.enr` получена запись для `triple` из неизменённого `examples/nanz/09_function_pointers.nanz`. Без Z3 и без перестановок старого назначения текущий независимый допуск принимает её. Обычный CLI использует конвертированную таблицу: полный BIN 34 → 31 байт, прямой вызов 66 → 54 T, реальный косвенный `apply(triple,x)` 123 → 111 T. На всех 256 входах VM и native совпадают с `3*x mod 256`; после native вызова SP и весь машинный код неизменны. Это измеренная польза одной записи, не доказательство оптимальности исторической таблицы.

## Что именно читается

`/mnt/safe/z80-compiler/tables/enriched_4v.enr` и `enriched_5v.enr`: ENRT v1, 16-байтный заголовок, 12 u16 метрик, переменные записи: `ff` означает неосуществимую форму; иначе `n:u8, cost:u16LE, assignment[n]:u8, flags:u16LE, metrics[12]:u16LE`. Проверяются версия, резервные байты, число метрик, полное каноническое число форм и число vreg каждого прочитанного осуществимого record. Обход ограничен 256 MiB; весь файл в RAM не попадает. Для выбранной записи сохранены индекс, прочитанные байты, SHA заголовка и SHA прочитанного префикса. Последний **не является SHA всего файла**; хвост не проверен.

Используется существующая каноническая enum-модель: n=2..max, затем width combo, mixed-radix location sets, затем interference mask. Заголовки 4v/5v совпадают с её 156 506 / 17 366 874 формами. Пространство byte locations — A, C, все GPR8, GPR8 без A; word — HL, DE, BC/DE/HL. GPU locations явно переводятся в текущий namespace.

У подготовленной текущей задачи вычисляются точные одновременные live values, а не объединение мёртвого входа и заменяющего результата. Реальный ParamLocs фиксирует представимый ABI (A/C/HL/DE). Непредставимый точный ABI отказывается, а не расширяется. Исторический ключ **не содержит op bag, immediates, pattern scratch/flags или реальную стоимость emitted moves**. Поэтому найденное назначение — только предложение; `AddChecked` заново проверяет текущие инструкции, ABI, aliases, scratch, flags и стоимость, создавая текущий полный ключ. `LookupChecked` остаётся неизменным.

6v dense имеет другое перечисление подмножества и отказывается до получения оригинального порядка mask. OFB/IX не угадываются. Для production-применения эти форматы не конвертированы.

## Проверенные результаты

| Исходная функция | Запись | Прочитано | Результат полного BIN |
|---|---:|---:|---:|
| 09: triple, ENRT5v | 158626 | 4 107 216 B | 34 → 31 B |
| 09: double, ENRT4v/5v | 98 | 2 966 B | равенство; smaller selector сохраняет baseline |
| 16: low_nibble | 98 | 2 966 B | 192 → 192 B |
| 20: test_eq/test_neq | 16 | 483 B | 45 → 45 B |
| 06: weighted | нет | 0 | 13 vreg; вне целой задачи ≤6 |
| 20: hash3 | нет | 0 | 9 vreg; вне целой задачи ≤6 |

Weighted может состоять из маленьких арифметических форм, но нынешний consumer связывает **целую подготовленную функцию**, включая save/return moves. Для повторного использования block/island assignments потребуется отдельный доказанный boundary contract. Этого изменения здесь нет.

В одном прогретом host запуске обычная compilation до импорта заняла около 1.67 ms, адресный импорт двух назначений около 220 ms. Это wall time одного запуска, включая последовательный prefix scan, не устойчивый benchmark и не оценка cold I/O. Память reader ограничена малым record buffer; память самой compilation отдельно не измерена.

## Команды

Из `minzc/` (Go 1.26.8):

```sh
go run ./cmd/import-enrt --source ../examples/nanz/09_function_pointers.nanz \
  --enrt /mnt/safe/z80-compiler/tables/enriched_5v.enr \
  --out /path/new-checked.json --evidence /path/new-evidence.json

go run ./cmd/minzc ../examples/nanz/09_function_pointers.nanz \
  --Osize --asserts none --checked-local-table /path/new-checked.json \
  -o /path/selected.a80
```

Конвертер сохраняет оригинальное имя и каталог исходника, принимает `--stdlib` для импорта, записывает SHA исходника и полный per-function отказ/допуск. Output paths должны быть разными и новыми. Source/ENRT errors у отдельных функций записаны в diagnostics; полученный checked JSON может быть пустым. Native-замер **не выполняется самим конвертером**; в данном пакете он отдельно подтверждён тестом и exact артефактами.

## Проверки и артефакты

`TestIndexedLegacyShapeSimultaneousValues`: dying tied input, live input/result, equal move, simultaneous params, new definition/liveout; независимый `checkedLocalCost` принимает destructive dead-input reuse и отклоняет живой old input.

`TestIndexedENRTReaderAndCurrentBinding`: variable prefix records, infeasible markers, corrupted magic/version/count/metrics, truncation, wrong record n, index outside max; immutable bound function, изменённый immediate (same coarse shape, новый checked key), ложный return-C ABI. Исторический cost 9999 не принимается за текущий.

`TestIndexedActualHistoricalENRT` — opt-in корпусный probe через `INDEXED_IMPORT_EXPORT`. `TestIndexedHistoricalTripleNative` — opt-in реальный ENRT через `INDEXED_ENRT5`, native все 256 прямых и косвенных входов. Полный `go test ./pkg/vir -count=1` прошёл (2.111 s). Generated CLI ASM byte-for-byte совпал с API ASM. Exact source, ASM/BIN, checked tables, provenance и логи находятся в `reports/artifacts/indexed-import`; SHA manifest и архив packet сохраняются на шаре.

Полный pipeline suite этим шагом не заявляется. Historical source tables не редактировались, old evidence/IDs не переписаны, default baseline backend не изменён.
