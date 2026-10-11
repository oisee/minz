# Последовательное little-endian чтение/запись слова в MIR2

Исходные `ld_word` и `st_word` из полного `stdlib/fs/fat12.nanz` теперь имеют экспериментальное преобразование MIR2. На 1536 вызовах этих двух helpers оно уменьшает фактическую работу Z80 со **115 200 до 89 088 T-states (−22,67%)**, а BIN полного модуля — с **9543 до 9397 байт (−146)**. Это нагрузка helpers, не измерение дисковой системы или всей библиотеки FAT.

Исходник не изменён. До преобразования заморожен полный файл, SHA256 `6c11a845959df00dc01471e273f0196e7939121eaa9cb6e1f6206cc492ffbbb5`, база компилятора `e436bd98b064db730b2a9acaa5b9723d7b55f95f`. Независимая модель: little-endian `lo | hi<<8`, обратное разложение `uint16`, адреса по модулю 65536. См. исходные определения `stdlib/fs/fat12.nanz:32`, Book v7 §3.1/3.3 для числовых/указательных типов и `docs/grace-gap-closure.md` для реализованного wrap-контракта VM/Z80.

## Как включить

`minzc --fuse-byte-words ...` или `pipeline.Options{FuseByteWords:true}`. По умолчанию false. Обе публичные HIR pipeline функции учитывают флаг; `ProfileCompilationID` сериализует его. Нулевые новые поля JSON опущены, поэтому неизменённые конфигурации/инструкции сохраняют прежнюю сериализацию.

## Что перенесено из LIR/VIR

Новый `FuseSequentialByteWords` распознаёт два соседних byte-access в одном SSA-блоке. Он требует корректный SSA/dominance, единственного потребителя удаляемых промежуточных результатов, порядка low→high, общего 16-битного основания адресов и смещения +1. Между доступами разрешены лишь перечисленные тотальные чистые операции; память, вызовы, ports, непрозрачные эффекты прекращают сопоставление. Функции с ASM отвергаются целиком. Глобальное небезопасное удаление DAG из VIR не скопировано.

`Inst.SequentialBytes` отличается от обычной широкой операции: каждый байт отдельно проверяется и наблюдается; ошибка второго байта сохраняет эффект первого. Неотмеченная широкая операция оставляет прежнюю предварительную проверку всего диапазона. Тот же низкий→высокий порядок даёт имеющийся Z80-emitter слова.

VM `CheckDataAccess` получает **точные исходные указатели на byte-инструкции**, включая Op/Ty. Их происхождение и копия чистого адресного DAG сохраняются как хешируемые metadata. Это не дополнительный исполняемый код. MIR verifier заново проверяет byte op/type/адресную смежность; MemoryHost независимо выводит адрес каждого оригинала из текущих символических значений и чистого DAG. Произвольные metadata не дают права на глобальную память. Перенумерация отдельно сохраняет ID metadata; inlining таких функций пока запрещён, поскольку иначе нужен отдельный контракт переноса callback identity.

Не введён универсальный volatile/MMIO интерфейс: сохраняются существующие byte-access и их порядок, эффекты не пересекаются. Это не доказательство всех аппаратных MMIO устройств.

## Проверки и результаты

- Полный исходный FAT: 3 смещения (1,255,512) ×256 независимых слов ×store/read =1536 persistent вызовов; полные 1026 байт ресурса после каждого вызова, независимые per-byte counts и ordered final-VM события; assembled native совпадает с raw snapshot/access evidence.
- Дополнительно фактический native `FFFF→0000`: 256 слов ×store/read ×2 режима =1024 вызова, точный порядок read/write/value, вся посторонняя RAM вне явно ограниченного используемого стека, immutable BIN, исходный PC возврата и восстановленный SP.
- 36 raw/fused VM runs: store/load ×размер1/2/65536 ×denied first/second/none ×2; независимые fault-prefix/event/value ожидания, полная heap RAM. Отдельно fault без observer и прежний unmarked-wide атомарный range check.
- Отрицательные controls: shared intermediates, reversed order, intervening call/load/store/ASM, forward SSA, malformed flag, forged high address, независимое отсутствие global authority, exact original callback identity, metadata после renumber и запрет inlining.
- Полный MIR2 **PASS 24,121s**, полный HIR **PASS 32,961s**; focused native+identity+global-authority **PASS 10,581s**, дополнительные focused MIR safety **PASS**. CLI help содержит `--fuse-byte-words`. Полный медленный pipeline/CLI suite повторно не запускался.

| Режим | T-states | BIN | SHA256 BIN |
|---|---:|---:|---|
| Обычный | 115200 | 9543 | `60803503e50440190c40715fc3aa2c89ad23ba8ac83a301dd6beb91c64612b6c` |
| Fusion | 89088 | 9397 | `10017cc646901373a84d283bb5f5e4d5fe67d0cfabbb6b51eed5c3371efe6f13` |

Первый журнал `first.txt` — предварительный замер до полного provenance-аудита; итоговый `native.txt` относится к исправленному provenance-пути. Эксперимент с сохранением высоких адресов как исполняемых live-значений оказался убыточен (220416 T/9552 B) и заменён проверяемыми metadata. Это решение не меняет исходный алгоритм.

## Воспроизводимость

Из `minzc/`, Go1.26.8:

```
go test ./pkg/mir2 -run '^TestSequentialByte' -v
go test ./pkg/pipeline -run '^(TestFATSequentialWordFusionNative|TestByteWordFusionCompilationIdentity|TestSequentialByteGlobalOriginAuthority)' -v
go test ./pkg/mir2 ./pkg/hir
```

`MINZ_FAT_WORD_FUSION_ARTIFACTS=<directory>` сохраняет точный исходник, workload, raw snapshots, final MIR, ASM/BIN и native evidence. Большие JSON в пакете сжаты gzip без изменения распакованного содержания. Manifest фиксирует SHA256 всех payload-файлов и исходную базу. Scope — один полезный элемент миграции LIR; LIR/VIR этим коммитом не удалены.
