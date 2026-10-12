# Полный pipeline 350149ad: терминальный результат

Процесс PID 2860836 / session 13077 завершился с exit 1. Полный неизменяемый бинарник снимка `350149ad`, аргументы `-test.count=1 -test.timeout=180m -test.v`, cwd `minzc/pkg/pipeline`. Прогон занял примерно 2 часа 41 минуту; timeout не сработал.

528 верхнеуровневых тестов PASS, 6 FAIL, 29 SKIP. Пропуски перечислены в result.json и полном логе: opt-in экспорт, исторические proposal-файлы или offline Z3. Этот прогон не доказывает их выполнение.

Все шесть падений разобраны после обнаружения:

- PascalCPMMainAdmission, PascalHelloProductionCPM: runtime closure удалял частный digit helper, вызываемый сертифицированным ASM. Исправлено 9aa6d6d6; повтор main во всех режимах PASS.
- PascalDecimalProductionFullDomain: то же удаление helper; после исправления выявлена ошибка стенда, вызывавшего удалённый недостижимый WriteU16. Реальный вызов из main восстановил область теста. 262144 вызова полного signed/unsigned диапазона PASS, 9aa6d6d6.
- PFCCOMeasuredNativePacket: устаревший штраф за размер в Speed-тесте. PBQP 7735 T / 53 B против 8857 T / 52 B. Pure Speed проверка и frozen heldout PASS, 9aa6d6d6.
- MeasuredContractStrategiesOriginalArena: устаревшее ожидание ничьей. Size off 1121 B; Speed PBQP 3766 T / 1122 B. Frozen heldout без дополнительного выигрыша. Повтор PASS, d3ded4ea.
- SumProfileCostFullU8Domain: старые costs против уже существующей модели v5. Строгий повтор 1022 адреса/длины с независимым oracle VM/Z80 PASS, d3ded4ea.

[Повторы Pascal/PFCCO](../pascal-closure-pfcco-speed/README.md) · [Повторы sum-array/arena](../profile-cost-arena-v5/README.md).

Шесть исходных FAIL сохраняются. Нельзя называть этот прогон PASS для нового HEAD: последующие изменения не входят в его бинарник. Корпусные VM/Z80 проверки, heldout и transfer-проверки, исполнившиеся в этом снимке, отражены в полном логе. Общая цель требует отдельного итогового аудита и закрытия оставшихся семантических границ.
