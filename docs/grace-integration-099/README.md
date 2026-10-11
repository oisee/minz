# Проверка исправленных FAT12/FAT16 и индексного поиска

Общая ветка: production fixes `6616f083` (Grace flags safety), `3054481f` (live HL indexed shifts), `13d2acc0` (BIT admission); ENRT checkpoint index `b6058e46`. Эти изменения находятся в исследовательской `feat/grace-hir-vm`, не заявлены как GitHub main.

Root полный MIR2 PASS 11.473 с; полный VIR PASS 2.343 с. Исходный frozen FAT12/FAT16: все 108 вызовов в default/Grace Size/Grace Speed PASS 42.23 с. Проверка охватывает 36 случаев на режим, заранее сохранённые training/heldout входы, raw/final VM, независимые ожидаемые значения и наблюдения памяти, собранный native Z80, сохранность кода и SP. Это доказательство корректности этих случаев, не автоматический PGO selector winner.

Root original triple: индексный путь ENRT5 читает один блок; 256 входов прямого и косвенного вызова проходят. Полный модуль 34 → 31 байт, прямой вызов 66 → 54 T, косвенный 123 → 111 T. PASS 2.001 с. Отдельный обычный prefix-path прогон также проходит; объединённый FAT/prefix suite PASS 42.861 с.

Проверены SHA256 всех 49 файлов shift-proof, 126 flags-proof, 33 directory payloads и 25 файлов индексного архива. Пакеты скопированы в `/mnt/safe/minz/grace/2026-10-11/compiler-reports/`. Повторный directory caller имеет 13 574 T default/Size и 10 948 T Speed; его результаты получены агентом и сохранены отдельно, root в этом checkpoint не повторял directory запуск. Размеры текущего whole-module BIN: 9694/9871/9887 байт; прежние артефакты сохранены.

Общая цель активна. Контролируемый профиль packed FAT пока не допущен из-за правил unreachable extern; работа над этим отдельно. Композиция исторических форм требует проверки целого назначения и ещё не дала подтверждённого выигрыша. Published Sites остаётся v64; этот новый пакет сейчас локальный и на шаре.
