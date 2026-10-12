# MinZ / Nanz: свежий checkpoint Grace и Z80

Состояние на 12 октября 2026 года, код до `14bf2612` в исследовательской ветке `feat/grace-hir-vm`. Общая цель ещё открыта. [Полный обзор компилятора](2026-10-11-092-grace-vm-z80-status.md) уже доступен из README в main; этот документ уточняет более поздние результаты, не объявляя экспериментальный код интегрированным в main.

Единый HIR → MIR2 → собранный Z80 уже даёт проверяемую пользу на нескольких программах. Самое важное сейчас — довести общий gate до конца и сохранить честные границы корпуса: compile/assemble, исполнение в VM, исполнение Z80, CP/M runtime и оборудование — разные уровни доказательств.

## Что изменилось в коде

- `9aa6d6d6`: CP/M runtime сохраняет канонический private digit helper, необходимый reachable decimal output adapter. Полные signed/unsigned 16-bit домены и оба режима BDOS clobber прошли; исключены неправильное pruning helper и искусственная недостижимость unsigned formatter в тесте. [Пакет](pascal-closure-pfcco-speed/README.md).
- `d3ded4ea`: строгие ожидания sum-cost приведены к уже действующей модели v5 после повторного независимого измерения. Speed в PFCCO/arena проверках выбирает меньшие такты даже при росте размера; Size проверяется отдельно. [Пакет](profile-cost-arena-v5/README.md).
- `20e80776`: маленькая native проверка десятичной печати теперь выполняется по умолчанию, без export-переменной. [Пакет](decimal-native-default/README.md).
- `85fd7531`: оба memory-host варианта исходного parser проверяются существующим независимым Go-оракулом, а не только raw HIR. [Пакет](parser-global-word-independent/README.md).
- `5183e540`: удаляются перезаписанные byte immediate loads внутри непрерывных цепочек LD r,n. Метки, память, пары, CALL и любые другие инструкции останавливают правило. 768 native state/flags случаев и полный MIR2 прошли. [Пакет](literal-overwrite-native/README.md).

## Что прошло после последней production правки

| Проверка | Результат | Доказательство |
|---|---|---|
| Полный MIR2 | PASS, 90,175 s | [Лог](literal-overwrite-native/literal-overwrite-mir2-full.txt) |
| HIR и 11 пакетов фронтендов/инструментов | Все 12 PASS | [Список](frontends-after-literal-overwrite/result.json) |
| Showcase compile/assemble | 193 PASS, 1 capacity SKIP, 10,871 s | [Точные статусы](showcase-cli-3ebed86e-checkpoint/showcase-result.json) |
| Полный CLI | 74 PASS, 0 FAIL, 0 SKIP; paired collect/replay PASS | [Результат](cli-after-literal-overwrite-terminal/result.json) |
| C89 effects, PL/M entry, Lizp crypto, Pascal records | Пять independent native gates PASS | [Лог](frontends-after-literal-overwrite/native.txt) |
| Parser/hashmap heldout | Поведение и польза сохраняются | [Пакет](heldout-after-literal-overwrite/README.md) |
| Таблицы × PFCCO × Grace | 12 вариантов PASS; exact miss/re-admission PASS | [Лог](heldout-after-literal-overwrite/table-combined.txt) |
| Защищённые CP/M main parser/tokenizer | Независимые stdout/memory/globals/stack checks PASS | [Пакет](cpm-main-after-literal-overwrite/README.md) |

Полный pipeline `5d90b7aa` всё ещё работает из неизменяемого бинарника. На момент записи он прошёл search lifecycle и native composition boundary lattice, но полного итога ещё нет. Он собран **до** literal-overwrite, поэтому даже его будущий PASS не будет автоматическим полным доказательством более позднего backend. [Команда, session/PID и SHA256](pipeline-5d90b7aa-checkpoint/job.json).

Предыдущий полный pipeline `350149ad` действительно завершился FAIL: 528 PASS, 6 FAIL, 29 SKIP. Найденные причины исправлены или уточнены отдельными пакетами; исходный FAIL сохранён. [Терминальный результат](pipeline-350149ad-terminal/result.json). Логи разных checkpoint не склеиваются в выдуманный общий PASS.

## Где есть измеренная польза

