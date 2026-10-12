# Историческая hash3 proposal на текущем backend

Opt-in TestCompositionExactHashNative повторён на checkpoint 1ab9527b с сохранённой hash3-proposal-table.json, скопированной в новый каталог. Native baseline и вставленный table candidate прошли по 65536 bounded tuples каждый; математический byte oracle и final MIR VM согласны, exact PC/SP и неизменность emitted code проверены. Оба пути имеют hash3 6 B / 30 T и полный модуль 45 B: нового выигрыша нет, кандидат равен baseline.

Команда: COMPOSITION_EXPORT=<fresh output with copied proposal> GOCACHE=/tmp/minz-grace-gocache go test ./pkg/pipeline -run '^TestCompositionExactHashNative$' -count=1 -timeout=10m -v. PASS, exit 0. Это fixture experiment с отдельной заменой функции после admission; не новый production selector или полный трёхбайтовый Cartesian. Сохранены input proposal, исходник, тест, два ASM и лог. Большой pipeline пропускает этот экспорт без COMPOSITION_EXPORT; повтор выполнен отдельно.

Повтор экспортера после добавления сохранения BIN завершился PASS 0,127 s. Теперь пакет содержит именно собранные и исполненные baseline/candidate BIN по 45 B, allocation/options/result JSON и новый test source composition_export_test.go. Исторические source/log сохранены. Экспортер проверяет ошибки записи.
