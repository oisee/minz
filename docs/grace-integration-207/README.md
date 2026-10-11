# C89 switch effects и полный прогон фронтендов

На fc8afa44 полный C89/Frill/ABAP/Lanz/Lizp/Pascal запуск выявил два разных сбоя: C89 FatFS unknown __post_1 при HIR lowering и отсутствующий npm @abaplint/core в ABAP bridge. Frill/Lanz/Lizp/Pascal PASS. npm ci по существующему lockfile восстановил ABAP: полный пакет PASS2,290 с; lockfile/source не изменены.

Минимальная независимая C89 проба switch-arm postfix воспроизводила неверное значение: probe(0,0) возвращал512 вместо256. Pending effects разных case arms переносились наружу и выполнялись до выбора. Теперь селекторные effects предшествуют switch, а эффекты каждого исходного statement остаются в его arm. Для пустого switch селекторные effects тоже сохраняются.

Raw/final VM exhaustive selectors256 ×6 byte boundaries и native active via-z80 assertions в ordinary/Grace Speed/Grace Size проверяют selected-only postfix/prefix ordering и bytewrap. Дополнительно selector n++ с пустым и непустым switch проверен all256 VM плюс native boundaries. Actual FatFS_VM_Verify проходит вместе с регрессиями. Это не доказательство произвольного C switch fallthrough: прежнее documented frontend ограничение сохраняется.

Полный C89 после arm correction PASS, точное время в c89-full-switch-fix.txt; последняя empty-selector regression прошла focused вместе с FatFS. Полный C89 после самой последней правки будет отдельным gate. Старый полный pipeline29df08bb не содержит этого C89 frontend изменения и продолжает выполняться.

Общий docs-only отчёт и README feature merged в main через PR74. Исследовательский compiler этой публикацией не merged.
