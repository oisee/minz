# Реальные heldout после literal-overwrite

Checkpoint `5183e540` включает новое удаление перезаписанных byte immediate loads. Повторены существующие prospective/parser и hashmap train/heldout протоколы без изменения входов или оракулов. Это регрессионная проверка прежних зафиксированных экспериментов, не новое слепое исследование.

Parser выбирает mask3 только на training: 1 041 370 → 1 013 143 T, 2377 → 2478 B. На отложенных входах исходный BIN 2 195 027 T, выбранный 2 137 641 T (−2,61%). Heldout использует побайтно тот же BIN выбранного кандидата; independent Go parser проверяет результаты, память и ordered events. Pure Speed вправе добавить 101 байт.

Hashmap выбирает mask3 в Speed и Size: training 26 346 → 23 437 T; полный BIN 996 → 976 B. Heldout 48 вызовов 258 962 → 237 948 T (−8,11%), независимые результаты/память и идентичность выбранного BIN проверены. Оба сценария PASS 25,871 s вместе.

Отдельно выполнена 12-вариантная матрица contract off/greedy/PBQP × Grace × checked table: прежняя польза weighted 44 → 20 B и caller 238 → 142 T сохраняется с независимым all-byte oracle. Проверка изменения константы и ABI подтверждает exact miss, отказ старому assignment и повторный допуск подходящей задачи; PASS 5,866 s. Исследовательский Z3 использован только для проверки таблиц, не как зависимость production.

Команды: `MINZ_HASHMAP_HELDOUT_EVIDENCE=<hashmap> go test ./pkg/pipeline -run '^(TestParserProspectiveHeldout|TestHashmapMeasuredTrainingAndUnseenWorkload)$' -count=1 -timeout=20m -v`; затем `VIR_TEST_Z3=<offline-z3> go test ./pkg/pipeline -run '^(TestCombinedContractTableGraceNative|TestCheckedExactBindingMissAndRevalidatedTransfer)$' -count=1 -timeout=10m -v`.

Источники, frozen inputs, профили, decisions, allocations, ASM/BIN и наблюдения рядом. Полный pipeline 5d90b7aa продолжает отдельный прогон до этой правки; его успех ещё не объявлен. Эти focused проверки не доказывают полный корпус нового checkpoint.

## Исправление пакета артефактов

Первоначальный коммит c081eaa7 ошибочно приложил старые parser файлы из docs к свежему PASS логу. Они заменены свежим экспортом из пустой папки после повторного PASS 24,122 s. Проверены decision selected=3, точные heldout T-states и SHA256 каждого baseline/selected BIN; устаревший heldout-mask-2 удалён. Для экспорта parser требуется `MINZ_PARSER_PROSPECTIVE_EVIDENCE=<output>`. Hashmap пакет первоначального запуска был экспортирован правильно.
