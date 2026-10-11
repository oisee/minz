# Эмулятор отделяет запись данных от исполнения RST

В 64f3da6d исправлена связь screen SMC tracker→RST hooks. Обычный store старого DF/E7 мог неправомерно вызвать input callback и портить unrelated RAM ffff через SetRegisterA. Теперь hooks вызываются только для фактически fetched unprefixed RST, а SetRegisterA пишет настоящий A. Prefix DD/FD сохраняют обычную CPU-семантику; Agon vector handler отдельный.

Handled host-ROM abstraction явно 11 T, PC+1, SP/F сохраняются; DF/E7 могут вернуть A. Unhandled RST исполняет настоящий CPU push/vector. Это не моделирование полной ZX ROM. Независимые tests сверяют все 8 RST старых данных, весь register/state/ffff/code/SP, Step/Run handled/unhandled и patchedopcode. После интеграции полный emulator PASS 0,050 с. Реальные parser/tokenizer/mapInPlace/profile проверки выполняются отдельно; старые freeze/captures не переписаны.

Пакет docs/grace-emulator-rst-execution SHA256 проверен на канонической шаре, лог сохранён в docs/grace-integration-062. Большой pipeline-cc502-full и новый CLI-c784-full продолжаются; terminal PASS не заявляется.
