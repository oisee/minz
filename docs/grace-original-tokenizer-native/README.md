# Исходный self_tokenizer: независимый полный state oracle на Z80

База `8bfb0b02`. **`examples/nanz/self_tokenizer.nanz` не изменён**. Его исходные функции `init_keywords` и `tokenize_all` выполнены на raw MIR2 VM и реально собранном Z80 с независимой Go-моделью лексера и interning. Checked derivative не используется как реализация или оракул.

Один init и 30 последовательных входов: исходный `fun add`, все операторы, слова, повторное interning, числа, пробелы, комментарии, unknown, NUL, пустые/незакрытые строки, 255/256/257-байтовые атомы и повторная нагрузка после них. После **каждого** вызова проверены все 8192 байта string arena, 4096 байтов token arena, 4096 байтов source region и все 47 globals. Неиспользованный хвост памяти тоже входит в оракул. Модель вычисляет токены, pointer sharing, keyword pointers, позиции/count/bump сама через host string scanning и Go map.

Native результат: **1 916 274 T-states, 5466 байтов**, BIN SHA256 `cb7a8da6dd152d5bc97f99d05a7f33c41b53bda6c8accb56b1f7f1144c6f5545`. Сохранены source, workload, независимые ожидания, raw/native observations, полный ASM/BIN и MIR. Нативный стенд дополнительно сверяет точную последовательность extern peek/poke с raw VM, вход/выход функций и восстановление SP. Независимая модель проверяет результаты/память отдельно; совпадение VM↔Z80 само по себе не служит оракулом.

## Что исправлено

Первый nonempty native вход отклонялся: `execution outside instructions at 074e`. Это реальный сбой допуска в закрытом memory-host стенде, **не ошибка tokenizer lowering**. Codegen создал tail-call alias `str_len EQU peek`; оба имени имеют адрес 0x074e. Старый range selector выбирал последнее имя в группе, недостижимый `str_len` перекрывал достижимый `peek`.

`memoryHostReachableInstruction` допускает группу **одинакового entry PC**, если достижимо любое её имя. Следующая группа с другим entry PC начинает новое решение, поэтому разрешение не переходит на соседнюю недостижимую функцию. Длины и executable bytes по-прежнему берутся только из строк реально собранных инструкций, не из диапазона имён.

Регрессия собирает настоящий EQU alias и проверяет выполнение достижимого тела в обоих вариантах достижимости имён. Переходы внутрь immediate operand, в DB и в следующую отличную недостижимую функцию остаются запрещены. Existing code-as-data refusal, частичный byte prefix, indexed prefix и stack tests прошли. Bytegroup alias semantics не ослабляет runtime/неизвестные вызовы.

При первоначальной диагностике тест неверно вызвал pre-allocation SSA verifier на финальном allocated MIR: `%r114` уже намеренно объединён с loop block param через `PreallocCoalesce` (см. комментарии `precoalesce.go`, `alloc.go`). Этот отдельный тестовый вызов удалён; production preallocation Verify и native admission сохранены. `before.txt` фиксирует раннюю диагностику; `native-first.txt` — настоящий alias admission failure; `after.txt` — успешный исправленный путь.

## Семантика и пределы исходного примера

Book v7 §2.6/§3.2 описывает byte/address widths; оригинальный файл определяет собственный формат токенов и fixed-address arenas. Модель сохраняет его фактические правила, не выдаёт их за полный lexer языка:

- `classify` признаёт KW только для десяти слов, хотя init выделяет пятнадцать: `enum/global/struct/import/assert` остаются IDENT. Это ограничение исходного tokenizer, а не miscompile.
- Четвёртый байт `[type, ptr_lo, ptr_hi, 0]` всегда нулевой: заголовочный комментарий «len» не соответствует emit implementation.
- Длина interned текста сужается до u8. При 256 получается пустая строка, при 257 — первый байт. Это сохранённая реализация; поддержки произвольно длинных лексем не заявляем.
- Нет arena/token capacity admission внутри примера. Workload явно ограничен и помещается в arenas; не обещаем безопасность OOM/неограниченной рекурсии skip.

## Runtime: что реально исполняется

Оригинальный файл сам говорит `Run: mzv`, объявляет extern `peek/poke` и имеет печатающий `main`. Для **вычислительных entry** тест использует настоящие Z80 тела `LD A,(HL); RET` и `LD (HL),C; RET` с проверенным фактическим ABI. Весь original source сохраняется, main тоже компилируется, но main не вызывается. Code origin 0x0400; strings 0x9000..0xafff, tokens 0xb000..0xbfff, source 0xc000..0xcfff, отдельный stack 0xe000..0xefef и return PC 0x0100.

Это native computational proof с явным memory runtime. **Не запуск оригинального main под CP/M, не console-output proof, не реальное железо.** У `@print` этого примера текущий default backend выводит порт 0x23, поэтому приписывать этому тесту BDOS было бы ошибкой. CP/M main/печать требуют отдельной сборки/целевого runtime и оракула stdout. Checked tokenizer по-прежнему отдельный файл и отдельный тест.

Каталог сохраняет все 193 пути и теперь отмечает **23 ограниченно проверенных**; новый original-tokenizer row содержит exact test/hash/workload/runtime boundary. Старые файлы карт других этапов не перезаписаны.

## Проверки и воспроизведение

Из `minzc/`, Go 1.26.8:

```sh
MINZ_ORIGINAL_TOKENIZER_ARTIFACTS=/absolute/evidence go test ./pkg/pipeline -run '^TestOriginalTokenizerFullStateVMZ80$|^TestTypedTokenizerProgramBaseline$' -count=1 -v
go test ./pkg/pipeline -run '^TestMemoryHostReachableAliasAndUnreachableNextFunction$|^TestMemoryHostNative(CodeDataReadRefusal|PrefixAndStack|ByteEffects|InvalidSecondBytePrefix)$' -count=1 -v
MINZ_SHOWCASE_ORACLE_CATALOG=/absolute/catalog.json go test ./pkg/nanz -run '^TestShowcaseExecutableOracleCatalog$' -count=1 -v
```

Оригинальный и checked tests прошли вместе за 7.687 s; alias/защитные regressions за 0.022 s. Полный медленный pipeline/showcase не запускался этим этапом. JSON payloads сжаты gzip без потери; manifest содержит SHA256 всех сохранённых файлов, кроме себя.
