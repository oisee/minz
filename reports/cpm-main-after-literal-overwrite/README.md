# Защищённые CP/M main после literal-overwrite

Checkpoint e8a326fc содержит backend изменение 5183e540. Два исходных main проверены в production CP/M runtime с резервом стека 1024 байта и ContractOpt=true, AssertMode=none. Test-owned peek/poke реализуют ABI компилятора; BDOS host моделирует консоль и завершение. Это запуск в Z80 эмуляторе, не на аппаратуре.

Self-Lanz parser: **419 878 T / 3984 B**, stdout 135 байт; resident stack touched 46 B. Self-tokenizer: **411 987 T / 7051 B**, stdout 146 байт; touched 30 B. Независимый source-derived oracle сверяет stdout, memory regions и globals; проверяются runtime guards, SP и завершение. Точные source/ASM/BIN/capture/oracle сохранены рядом и сверены с логом. Это абсолютные результаты этой сборки; изменение скорости относительно прошлой сборки здесь не утверждается.

Повтор: `MINZ_ORIGINAL_LANZ_MAIN_ARTIFACTS=<parser> MINZ_ORIGINAL_TOKENIZER_MAIN_ARTIFACTS=<tokenizer> go test ./pkg/pipeline -run '^(TestCPMRuntimeOriginalSelfLanzMain|TestCPMRuntimeGuardedTokenizerMain)$' -count=1 -timeout=10m -v`, PASS 0,907 s. Резерв 1024 B не равен фактическому расходу стека и не доказывает расход на всех возможных входах.

Свежий полный CLI3ebed86e ещё работает; полный pipeline5d90b7aa работает на коде до literal-overwrite. Ни один текущий живой gate не объявлен PASS заранее.
