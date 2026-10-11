# Первый packed FAT12 backend fix; следующий mismatch сохранён

Snapshot13d2acc0. BIT comparison matcher теперь пропускает оригинальные
AND/shift/zero definitions только если выбранная physical BIT форма кодируется.
Fallback на IX/IY/spill сохраняет исходные вычисления. Root BIT tests
PASS3.553с, включая1536 actual native parity executions.

Полный frozen FAT packed native test теперь проходит even case0,
но **FAIL4.179с** на odd case1: write_fat12 cluster9/base17408/value3445,
ожидаемый packed word55130, наблюдаемый low byte80. Raw/finalVM совпадают
с независимым oracle. Следующая причина: constant shift в IX/IY черезHL
затирает живойHL. Исправление расследуется отдельно; первый BIT fix
не объявляется полным закрытием FAT packed correctness.

Training и heldout workload bytes frozen ранее в bfff57c9/57ece24f.
Они не изменяются ради получения PASS. Public controlled selector пока
отказывает на недостижимом disk_read extern всего исходника; это отдельный
admission gap. Нет partialwinner или доказанного нового heldout выигрыша.
Все59 payload исходного diagnostic packet root SHA-проверены и сохранены
на шаре. ExactFAIL остаётся воспроизводимым новым regression test.
