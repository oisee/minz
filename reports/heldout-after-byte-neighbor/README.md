# Heldout после сохранения соседнего байта пары

Checkpoint214063cd включает production div/mod live-byte preservation и research VIR fixes. Frozen parser/hashmap протоколы повторены без изменения training/heldout входов и независимых оракулов. PASS22.580s, exit0.

Parser train mask3:1041370→1013143T,2377→2478B; heldout2195027→2137641T (−2.61%). Hashmap Speed/Size mask3:26346→23437T training,996→976B;48 heldout calls258962→237948T (−8.11%). Одинаковый selectedBIN на train/heldout проверен тестом. Выигрыши и стоимости совпадают с прежним checkpoint; это повтор после исправления backend, не новое слепое исследование.

Fresh экспорт создан в новой пустой папке: исходники, inputs, raw models, decisions, actual ASM/BIN и cost evidence рядом. PureSpeed может добавить101B parser; Size отдельная цель. Полныйpipeline b6b4e0c7 предшествует позднимproduction изменениям и ещё выполняется, общегоPASS новогоcheckpoint нет.

Команда: MINZ_PARSER_PROSPECTIVE_EVIDENCE=<parser> MINZ_HASHMAP_HELDOUT_EVIDENCE=<hashmap> GOCACHE=/tmp/minz-grace-gocache go test ./pkg/pipeline -run '^(TestParserProspectiveHeldout|TestHashmapMeasuredTrainingAndUnseenWorkload)$' -count=1 -timeout=20m -v.
