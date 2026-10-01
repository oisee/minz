# Аудит MinZ и экосистемы GPU-поиска — 2026-09-19

## Вывод

У проекта есть несколько ценных, реально реализованных направлений: production MIR2→Z80, экспериментальный LIR с распространением ограничений, GPU/CPU-поиск оптимизаций и allocation tables, PRNG-графика и AY/PT3-аудио. Их готовность существенно различается. Ближайшая полезная инвестиция — достоверные проверки и воспроизводимые артефакты, затем небольшие интеграции с измеримым выигрышем.

Очередь работ и критерии готовности: [древовидный бэклог](../docs/project/BACKLOG.md). [Инвентарь таблиц](2026-09-19-Artifact-Inventory.json) фиксирует размеры, заголовки и Git tracking; это не сертификат корректности содержимого.

## База и границы проверки

| Каталог относительно `~/dev` | Проверенная ревизия | Состояние |
|---|---|---|
| `minz` | `f51171a5` | master совпадал с origin/master после fetch; есть пользовательские untracked-примеры |
| `z80-optimizer` | `7701e474705b0a2d58364764870ff98d3c6390b7` | Есть большие untracked-таблицы и локальные результаты |
| `gpuforce` | `622109c75657d6930a79ff1ed54178215ddc9f10` | Локально изменён PRNG CUDA source/binary, удалены tracked build binaries |
| `6502-optimizer` | `c643bb9074c5e5743016691a80466238d8199030` | `git status --short` пуст |
| `z80-tables` | Не Git checkout | Сырые JSONL-таблицы, включая примерно 5.14 GB dense 6v |
| `minz-clones-archive-2026-09-17` | Архив | Сохранённые untracked-файлы и stashes старых MinZ-клонов |
| `_arch/gpuforce_from_z80opt` | Архив, не Git checkout | Найдены материалы PRNG-галерей; содержимое целиком не проверялось |

Соседние репозитории не изменялись намеренно: читались исходники, метаданные и запускались ограниченные CPU-тесты. Их remotes не обновлялись, поэтому это аудит локальных checkout, а не утверждение о последних upstream-версиях. GPU-перебор, полная проверка многогигабайтных таблиц, audio parity matrix и интерактивные demos не запускались. На машине доступны `nvcc` и две RTX 4060 Ti по 16 GB; наличие оборудования не является результатом GPU-теста.

## Что сейчас проверено исполнением

| Проверка | Результат | Ограничение |
|---|---|---|
| MinZ `go build -o /tmp/minz-backlog-mz ./cmd/minzc` | PASS, Go 1.24.3 | Сборка основного CLI |
| MinZ `make test-quick` | В логе 46 `ok` пакетов, 4 failing packages, 21 top-level failing test; **make вернул 0** | `-short`; skips скрыты фильтром; untracked влияют на showcase; это не release baseline |
| PBQP div/mod/nested-call repro старого аудита | PASS для трёх Z80 asserts | Не исчерпывающая проверка arithmetic/ABI |
| MinZ Z80T v2 loader, header count=2 / body=1 | **Принят без ошибки**, actual=1 | Минимальный синтетический repro |
| z80-optimizer `pkg/regalloc`, `pkg/cpu` | PASS | CPU unit tests; `pkg/enr`, `pkg/peephole`: no test files |
| gpuforce `pkg/vt2` | PASS | `pkg/pt3`, `pkg/ayplayer`: no test files |
| gpuforce `pkg/ayumi` | FAIL: три audio assertions, затем **panic: send on closed channel** | Один наблюдаемый прогон; не оценка всего PT3 pipeline |
| 6502-optimizer `pkg/cpu`, `pkg/search` | PASS | `pkg/inst`: no test files; GPU rules не перепроверялись |

Команды соседних CPU-проверок, из соответствующего каталога:

```sh
GOTOOLCHAIN=go1.25.0 go test ./pkg/regalloc ./pkg/enr ./pkg/peephole ./pkg/cpu -count=1 -timeout 60s
GOTOOLCHAIN=go1.25.0 go test ./pkg/pt3 ./pkg/vt2 ./pkg/ayplayer ./pkg/ayumi -count=1 -timeout 60s
GOTOOLCHAIN=go1.25.0 go test ./pkg/cpu ./pkg/inst ./pkg/search -count=1 -timeout 60s
```

