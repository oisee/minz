# Исторические формы: покрытие тех же 118 функций

Повторный аудит тех же 12 исходных программ, 118 функций, через настоящие ENRT4v/5v и текущие direct/quotient API дал **9 уникальных допущенных функций**: 3 строго меньших кандидата, 4 равных, 2 больших. Каждый допущенный кандидат исполнен native внутри полного исходного модуля, включая кандидаты, которые selector не выбирает. У baseline и кандидата 67 074 проверенных набора входов на путь; независимый оракул — raw HIR→MIR VM, не solver или table codegen.

| Функция | Тело baseline/candidate | Native T до/после | Наборы |
|---|---:|---:|---:|
| weighted | 36 / 12 B | 150 / 54 | 256 |
| 09 double | 2 / 2 B | 14 / 14 | 256 |
| 09 triple | 7 / 4 B | 34 / 22 | 256 |
| tiny add | 2 / 2 B | 14 / 14 | 65 536 |
| tiny double | 2 / 2 B | 14 / 14 | 256 |
| tiny triple | 7 / 4 B | 34 / 22 | 256 |
| StrRef test_eq | 3 / 4 B | 17 / 21 | 1 (нет параметров) |
| StrRef test_neq | 3 / 4 B | 17 / 21 | 1 (нет параметров) |
| low_nibble | 3 / 3 B | 17 / 17 | 256 |

Здесь T измерены от входа функции до исходного return sentinel, включая RET; setup bootstrap не включён. Поэтому они на 32 T меньше чисел helper с bootstrap в предыдущем weighted отчёте. Все native пути восстановили SP, вернулись на sentinel и сохранили весь исходный машинный image. Для `tiny add` проверены все пары byte-входов; остальные byte unary — все 256; константные функции — единственный случай. SIMD/GPU исполнение в этом аудите не использовалось.

**Полный BIN** уменьшается только у weighted 36→12 B, `09_function_pointers` 34→31 B, `tiny` 11→8 B. Остальные 9 модулей сохраняют размер. Строго меньший selector не выбирает equal/larger candidates. Ранее в промежуточном сообщении две StrRef-константы были названы равными по размеру — фактическая таблица этого аудита уточняет: они больше на 1 B и медленнее на 4 T.

## Прямые формы и quotient

Direct ENRT допущен для 7 уникальных функций; quotient — для 6, пересечение 4. Для каждой формы пробовались оба файла: raw-count 14 direct admissions и 12 quotient admissions — это **дубли между 4v/5v**, не 26 разных функций. У пустой таблицы нет hit. Исторические entry labels/cost не считаются сертификатами: каждый proposal проходит текущий полный `AddChecked`, а selector — повторный `LookupChecked`.

107 функций отклонены до поиска: 55 не являются single-result leaf block; 24 содержат hard class/invalid definition; 9 имеют неподдержанные параметры; 19 — неподдержанные opcodes: ext7, shr3, sdiv3, sar2, smod2, sext1, call1. Осталось 11 подготовленных задач, две без допущенного назначения:

- `StrRef hash3`: direct содержит 9 vreg, quotient имеет точный singleton ABI, отсутствующий в исторической locset модели. Текущее ограничение не снимается.
- `hash16`: direct требует 5v и доходит до **256 MiB prefix read budget**, quotient явно byte-only. Ошибка теперь различает лимит чтения и физический EOF/truncation. Это не утверждение отсутствия решения в таблице и не допуск word XOR. Предыдущий свежий solver audit не выдавал сертификат для hash16 из-за missing direct word-XOR pattern. Число Prepare-eligible функций здесь 11; положительных generator admissions в старом отчёте было 10. Новая word ISA область не включена.

В одном прогретом host corpus run последовательный поиск `hash16` занял около 15.3 s — это главный отрицательный compile-time результат. Остальные admitted samples примерно 10–377 ms с native проверками в измеренном интервале; этот `ImportMS` включает также emission и exhaustive native loops, поэтому его нельзя называть чистым временем импорта. Нет измерения peak RAM compilation или cold I/O. Reader хранит малый record buffer и ограничивает суммарное чтение 256 MiB на lookup.

## Parametric reuse

`TestIndexedSourceCloneProposalReuse` собирает обычные Nanz исходники `x+3`, переименованный `x+17`, и `x+255` после другой функции (изменившаяся нумерация MIR vreg). Историческая форма и record одинаковы, но текущие full keys различны; старый certificate не hit. Каждый proposal заново допускается и native проверяется на всех 256 входах против независимого `x+imm mod256`.

`TestIndexedParametricSourceAndRegisterIdentity` дополнительно принудительно сдвигает vreg на 37 через MIR API, меняет имена/константы и проверяет то же свойство, также все 256 native входов. Это повторное использование готового **назначения формы**, а не повторное использование полного сертификата при изменившемся исходнике. Задания не решались заново через Z3.

## Следующий практический gap

Адресный prefix scan неудобен для поздних записей: целесообразен отдельный проверяемый offset index/checkpoint file, связанный с точной идентичностью ENRT. Это может уменьшить compile-time без изменения формы/ABI admission. Для StrRef сначала требуется доказанная адаптация ABI или представимый proposal domain; просто ослабить exact ABI нельзя. Для word hash16 по-прежнему нужен прямой checked pattern с реальной native пользой — исторический assignment сам по себе не решает этот ISA gap.

## Воспроизведение и evidence

Из `minzc/`:

```sh
INDEXED_COVERAGE_EXPORT=/path/new-packet go test ./pkg/pipeline \
  -run '^TestIndexedShapeCorpusCoverage$' -count=1 -v
INDEXED_ENRT4=/mnt/safe/z80-compiler/tables/enriched_4v.enr \
  go test ./pkg/pipeline ./pkg/vir \
  -run '^(TestIndexedSourceClone|TestIndexedParametric|TestIndexedReadBudget)' -count=1 -v
```

Исходные ENRT файлы — `/mnt/safe/z80-compiler/tables/enriched_4v.enr`, `enriched_5v.enr`; 6v dense/OFB по-прежнему не принимаются. Exact source, baseline/selected/candidate ASM, полные BIN, MIR, current tables, per-attempt record/prefix SHA, native sample counts и причины отказов находятся в `reports/artifacts/indexed-shape-coverage-evidence.tar.xz` с SHA256 manifest. Prefix SHA не выдаётся за full file SHA. Полный VIR проверяется отдельно. Полный pipeline PASS не заявляется.
