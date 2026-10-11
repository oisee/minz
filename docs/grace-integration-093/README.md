# Убрана повторная compatibility в одной границе допуска

Snapshot deefce9a. Старые public API сохранены; combined helper возвращает
ключ, который уже вычислен для проверки того же context. Девять adjacent
pipeline пар используют этот ключ без повторного semantic fingerprint.
Ни одна callback/mutation boundary не удалена; executable SHA вычисляется
заново на каждом вызове, общего cache нет.

Root: полный HIR PASS28.209с; equivalence/ownership/refusal PASS0.055с;
существующие late-factory mutation и preflight проверки pipeline PASS20.574с.
Первый общий focused лог сообщает HIR «no tests to run» из-за regex;
отдельный equivalence лог явно выполняет требуемый тест с пятью negatives.
Он не подменяется этим пустым первым результатом.

[Замеры агента](../grace-profile-context-once.md): короткая нагрузка
21.338→17.797с; selector18.389→15.828с. Это одиночные wall observations
на двух snapshot, не полный CLI benchmark. Повторное хеширование executable
сохраняется. Полный CLI71/71 PASS279.75с предшествует этому изменению.

Полный pipeline3e143f9f всё ещё выполняется тем же процессом;
его пустой buffered log не является ни PASS, ни завершением.
Общая цель остаётся активной, в том числе реальные train/heldout workloads
и дальнейший перенос shape assignments.
