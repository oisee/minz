# Старый Tetris: runtime admission и ошибка source API разделены

Полный94a41120 сообщил obsolete assertion: TestOriginalTetrisCPMMainSourceAPIRefusal ожидал отказ любого ASM. Текущий CP/M runtime уже допускает строго canonical TUI bodies/ABI (cpm_runtime_tui.go). Исправленное публичное source имеет отдельный native whole-main proof; runtime не должен снова отвергать эти тела.

Старый frozen исходник по-прежнему имеет несоответствие SString/NUL puts. Диагностический тест теперь проверяет успешную compilation admission, затем сохраняет прежний bounded native источник/API mismatch во всех3режимах; он не выдаётся за успешное выполнение старой программы. Исправленный публичный Tetris отдельно проходит output/state/OSreturn/SP. Совместно PASS4,958с.

Adversarial canonical-hostile tests PASS2,983с: hidden ASM, extern, неверные physicalhome/kind и остальные изменения шаблонов отвергаются. Production code не меняется: обновлено только устаревшее ожидание теста после native и hostile proof. Старый terminal full FAIL сохранён в pipeline-94a41120-terminal, новый retaineddd100284 продолжает выполнять старый executable; его результат остаётся самостоятельным.
