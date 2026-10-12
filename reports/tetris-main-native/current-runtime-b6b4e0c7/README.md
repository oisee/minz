# Tetris CP/M: текущий допуск runtime, 12 октября 2026

На checkpoint `b6b4e0c7` повторный `TestOriginalTetrisCPMMainNative` завершился PASS за 2,553 s во всех трёх режимах: ordinary, Grace Size, Grace Speed.

Текущий тест заново разбирает исправленный `examples/nanz/tetris_cpm.nanz` и требует успешного CompileHIRSteps с RuntimeCPM и RuntimeStackBytes=1024. Поэтому старое утверждение REPORT.md о продолжающемся отказе публичного RuntimeCPM больше не описывает текущий код. Исторические freeze, negative payload и summaries оставлены без переписывания.

Запуск, проверяющий stdout, глобальное состояние, HALT и SP, всё ещё использует отдельный тестовый загрузчик CALL main и модели BDOS 1/2. Он выполняет настоящие импортированные ASM-функции. Это подтверждает игровой main на собранном Z80 и отдельный допуск production runtime, но не исполнение именно production COM и не интерактивную игру на настоящей CP/M. Следующий шаг — исполнить образ, выпущенный RuntimeCPM, с тем же независимым сценарием.

Команда из minzc: `GOCACHE=/tmp/minz-grace-gocache go test ./pkg/pipeline -run '^TestOriginalTetrisCPMMainNative$' -count=1 -v`. Использован Go 1.26.8. Полный текущий pipeline этим повтором не заменяется.