Без override локальный Go launcher пытался скачать `go1.25` и завершался `toolchain not available`. Уже установленный `go1.25.0` позволил выполнить тесты без изменений go.mod. Это отдельная проблема воспроизводимого запуска на данном окружении.

Четыре failing packages MinZ:

- `pkg/abap`: отдельный повтор подтвердил отсутствие `@abaplint/core`. Это setup failure, не установленный дефект ABAP lowering.
- `pkg/nanz`: imports Lanz/Lizp, bit-accessor factcheck, showcase compile/assemble. Showcase обходит файловую систему и захватывает личные untracked-примеры.
- `pkg/vir`: 11 top-level failures, включая arithmetic, GCD, calls, lookup loop. Это экспериментальный/oracle-путь, не production backend.
- `pkg/z80testing`: E2E и iterator quality harnesses падают на Nanz parsing старого синтаксиса с `;`. Пакет занял около 240.6 s. До codegen эти конкретные repro не доходят.

## 1. MinZ: что изменилось относительно старых аудитов

Production использует PBQP/Z80 codegen; `--vir` удалён. `go list -deps ./cmd/minzc` не показал `pkg/vir`. PFCCO остаётся в `pkg/mir2/contracts.go`. Таблицы и solver пока физически находятся в общем `pkg/vir`; его импортируют `cmd/mir2asm`, `cmd/gpu-bench`, `cmd/mzv/file_host.go`.

Сентябрьский аудит старого checkout нельзя считать диагнозом текущего HEAD. Сейчас следующий файл успешно компилируется основным CLI и выполняет Z80 asserts:

```nanz
fun div8(a: u8, b: u8) -> u8 { return a / b }
fun mod8(a: u8, b: u8) -> u8 { return a % b }
fun double(x: u8) -> u8 { return x + x }
fun double_sum(a: u8, b: u8) -> u8 { return double(a) + double(b) }
assert div8(10, 3) == 3 via z80
assert mod8(13, 5) == 3 via z80
assert double_sum(3, 4) == 14 via z80
```

Однако достоверность общего тестового сигнала нарушена: `minzc/Makefile` пропускает `go test` через `grep`/`tail` без сохранения его exit code. Ложный green для `test-quick` воспроизведён. CI по-прежнему требует старые Go 1.20/1.21 и root npm setup при Go 1.24 в модуле и отсутствии tracked root package.json. В pipeline `LABEL AUDIT` остаётся комментарием; LLVM/GPU emitters имеют TODO-fallbacks вместо отказа для некоторых операций.

**Приоритет:** A1/A2 → A3/A4 в бэклоге. Исправления compiler semantics принимать по реальному Z80 execution, отделяя setup и устаревшие harnesses.

## 2. WFC на коде: где он и что делает

Реализация находится в MinZ: `minzc/pkg/lir/wfc.go`, `wfc_interblock.go`, `isel.go`, `pipeline.go`, описание ISA — `z80.go`. Экспериментальный `UseLIR` подключён в общем pipeline.

Проверено по текущему source:

- Есть домены допустимых locations, propagation и выбор с PBQP hints.
- `Collapse()` явно обходит инструкции по порядку (`for i := range s.Cells`).
- `Entropy()` определена, но поиск вызовов `.Entropy(` в `pkg/lir` ничего не нашёл.
- Поле `Loc.Alias` объявлено; чтение есть в `z3solve.go`, присваивания/инициализация в просмотренном пакете не найдены.
- Быстрые LIR unit tests прошли. Это не доказательство полного corpus execution.

Поэтому точное описание текущего пути — **распространение ограничений с последовательным выбором и PBQP-подсказками**. Entropy-driven choice и полноценный backtracking нельзя приписывать этому `Collapse()`.

Отдельный настоящий CPU backtracking есть в `z80-optimizer/cuda/z80_regalloc.cu`: `BacktrackState`, `backtrack`, `solve_backtrack`; тот же исходник присутствует в gpuforce. Это механизм поиска allocation assignments, а не доказательство интеграции WFC в production MinZ.

