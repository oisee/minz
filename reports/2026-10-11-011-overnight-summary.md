# Ночной итог Grace / PGO — 11 октября 2026

Актуально на 09:00 UTC. Цель продолжается: оптимизировать реальные программы
по проверенным профилям, от исходника через HIR/MIR до собранного Z80-бинарника.
MIR VM служит промежуточным oracle. Итоговые варианты исполняются в
Z80-эмуляторе: проверяем результаты, память, порядок ресурсных обращений,
размер и T-states. Настоящее железо остаётся следующим уровнем проверки.

## Готово

- HIR-захваты и сигнатуры вызовов, iterator lowering, MIR/DJNZ и backend
  исправлены; sum_array, checksum, tokenizer и hashmap проверены на VM и Z80.
- Неизменённый Self-Lanz работает через ограниченный memory-host контракт.
  Профиль привязан к исходнику, workload и сборке; несовместимые и неполные
  данные отклоняются. Профиль проверяется повторным исполнением.
- Arena-first и append-copy, совместный selector четырёх вариантов,
  typed precompile proofs и явный opt-in CLI сохраняют выбранные ASM/BIN.
  Выбор использует фактические T-states и байты.
- Последний production этап: direct global u16→DE/BC loads, `2908f18f`.
  Полный pipeline **1107.431 s PASS**, HIR/MIR/Nanz/Lanz/C89/CLI — PASS.
  Showcase: **31 прежнее падение, 0 новых**. Последующие отчётные этапы
  production-код не меняли; повторный общий прогон для них не заявляется.

Текущая сборка CLI имеет SHA256
`dba34fdc489d4fa00f985bff82952076f8de8f68488999dfe98364af242c9ca5`.
Для 30 последовательных вызовов Self-Lanz:

| Вариант | T-states | Байты |
| --- | ---: | ---: |
| Original | 1135370 | 2000 |
| Arena-first | 1135211 | 2121 |
| Append-copy | 1079795 | 1978 |
| Оба преобразования | 1079636 | 2099 |

Speed выбирает оба преобразования, size — append-copy. Четыре неизменённых
workloads, 50 вызовов, 32 actual Z80 кандидата и восемь свежих CLI решений
прошли независимые проверки. В этом корпусе текущие speed и size выбирают
append-copy: arena-first добавляет 48–1651 такт. Универсального победителя
не заявляем. Старая сборка и её отличающиеся профили сохранены отдельно.

## Последний эксперимент

Изолированный normal-global diamond forwarding сохранил все 60 raw/native
наблюдений и 3017 упорядоченных host events, но оказался медленнее:
**1137339 T / 1998 B** вместо **1135370 T / 2000 B**.
Итог: **+1969 тактов, −2 байта**. Оба независимых аудита прошли.
Helper и диагностический hook остаются в отдельной ветке; production
не включает это преобразование. Артефакты и отрицательный результат сохранены.

## Опубликовано и локально сохранено

Приватный Sites, подтверждённая **версия 50**:
https://minz-grace-hir-mir-report.grittysnipe3.chatgpt.site/

- [Отрицательный diamond эксперимент](https://minz-grace-hir-mir-report.grittysnipe3.chatgpt.site/normal-global-diamond.html)
- [Обновлённый корпус](https://minz-grace-hir-mir-report.grittysnipe3.chatgpt.site/global-word-load-corpus.html)
- [Direct global word loads](https://minz-grace-hir-mir-report.grittysnipe3.chatgpt.site/global-word-load.html)
- [Совместный selector](https://minz-grace-hir-mir-report.grittysnipe3.chatgpt.site/memory-host-composition-selection.html)
- [CLI](https://minz-grace-hir-mir-report.grittysnipe3.chatgpt.site/memory-host-cli-selection.html)

Весь сайт, исходники, профили, решения, proof records, ASM/BIN, логи,
публикационные архивы и Git bundles сохраняются локально:
[офлайн-главная](../artifacts/grace-reports/2026-10-10-resume/site/index.html).
Полная копия версии 50 проверена: **14640 файлов, 60 страниц**, все SHA256
и публикационные архивы PASS. Потерянных ссылок и внешних зависимостей нет.

## Следующий шаг и сохранение работы

Три подагента готовят частный прототип unsigned u16 compare, независимый
review флагов/живых регистров и сквозной source→Z80 harness. Проверим,
может ли сравнение без разрушения пар устранить лишние PUSH/POP.
Локальная оценка стоимости пока не считается выигрышем всей программы.
Включение в основной компилятор возможно только после проверки и замеров.

Сводка и закрытые этапы committed/pushed в
[feat/grace-hir-vm](https://github.com/oisee/minz/tree/feat/grace-hir-vm).
Нужные исторические /tmp ветки сохранены в remote `archive/2026-10-11/`;
SHA перечислены в [манифесте](2026-10-11-011-archived-branches.json).
Рабочие ветки сохраняются commit/push; текущий WIP не объявляется закрытым.
Все активные рабочие каталоги находятся вне /tmp:
`/home/alice/dev/minz-artifacts/grace-workspaces/`.
Дополнительные bare repository и bundle — в `grace-checkpoints`.
Пользовательское изменение `minzc/cmd/mzx/BUILD` не тронуто.

**Блокеров и решений Alice сейчас нет. Цель остаётся активной.**
