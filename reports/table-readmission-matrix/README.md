# Повторный допуск таблицы × контракты × Grace

Один saved weighted assignment заново допускается для каждого текущего MIR/ABI. Матрица off/greedy/pbqp contracts × Graceoff/on × tableoff/on:12сборок без внешнего solver. Каждая пара baseline/selected проверена all256caller inputs независимой формулой byte(11*x+1), MIR VM и собранным Z80 с фактическим ABI/SP. Все6пар:44→20байт,caller256total60928→36352T (238→142на вызов). PASS3,129с.

PFCCO/Grace не добавляют выигрыша на этом конкретном weighted helper. Эффекты не объявляются универсально аддитивными. Новое API повторного допуска совместимо с текущими режимами; caller остаётся обычным call backend и его moves/saves включены в измерение. Это повтор прежнего корпусного примера, не новый heldout.

Тест minzc/pkg/pipeline/checked_local_readmission_weighted_test.go; input table docs/grace-real-leaf-local-tables/caller-table.json. Подготовка используется без Z3 и тест не skip при отсутствии solver. Остальные semantic/runtime gaps корпуса сохраняются.
