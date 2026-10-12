# Воспроизводимый пакет повторного допуска таблицы

Неизменённый source.nanz соединяет два sum helper/caller с add5. Исходный exact certificate от отдельного add5 не подходит текущим MIR identities. Bounded API заново допускает прежнее dense назначение на текущем MIR/ABI; новый JSON сохранён, загружен в другой RegAllocTable и даёт побайтно тот же текст функции.

Обычный Size pipeline выбирает readmitted add5. Все256входов собранного Z80 проверены по независимой формуле byte(x+5) и восстановленному SP. native-observations.json содержит каждый вход, ожидаемый результат и TStatesWithHarness; это такты CALL/boot/stop harness, не стоимость чистого тела функции. PASS0,620с. selected.bin — полный модуль125байт, не только add5.

Пакет executed содержит точный source, baseline MIR/ASM, исходную и readmitted таблицы, selected ASM/BIN и observations. Связанные profiles и решения совместного поиска сохранены отдельно в reports/joint-profile-pinned-table; этот опыт не объявляет новую profile/heldout проверку. Manifest связывает каждый файл SHA256. На шаре: /mnt/safe/minz/grace/2026-10-12/compiler-reports/table-readmission-artifacts/.

Повтор: cd minzc; MINZ_TABLE_REBIND_EXPORT=/absolute/output go test ./pkg/pipeline -run '^TestCheckedTableModuleRelocationRequiresReadmission$' -count=1 -v. Go1.26.8, текущий branch feat/grace-hir-vm. Общий production lookup остаётся точным; offline readmission не доказательство произвольных CFG/calls/islands.
