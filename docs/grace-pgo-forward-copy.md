# Forward byte copy: проверяемый выбор PGO до Z80

Новый режим оптимизирует точный цикл `dst[i]=src[i]; i++` с нулевым u8 индексом, неизменными pointer/count параметрами и возвратом конечного индекса. Правило `hir-guarded-forward-copy` по умолчанию выключено. Быстрая ветка использует два последовательно продвигаемых указателя, обе проверки диапазона адресов и исходный indexed fallback. Перекрытие буферов разрешено: каждый байт читается перед записью, поэтому результат сохраняет последовательную семантику исходного цикла.

`MeasureForwardCopySiteVariantSets` и CLI `--hir-profile-forward-copy-sites --hir-profile-joint --hir-profile-auto DIR --hir-profile-auto-workload workload.json` измеряют до четырёх наблюдавшихся source sites, до16 кандидатов. Режим несовместим с другими режимами выбора правил. Профиль v5 определяет наблюдавшиеся циклы; его counts проверяются повторным выполнением исходного workload. Новая evidence schema `minz.forward-copy-site-variant-sets/v1` хранит фактические бинарники и стоимость полного workload. Guard summary и экстраполяция стоимости не используются. Отдельные decisions сохраняют выбранные copy sites, proof и SHA256 бинарника.

Независимый структурный validator до и после compilation проверяет исходную форму, metadata/globals/target, обе границы, свежие временные имена, явное чтение перед store, единичное продвижение указателей, exact fallback, конечный индекс и lineage. Неисполненные подходящие циклы остаются исходными. Повторно используемые или изменённые factory inputs, предыдущие policies, поддельный профиль и превышение лимита отклоняются.

Caller воспроизводит typed token-allocation pattern: заголовок длины, копирование, persistent appended/arena_used и их переполнение. Это отдельный typed port паттерна self_tokenizer, а не утверждение о проверке неизменённого tokenizer с его extern/ASM. 30 последовательных вызовов покрывают пять шаблонов памяти, включая overlap, и длины0/1/5/16/64/255. Каждый кандидат совпадает с независимым host oracle по результатам, состоянию и точному числу чтений/записей.

На этом workload исходный вариант: **388885 T / 86 B**; pointer вариант: **283030 T / 221 B**. Режим скорости выбирает ускорение27,2%; режим размера сохраняет исходный вариант. Собранный CLI выбирает те же бинарники. Это измерения Z80 emulator, hardware проверки ещё впереди.

Проверка исходного варианта обнаружила backend ошибку: byte store через A уничтожала живой индекс, также размещённый в A. Backend теперь сохраняет AF в затрагивающих A store paths, с правильным порядком восстановления AF/HL. Диагностика ошибочного baseline сохранена отдельно; сравнительные измерения сделаны после исправления обоих вариантов.

Raw VM domain:8192 combinations, все256 длин, heaps16/65536, alias и граничные адреса, первый trap и полное частичное состояние памяти. Дополнительно64 исходных/оптимизированных Z80 boundary cases и38 машинных store regressions проверяют backend. [Артефакты и независимый аудит](grace-pgo-forward-copy-artifacts/README.md).

Полный gate первоначально выявил устаревшую предпосылку budget fixture: после weighted v4 оба его цикла стали eligible для pointer walk. В тестовом исходнике переставлены две чистые modular additions (byte contribution перед invariant weight), чтобы оставить исходную scalar budget competition вне узкого weighted pointer proof. Host arithmetic/access oracle и ожидаемые hoist counts сохранены. Исторический неуспешный gate сохранён; окончательная проверка проводится заново.

Финальный полный pipeline, включая budget competition: **PASS722.080s**. HIR/MIR2/Nanz7.766/3.887/1.633s; profileCLI17.723s; дополнительный target policy refusal0.291s. Showcase31existingfailures0new. Независимый аудит720caller results и4Go evidence SHA256 прошёл.
