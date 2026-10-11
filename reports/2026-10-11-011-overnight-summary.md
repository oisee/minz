# Ночной итог Grace / PGO — 11 октября 2026

Актуально на 07:42 UTC. Цель продолжается: проверяемая оптимизация реальных
программ от исходника через HIR/MIR до собранного Z80-бинарника. MIR VM —
промежуточный oracle; итоговые варианты исполняются в Z80-эмуляторе с проверкой
результатов, памяти, порядка ресурсных обращений, размера и T-states.
Настоящее железо остаётся следующим уровнем проверки.

## Завершено

- Исправления HIR/backend и opt-in PGO для sum_array, checksum, tokenizer и
  hashmap проверены на VM и actual Z80. История и отдельные ограничения
  сохранены в [handover](../docs/project/HANDOVER-2026-10-10-GRACE-PGO.md).
- Неизменённый Self-Lanz проходит закрытый memory-host контракт: ограниченные
  RAM-регионы, byte peek/poke, persistent calls и проверка частичного префикса
  до отказа. Его source-loop профиль привязан к исходнику, workload и сборке;
  загрузчик отклоняет несовместимые или неполные данные и повторяет исполнение.
- Реализованы отдельные arena-first и append-copy преобразования, совместный
  selector четырёх вариантов, typed precompile proofs и явный opt-in CLI.
  Решение использует фактические T-states/байты, сохраняет выбранные ASM/BIN.
- Последний полный pipeline: **569.037 s PASS**. После добавления CLI его
  общие тесты: **30.102 s PASS**. Showcase: **31 прежнее падение, 0 новых**.
  Последующие этапы меняли только workloads, артефакты и документацию;
  повторный полный pipeline для них не заявляется.

| Self-Lanz, 30 последовательных вызовов | T-states | Байты |
| --- | ---: | ---: |
| Original | 1221494 | 2091 |
| Arena-first | 1218395 | 2212 |
| Append-copy hoist | 1150124 | 2076 |
| Оба преобразования | 1147025 | 2197 |

Speed выбирает оба преобразования: **−6.10% T**. Size выбирает append-copy:
**−15 байт**. Production CLI/source: `b770f3be`; сохранённая сборка имеет
SHA256 `1bd95516f7b1ef1ba56c17822104936e2b73fe8bde5fe7368dd128811009f041`.

Дополнительный корпус: **4 workloads, 50 вызовов, 32 actual Z80 кандидата,
8 CLI решений**. Speed выиграл 1.62–13.60% в этих наборах; универсального
победителя не заявляем. Три коротких workloads действительно сменили speed
выбор на arena-only: **24 кандидата и 6 решений** проверены независимо.
Пять отказов CLI подтвердили unobserved site, typed host-denial prefixes и
отдельный gas limit; accepted selection и output directory не создавались.
Последний завершённый этап артефактов: `99f779cc`.

## Опубликовано и сохранено

Приватный Sites, подтверждённая **версия 45**:
https://minz-grace-hir-mir-report.grittysnipe3.chatgpt.site/

- [Совместный selector](https://minz-grace-hir-mir-report.grittysnipe3.chatgpt.site/memory-host-composition-selection.html)
- [CLI](https://minz-grace-hir-mir-report.grittysnipe3.chatgpt.site/memory-host-cli-selection.html)
- [Корпус workloads](https://minz-grace-hir-mir-report.grittysnipe3.chatgpt.site/memory-host-corpus.html)
- [Смена выбора и отказы](https://minz-grace-hir-mir-report.grittysnipe3.chatgpt.site/memory-host-choice-boundaries.html)

Весь сайт сохранён офлайн: [локальная главная](../artifacts/grace-reports/2026-10-10-resume/site/index.html).
Проверены **12757 файлов, 55 страниц**, SHA256 всех файлов, читаемость всех
публикационных архивов; потерянных ссылок и внешних зависимостей нет.
Исходники, профили, решения, proof records, ASM/BIN, логи, обе разные CLI
сборки и Git bundles сохранены локально.

## В работе

Отдельная проверка **supplied-profile CLI route** на неизменённой сборке:
один настоящий корректный профиль, настоящий профиль другой сборки и три
явно синтетические completion/counter mutations. Входы проверены двумя
независимыми ревью; ограниченный прогон выполняется. Проверяем конкретную
причину отказа и отсутствие accepted output. Это не экспорт настоящего
failed profile и не изменение production кода.

После закрытия этого шага продолжим профильное исследование доминирующих
parser циклов и выбор следующего доказуемого преобразования по actual cost.

## Сохранение веток и решения Alice

Сводка и завершённые этапы committed/pushed в
[feat/grace-hir-vm](https://github.com/oisee/minz/tree/feat/grace-hir-vm).
Нужные исторические /tmp ветки сохранены в remote `archive/2026-10-11/`;
точные SHA перечислены в [манифесте веток](2026-10-11-011-archived-branches.json).
Текущие ветки работы и ревью также committed/pushed; WIP не выдаётся за
закрытый этап. Активные рабочие каталоги находятся вне /tmp:
`/home/alice/dev/minz-artifacts/grace-workspaces/`.
Дополнительные bare repository и bundle — в `grace-checkpoints`.
Пользовательское изменение `minzc/cmd/mzx/BUILD` сохранено без изменений.

**Блокеров и решений Alice сейчас нет. Цель остаётся активной.**
