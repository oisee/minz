# Удаление недостижимой старой модели ZX font

После подключения frozen whole-main oracle в80115d4f удалена obsolete ветка case2: прежний u16font parser, screen-only модель и её недостижимое RAM исключение. Source2 по-прежнему выполняет TestOriginalZSQLMaraColorMainNative во всех3режимах с полным760bytefont, точным source/text freeze и fullRAM proof. Общий TestOriginalZXAppsNative PASS3,041с; каталог evidence SHA обновлён после replay и проходит.

Изменение test-only, backend/source не менялись. Старый terminal panic сохранён в pipeline-94a41120-terminal. Native coverage не заменена отсутствием panic: реальный full-main oracle выполняется. Retaineddd100284 продолжает свой старый executable.