Remote-tracking ветка `origin/feat/lir-z80-hardening` заканчивается `c95b0f3f`. Все 22 записи `git cherry master origin/feat/lir-z80-hardening` помечены `-`: патчи уже представлены в master. Повторно сливать ветку ради её названия не нужно. Заявление в commit subject о нуле assembly errors не заменяет execution-проверку.

**Перспектива:** сначала alias/legality model и проверяемый отказ при конфликте; затем ограниченный эксперимент entropy ordering/backtracking против текущего baseline. G2/G5, без переключения production по названию алгоритма.

## 3. GPU allocation tables: данные есть, цепочка доверия неполна

В `z80-optimizer/data` реально находятся:

| Артефакт | Размер, bytes | Git tracked |
|---|---:|---|
| enriched_4v.enr | 4,104,402 | Да |
| enriched_5v.enr | 405,419,260 | Нет |
| enriched_6v_dense.enr | 942,369,916 | Нет |
| merged_ix_5v.bin | 400,702,579 | Нет |
| ix_expanded_6v_dense.bin | 1,684,752,173 | Нет |
| ix_derived_6v.jsonl | **0** | Нет |

Есть также два IX 6v JSONL-файла примерно по 11.5 GB. Это серьёзный объём вычислительных артефактов, но сам размер не доказывает завершённость, корректность или использование compiler. Хеш малого 4v ENRT и header counts сохранены в JSON-инвентаре; большие файлы целиком не хешировались и не декодировались.

MinZ loader ищет персональные `$HOME/dev/z80-optimizer` и уже архивированный `$HOME/dev/minz-vir`, а также cwd-relative paths; ошибки отдельных загрузок поглощаются. Нельзя переносить утверждение о работе таблиц на production CLI, который теперь не зависит от VIR.

Новая воспроизведённая проблема: **Z80T v2 ветка не сравнивает прочитанное число записей с заявленным**. Августовский fix ENRT/Z80T v1 не закрывает эту ветку. Минимальная программа для запуска через `go run /tmp/repro.go` из `minzc/`:

```go
package main
import (
    "encoding/binary"
    "fmt"
    "os"
    "github.com/minz/minzc/pkg/vir"
)
func main() {
    b := []byte{'Z','8','0','T',2,0,0,0,6,3,4}
    count := make([]byte, 8)
    binary.LittleEndian.PutUint64(count, 2)
    b = append(b, count...)
    b = append(b, 255) // одна infeasible record вместо двух
    p := "/tmp/minz-truncated.z80t"
    if err := os.WriteFile(p, b, 0600); err != nil { panic(err) }
    table, err := vir.LoadEnrichedBinary(p)
    if err != nil { fmt.Println(err); return }
    fmt.Printf("declared=2 actual=%d error=nil\n", len(table.Entries))
}
```

Наблюдаемый вывод: `declared=2 actual=1 error=nil`.

В той же ветке loader читает body целиком и предварительно выделяет память по header count; перед автоматическим подключением крупных таблиц нужны явные limits и бюджет памяти. Это code-review finding, отдельный OOM-repro не запускался.

В GPU `check_interference` (`cuda/z80_regalloc.cu`) конфликт проверяется как равенство индексов assignments. Проверка physical aliases B/BC, H/HL этим не выражена. Это ограничение проверенного генератора; не утверждение, что каждый файл таблиц создан именно этой версией или содержит ошибку.

**Перспектива:** manifest generator revision + domain + format + hash + verification mode; маленький проверенный table slice, затем measured hits в конкретном consumer. Не начинать с повторного многосуточного перебора всей базы.

## 4. Peephole и арифметический superoptimization

`z80-optimizer` содержит CPU executor/verifier, CUDA search, arithmetic libraries и enrichment tools. `6502-optimizer` — отдельный ISA backend с CPU/search tests, которые прошли. Обобщение конкретных правил в семьи — полезный кандидат для переноса, но headline «403 families» здесь взят из README и заново не пересчитан.

