# Полный typed tokenizer: source → Z80 и bounded copy PGO

Typed-порт `examples/nanz/self_tokenizer.nanz` теперь проходит полный путь через HIR/MIR, ассемблер и Z80 emulator. В отличие от предыдущего interning subsystem этот caller выполняет сканирование исходника, классификацию, поиск/размещение строк и запись токенов. Проверяется порт с явными resource pointers; исходный extern peek/poke/printing ABI и physical hardware этим этапом не проверяются.

## Поведение и независимый эталон

[Исходник](../examples/nanz/grace_tokenizer_checked.nanz) использует persistent Pascal arena, отдельные source/token resources и реальные четырёхбайтовые token records. Тридцать последовательных вызовов включают обычный Nanz, все операторы и keywords, комментарии, NUL, повторные строки, незакрытые строки и spans 255/256/257 байтов. Scanner position/token count имеют u16; длина interned span сохраняет исходное u8 wraparound.

Сохранены особенности исходника: enum/global/struct/import/assert инициализированы, но классифицируются как identifiers; четвёртый байт token record равен нулю; EOF не записывается; escapes не интерпретируются. Физический zero sentinel после logical source length обслуживает исходный lookahead, следующая guard byte остаётся непрочитанной. Keyword initialization выполняется однажды, arena и неиспользованный token-buffer tail сохраняются между вызовами.

Независимый host lexer проверяет результаты, все global values, полные resource snapshots и точное число resource reads/writes. Счётчики доступа к globals сверяются между raw MIR VM и Z80, но не моделируются независимым host. Raw HIR/MIR, postcompile replay и исполнение assembled binary должны согласоваться с host для каждого вызова. Token checksum читает настоящие records; синтетическая weighted работа не добавлена.

## Измеренный выбор

| Вариант | T-states, полный workload | Module bytes |
| --- | ---: | ---: |
| Original | 2 270 286 | 4 346 |
| Guarded forward copy | 2 259 865 | 4 475 |

Существующий opt-in forward-copy selector сохраняет оба кандидата для speed и size. Speed выбирает copy: −10 421 T, около −0,459%, +129 bytes. Size оставляет original. Замер относится к полному caller, а не только к внутреннему copy loop; доминирующие scanner и interning search остаются следующими участками исследования.

Для каждого кандидата сохраняются exact copy choices, source/pre/post structural proofs, полный replay, assembled binary, bytes/T-states и typed evidence SHA256. Новый permissive mixed proof не вводился: прежний mixed validator сохраняет свои ограничения.

## Исправление HIR function splitting

Первый прогон обнаружил ошибку CompileHIRSteps: splitter включал branch-local `s` в интерфейс извлечённой части scanner, хотя binding недоступен на границе. Это не ошибка исходного raw parser/VM. Исторический failing run сохранён отдельно.

Зависимости extracted suffix теперь собираются с lexical block bindings; sibling branch declarations не становятся внешними inputs. Для callable aliases учитывается имя вызова, неподдерживаемое scalar forwarding отклоняется. Предложенное преобразование проверяется целиком HIR Verify до публикации; при отказе тело функции восстанавливается. Исходные statement pointers сохраняют origins и lineage. Scanner split уменьшился с шести inputs до четырёх.

Регрессии проверяют branch locals, rollback, callable dependencies/captures, отдельные block bindings и сохранение loop origins. В ходе ревью отдельно воспроизведён прежний LowerModule bug: standalone block shadowing теряет outer binding. Исправление этого lowerer bug следует отдельным этапом; splitter test проверяет lexical interface и не объявляет этот backend case исправленным.

## Артефакты и воспроизведение

[Пакет доказательств](grace-pgo-tokenizer-artifacts/README.md) содержит passing и initial failing source, независимый host, typed SHA verifier, workload, state/access results, assembly/binaries и исходный patch. Большие JSON/MIR сохраняются с deterministic gzip; manifest фиксирует SHA256 каждого файла.

```sh
python3 docs/grace-pgo-tokenizer-artifacts/verify_host.py
cd minzc
go run ../docs/grace-pgo-tokenizer-artifacts/verify_evidence.go
go test ./pkg/pipeline -run '^(TestTypedTokenizerProgramBaseline|TestTokenizerIndependentHostKnownSemantics)$' -count=1 -v
```

CLI использует существующие `--hir-grace --hir-profile-joint --hir-profile-forward-copy-sites --hir-profile-auto <dir> --hir-profile-auto-workload <workload.json>`; `--Osize` выбирает размер. Проверки HIR/MIR/Nanz: 7,431 / 3,881 / 1,418 с; CLI: 18,910 с. Общий pipeline проходит за 237,363 с (отдельно проверенный budget fixture исключён); showcase сохраняет 31 прежнее падение без новых. Compile-only budget audit подтверждает точное равенство четырёх actual two-pass ASM и бинарников с предыдущим этапом: 344/344/232/344 bytes. Сохранённая прежняя 722,080-секундная проверка исполнения применима к этим неизменным бинарникам; новое исполнение budget workload не заявляется.