| Программа / режим | До → после, T-states | BIN, байты | Область вывода |
|---|---:|---:|---|
| Frozen self-Lanz parser, Speed heldout | 2 195 027 → 2 137 641 (−2,61%) | 2377 → 2478 | Прежний prospective протокол; selected BIN тот же на train/heldout |
| Hashmap, 48 heldout calls | 258 962 → 237 948 (−8,11%) | 996 → 976 | Speed и Size выбирают оба преобразования |
| Weighted caller с checked allocation table | 238 → 142 | Модуль 44 → 20 | Все byte inputs; пересылки и сохранения через CALL учтены |
| Self-tokenizer, PBQP Size, literal-overwrite | Здесь T не измерены | 5533 → 5513 | −20 B; greedy всё ещё меньше: 5465 B |

Исторические таблицы не закончились на «118 функциях». Свежая инвентаризация содержит 981 запись по 72 Nanz-исходникам; текущий checked bridge допускает 36 задач. Из 945 отказов 710 требуют single-result leaf block: рабочий механизм пока локальный, с явными opcode/ABI ограничениями. Это ограничение admission, не размер исторической ENRT базы. [Инвентаризация](table-inventory-5d90b7aa/README.md).

PBQP на tokenizer пока не лучше greedy по Size. Видимый рост связан с изменёнными homes и caller loads: после удаления 20 B остаётся проигрыш 48 B. [Точные ASM/BIN и attribution](tokenizer-contract-attribution-current/README.md). Нельзя объявлять PBQP глобально лучшим по одному успешному weighted примеру.

Защищённый CP/M runtime с резервом 1024 B выполняет исходный parser main за 419 878 T / 3984 B, tokenizer main за 411 987 T / 7051 B. На этих входах затронуто 46 и 30 B resident stack. Резерв не равен фактическому расходу; это абсолютные числа текущего runtime, а не дополнительный выигрыш Grace и не hardware benchmark.

## Что ещё не закрыто

Каталог сохраняет все 194 точных пути. 153 имеют независимое или ограниченное VM/Z80 покрытие, 21 — native ASM bounded, 10 — CP/M runtime, 2 — native SMC bounded. Отдельно остаются 5 Agon MOS runtime, 1 host-only memory model и 2 pending independent oracle. [Каталог](../docs/grace-integration-338/catalog.json). Эти категории не являются процентом готовности компилятора.

Два языковых вопроса требуют решения: будет ли Lizp defun возвращать последнюю форму или требовать explicit return; как исправлять учебный PL/M hello с пустым PRINTCHAR и чтением за точные literal bytes. Ни одна политика не выводится однозначно из текущей документации. [Lizp варианты и authority](lizp-return-policy/REPORT.md). Agon ADL products собираются, но отсутствует полноценная MOS runtime проверка; обычный Z80 mock её не заменяет. [Agon граница](agon-product-audit/README.md).

Живой LIR удалён, архив и перенос тестов сохранены; VIR остаётся исследовательским путём. [Проверка переноса](lir-test-transfer-completion/verification.json). Расширение таблиц на CFG/indirect islands не доказано текущими local hits.

Следующий шаг: завершить текущий pipeline, разобрать любой терминальный FAIL без перезапуска по тайм-ауту наблюдения; затем получить общий pipeline gate кода после literal-overwrite. Закрытие цели требует также явных решений по двум pending языковым случаям и проверки оставшихся обещанных runtime границ. Pure Speed сохраняет выбор минимума T без штрафа за байты; Size остаётся самостоятельной целью.

## Где смотреть и что сохранено

README в main: <https://github.com/oisee/minz#compiler-progress--october-2026>. Приватный сайт: <https://minz-grace-hir-mir-report.grittysnipe3.chatgpt.site/current-state.html>; опубликованный snapshot старше этой сводки.

Полная локальная копия сайта: `/home/alice/dev/minz-artifacts/grace-reports/2026-10-12/site/index.html`. Шара: `/mnt/safe/minz/grace/2026-10-12/site-and-artifacts/site/index.html`. Новые пакеты этого документа сохранены отдельно в `/mnt/safe/minz/grace/2026-10-12/compiler-updates/<commit>/`, бинарники тестов — в `compiler-checkpoints/`. Для каждого пакета есть manifest SHA256; parser heldout вложения исправлены свежим экспортом в `3ebed86e`, старые не выдаются за новый замер.
