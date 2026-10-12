# Tetris: исполнение штатного CP/M COM, 12 октября 2026

Свежий TestPublicCPMTetrisNative на коде b6b4e0c7 (HEAD 69ce1022 содержит только документационное уточнение) завершился PASS за 1,742 s. Исполнен именно COM, собранный CompileHIRSteps с RuntimeCPM, RuntimeStackBytes=1024, ContractOpt=true, AssertMode=none; ASM не заменён тестовым игровым загрузчиком.

| Режим | COM, B | T-states | Вывод, B | Результат |
|---|---:|---:|---:|---|
| ordinary | 5796 | 168311 | 326 | PASS |
| grace-size | 5800 | 168263 | 326 | PASS |
| grace-speed | 5800 | 168263 | 326 | PASS |

Независимый сценарий z,q,a проверяет два кадра, поворот, выход и подтверждение. Сверяются все globals, board, точные ANSI/stdout bytes, вся RAM за исключением фактически использованного resident стека, OS call-return word, runtime saved SP и именованных spill slots. Runtime status=0, исходный SP=0xf000 восстановлен, управление возвращено OS harness на 0x7003. BDOS 1/2 моделируются; на каждом вызове намеренно портятся flags, BC/DE/HL/IX/IY. Остальные вызовы и чужие instruction fetch отклоняются.

Это подтверждение штатного защищённого COM в Z80-эмуляторе с явно связанным BDOS, не интерактивная сессия в установленной CP/M и не аппаратный замер. Большая family матрица отдельно подтверждает механику; короткий main сценарий не доказывает любую последовательность пользовательских действий.

Предыдущее уточнение current-runtime-b6b4e0c7 было слишком осторожным: оно проверяло только admission плюс тестовый main loader. Более сильный TestPublicCPMTetrisNative уже существовал и прошёл также в полном pipeline-5d90b7aa. Этот пакет сохраняет свежий повтор и его фактические COM/ASM/состояния. Старые historical refusal документы остаются историей.

Команда из minzc: MINZ_CPM_TETRIS_ARTIFACTS=<export-directory> GOCACHE=/tmp/minz-grace-gocache go test ./pkg/pipeline -run '^TestPublicCPMTetrisNative$' -count=1 -v (Go 1.26.8).
