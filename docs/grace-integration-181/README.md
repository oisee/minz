# Интеграция 181: полный CLI после проверки typed indirect ABI

Полный сохранённый бинарник CLI на `2612a266` завершился **PASS, exit 0**. Исполнялись все тестовые тела CLI, не только focused selection; прежние три expected-refusal/warning падения закрыты независимым native proof в176. Точный бинарник и SHA256 сохранены до запуска, окончательный журнал — рядом и на шаре. Длительность не заявляется: nonverbose binary её не записал.

Этот checkpoint содержит caller-owned structural ABI и typed indirect CLI, но предшествует VM OpCallCond, обновлению parser reference и последним corpus-тестам. Поэтому это не полный PASS самого нового дерева. Отдельный полный MIR2 после OpCallCond ещё выполняется; pipelinecfaa7777 остаётся live и уже имеет сохранённые reference mismatches, исправленные позже.
