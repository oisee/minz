# CLI: SHA256 проверенной таблицы

Новый флаг --checked-local-table-sha256 передаёт точный digest JSON в pipeline.Options. Используется вместе с --checked-local-table и --Osize. При measured HIR profile replay content pin обязателен; ordinary compilation может оставить прежний неприкреплённый режим. Это публичный CLI доступ к уже проверенной pipeline функции, без изменения алгоритма распределения регистров.

Реальный собранный mz успешно скомпилировал arithmetic source с закреплённой таблицей. SHA без пути отвергнут; после замены JSON на [] старый SHA отвергнут как mismatch. Сохранены exact commands, source/table и итоговый ASM. Runtime CP/M и compiler-mode search/replay продолжают отвергать такое сочетание; новые negative controls и прежние admission tests PASS. Полный CLI не запускался для этой малой правки.

Пример:

```sh
mz source.nanz --Osize --asserts none --checked-local-table table.json --checked-local-table-sha256 SHA256 -o out.a80
```

Команды и файлы сохранены на шаре в compiler-reports/cli-checked-table-pin/executed. Для measurement/replay профиль должен быть собран тем же executable и закреплён теми же bytes таблицы; профиль из Go test binary нельзя переносить в CLI executable. Pure Speed и offline Z3 политика не меняются.