В `z80-optimizer/pkg/search/verifier.go` функции `exhaustiveAll` и masked variant переключаются на **reduced sweep** для трёх и более дополнительных регистров или SP. Поэтому README-формулировка «all inputs / provably correct» не универсальна: sampled sweep и полная проверка конечного домена должны иметь разные статусы. Даже полный перебор доказывает эквивалентность только в реализованной модели машины и заданных assumptions.

**Перспектива:** proof-status metadata для каждого правила, flags/clobbers/ISA preconditions, отдельный independent emulator check; применение одной востребованной семьи на production corpus с частотой срабатывания и ablation. Это ценнее количества JSONL-строк.

## 5. gpuforce: PRNG-графика и AY — самостоятельные треки

### PRNG / изображения

Исходники включают budget/cascade/segmented/joint search. Есть carrier catalog, сохранённые результаты и архив галерей. Локальный diff `cuda/prng_budget_search.cu` добавляет `--apply-mask` и маску применения; поэтому локальный binary нельзя описывать одной только committed revision.

Реализованный search и найденная картинка ещё не означают воспроизводимый target decoder. Нужна цепочка: fixture → pinned search parameters/seed → payload → decoder → битовая сверка → payload bytes + decoder bytes + T-states + visual error. Поиск изображения с ограниченным бюджетом — отдельный продуктовый эксперимент, его не следует откладывать до решения всего register allocation.

### AY / PT3

Код есть в `pkg/pt3`, `pkg/vt2`, `pkg/ayplayer`, `pkg/ayumi`, CLI `pt3decode`, `pt3render`, `ayplay`; присутствуют Pascal/C/MZE oracle tools и `scripts/psg_compare.py`.

Свежие проверки дали три failures (`TestAyumiBasicOperation`, `TestEnvelopeGeneration`, `TestPSGCommands`) и panic в `AudioMixer.mixSamples`, `orchestrator.go:159`. `Stop()` закрывает channels, пока mix loop потенциально продолжает отправку; это конкретное направление проверки lifecycle. Необходимо отдельно проверить синхронизацию mixBuffer; data race в этой сессии не подтверждалась `-race`.

Апрельский milestone внутренне противоречив: headline **10/12 PERFECT**, но перечислены три non-perfect fixtures — `Beautiful Agony`, `take5`, `disco08`; в таблице 12 строк и девять PERFECT. Новую parity matrix нужно получать машинно. Этот аудит не перепроверял её и не утверждает текущие проценты точности.

**Приоритет AY:** cancel/join/close lifecycle и audio output tests → fixture matrix с first divergent frame → затем audio search/atlas expansion. Синтез, player parity и codegen correctness требуют разных критериев готовности.

## 6. Архивы и сохранение результатов

Архив старых MinZ-клонов содержит README с инвентарём 99 untracked-файлов и 18 stash patches; эти числа взяты из README, не пересчитаны. Указаны rescued enriched table, регрессионные тесты, контексты, Tetris frames. Ветка `research/pfcco-paper-v2` существует локально и в origin tracking.

Архив полезен как источник потерянных fixtures и исследовательского текста, а не как ветка для массового применения старых patches. Перед переносом сравнивать patch-id/содержание с текущим master, затем переносить маленький самостоятельный артефакт с тестом. Соседние каталоги не чистить до inventory и provenance.

## Решение о последовательности

1. **Quick wins:** исправить тестовый exit code; Z80T v2 record count; актуализировать entry-point docs и команды toolchain.
2. **Фундамент:** clean corpus manifests, Z80 execution gates, asset provenance и alias/legality contracts.
3. **Практические результаты:** CP/M smoke, IRC/TUI mock; независимо — один воспроизводимый PRNG decoder и исправление AY lifecycle.
4. **Измеряемые исследования:** PFCCO ablation; table lookup coverage на holdout; WFC/search comparison; перенос peephole families из 6502-подхода.

Расширение GPU-перебора имеет смысл после фиксации модели и протокола проверки. Самые близкие результаты здесь — исправление наблюдаемых дефектов, сохранение уже найденных артефактов и маленькие работающие интеграции.
