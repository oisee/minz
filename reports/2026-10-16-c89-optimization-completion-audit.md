# Аудит цели C89/Nanz: измеренная польза и интеграция

16 октября 2026 года. **Цель ещё активна: часть проверенных изменений ожидает main CI.**
Это список доказательств полной цели, а не переопределение цели под уже сделанное.

## Требования и доказательства

| Требование | Проверяемое свидетельство | Текущее состояние |
|---|---|---|
| Идентичный C, отдельный эквивалентный Nanz/tuple | Source manifests всех четырёх папок cases/nanz-cases/nanz-tuple/heldout совпадают с frozen SHA; driver проверяет их до/после | Подтверждено |
| MinZ против SDCC 4.2/4.5 и sccz80 | Полная матрица 69 строк; обе SDCC Size/Speed; 66 PASS, три сохранённых compile errors | Подтверждено |
| Размеры и такты на настоящих Z80-инструкциях | Assembler + Remogatto emulator; итог execute()/helper CALL/RET; code+owned data; результат/заданная память/PC/SP | Подтверждено на заданных нагрузках |
| Raw/final MIR VM и независимый C oracle | 94 488 primary VM calls; 75 350 primary native; GCC 6850 expectations | Подтверждено |
| Безопасные обычные MIR/Grace-преобразования | Range-proven byte compare/arithmetic, direct byte loads, whole abs CFG, guarded word stores; refusal/native boundary tests | Реализовано и проверено; word stores ещё в PR #181 |
| Структура → scalar input/tuple return | C ABI tests проверяют слоты и отсутствие лишней Alloca; lifetime/escape/volatile/import tests; отдельный G2 audit | C механизм подтверждён; bool-budget correction ещё в PR #182 |
| Out pointer → scalar returns | Проверенный C admission, caller+callee, сохранённые эффекты; три frozen источника/12 отказных форм | Main PR #177 |
| Польза минимум двух семейств | Неизменные abs_diff и sum_array; также другие primary families | Подтверждено |
| Дополнительные исходники/отложенные нагрузки | Независимые in_interval и count_less, их прежние source SHA и данные; свежий совмещённый native/raw/final/GCC повтор | Подтверждено |
| Проигрыши и границы | Все compiler profiles, failed struct builds, copy/FAT12 losses, greedy PFCCO loss и baseline fallback сохранены | Подтверждено |
| Итоговые код и отчёты в main | PR #173–180 интегрированы; #181–182 ещё открыты на момент аудита | **Не завершено** |

Все 13 primary исходников и их manifests побайтно совпадают с интегрированным
baseline b9ebf78d; оба heldout-исходника — с первым проверенным checkpoint 650ff119.
Проверены SHA всех 15 файлов. В опубликованных costs.json SHA всех 69 подробных
measurements.json совпадают с локальными файлами direct-word-store-full.

Полная [текущая таблица](2026-10-16-c89-nanz-full-benchmark.md) содержит исходники,
ASM каждой успешной строки, raw/final MIR MinZ и стоимости. Сопоставление языков
не подменено одинаковым C. У aggregate parameter mutation C и Nanz наблюдения
различаются — этот тест проверяет семантику, а не скорость языков.
G2 означает проверенный C механизм и аудит возможности typed-Nanz переноса;
автоматическая scalarization всех Nanz-структур не объявлена работающей.

## Совмещённый финальный кандидат

В отдельном persistent worktree проверена совокупность #181 и bool-budget
correction из #182, на основе уже интегрированных C/MIR изменений.
Compiler checkpoint **7fb342e5** (полный SHA и версии в toolchains).
Совокупность актуальных PR #181/#182 имеет ровно тот же Git tree minzc:
`76beedd4cb240214fc51b214248831e353548819`. Это побайтовое совпадение всего
compiler source/test tree, а не только списка изменённых production файлов.
Фактический main после слияния предстоит сверить отдельно.
Все 69 результатов **побайтно равны JSON-матрице** проверенного direct-store
checkpoint: размеры, T, Calls, source/workload SHA и статусы. Это независимый
повтор окончательного набора правил перед main, а не новая таблица победителей.

[Primary summary](c89-optimization-final-validation/primary-summary.json) ·
[Primary GCC oracle](c89-optimization-final-validation/primary-host.json) ·
[Toolchains](c89-optimization-final-validation/toolchains.json).
Подробные native observations/binaries/manifests сохранены локально в
/home/alice/dev/minz-artifacts/grace-reports/2026-10-16/c89-nanz-sdcc45/final-union-full/.

## Свежий heldout на той же совокупности правил

| Источник | Исходный baseline: байты / T | Совмещённый кандидат: байты / T |
|---|---:|---:|
| in_interval | 136 / 869 152 | **76 / 528 554** |
| count_less | 408 / 313 299 691 | **322 / 90 325 939** |

Default, Grace и Size совпали, все сохранены. Вместе выполнено **19 968 native
и 39 936 raw/final VM вызовов**. GCC-host повторно подтвердил все **6656 ожиданий**.
Исходники были заморожены до первого измерения, а новый повтор использовал
прежние данные; статистического подбора новых входов под новый pass нет.

[Heldout summary](c89-optimization-final-validation/heldout-summary.json) ·
[Heldout host](c89-optimization-final-validation/heldout-host.json) ·
[interval ASM](c89-optimization-final-validation/in_interval/program.a80) ·
[count_less ASM](c89-optimization-final-validation/count_less/program.a80) ·
[Манифест](c89-optimization-final-validation/manifest.json).

## Что требуется для закрытия

Дождаться реальных required checks и интегрировать #180/#181/#182, разрешая
конфликты отчётов без потери текущих чисел. Затем сверить фактический main с
проверенной совокупностью production changes и замороженными source manifests,
проверить публикуемые ссылки/артефакты, выполнить необходимую совместную проверку
на main и обновить этот аудит точными commit/CI links. До этого цель не завершена.

ULA contention, peak stack RAM, все программы компилятора, полный RAM trace и
универсальная победа над SDCC в требования этой benchmark-цели не подставлены.
Обычный compiler не зависит от исследовательских Z3/CUDA; чистый Speed остаётся
допустимым выбором независимо от размера. Deep release/nightly корпус не назван
PASS на основании быстрых checks.

## Совместные проверки текущего кандидата

Все четыре полных пакета прошли на совмещённом compiler tree: HIR 29,030 s,
MIR2 111,433 s, C89 62,079 s, Nanz 25,199 s. [Протокол](c89-optimization-final-validation/combined-core.log).

CI output-identity sweep проверил 844 исходных пути и 334 выходных файла.
Изменений exit/stdout/stderr нет; различия ASM ожидаются после изменения
codegen, поэтому этот advisory check остаётся FAILURE. Это не утверждение
о проверенном поведении всех 844 программ. [Сводка](c89-optimization-final-validation/output-sweep.json).
