# Интеграция138: CLI terminal и native-bound cost assertion

Полный CLI checkpoint c69e350e завершился **FAIL351,983 с**, одно падение TestHIRProfileAutoMeasuredCompilation. [Точный terminal лог](cli-terminal-fail.txt). Тест фиксировал исторические774/395T, а исправленный backend измеряет1024/445T; pointer-walk остаётся быстрее. Нельзя объявлять fullCLI PASS из остальных завершённых тестов.

Проверка теперь сравнивает decision totals с точной native evidence row той же длины5 и fast base0x4000, требует единственную строку, execution digests и строго меньшиеPointerT; сохраняет полный1022-case oracle измерительного API, selected/replay ASMidentity и refusal другой конфигурации. Это проверка связи выбранного решения с текущим native output, а не новая пара жёстко вписанных цифр. **FocusedPASS19,153 с**. [Лог](cli-focused-pass.txt). Новый fullCLI будет выполнен на стабильном corrected checkpoint после текущего widemath backend исправления.

Site v67 published succeeded; первое копирование на share завершилось ошибкой metadata прав трёх.gitignore. Повторный процесс проверяет contentSHA и копирует только отличающиеся payloads; completion manifest публикуется после полной проверки. Ошибка копирования не влияет на успешную публикацию. Pipeline a7954042 по-прежнему жив; не перезапускается по таймауту наблюдения.
