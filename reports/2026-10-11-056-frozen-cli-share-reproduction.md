# Пользовательский replay со шары воспроизводится автономно

398865a8 сохраняет проверенный replay frozen CLI непосредственно из /mnt/safe/minz/grace/2026-10-11/compiler-checkpoints. Проверены запуск из worktree и автономный каталог с копией исходника по точному относительному source-unit пути. Оба воспроизводят training победителя и исполняют тот же52B BIN на12heldout calls за5568T без поиска. Это прежний эксперимент, не новая неизвестная нагрузка.

На шаре новый checkpoint manifest перечисляет все11 executable; предыдущий9-entrymanifest сохранён. SHA256 проверены276 payloads canonical/historical копий CLI-пакетов. FROZEN-CLI.md объясняет точные executable/SHA/sourcepath и команды. Каноническая документация находится в compiler-docs; исторические копии compiler-reports сохранены.

Все16 payloads аудита скопированы SHA256-проверенной копией в compiler-docs/grace-frozen-cli-share-audit. Полная новая сверка остальных repo docs/reports этим аудитом не заявляется. Полный pipeline-cc502-full выполняется; PFCCO model/adapters correction проверяется отдельно.
