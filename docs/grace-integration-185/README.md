# Интеграция 185: Grace сохраняет значения рёбер

Интегрирован 46ced4a1 (donor1c72b5cf). Одинаковый target не разрешает удалить BrIf с разными ThenArgs/ElseArgs. Predicate и action проверяют точный порядок и длину аргументов; nil/empty без аргументов эквивалентны. Root независимо повторил low-level controls и originalmax counterexample: MIR2PASS0,009с/pipelinePASS0,013с. Before raw219/final36/native36 сохранён; после все219.

Агентский полный MIR2 прошёл70,708с. Исходные LUT/forEach/mapInPlace прошли590592nativeCALL с независимыми raw/final моделями и порядком чтений/записей во всех трёх режимах. Root проверил все77 manifest payload локально и на canonicalshare, отдельно запустил полный исходный replay, который пока выполняется.

Каталог сохраняет194пути:71independent/bounded,66pending. Два March10 iterator paths уже имели прежнее независимое покрытие; новые доказательства расширяют ихscope, не увеличивают countдважды. Четыре раньшеpendingpaths повышены по точнымsourceSHA. Domainarrays bounded, не все массивы. Старые fullpipeline/CLI checkpoints не становятся текущимfullsuitePASS.
