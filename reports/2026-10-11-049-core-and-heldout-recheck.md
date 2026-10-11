# Повторная проверка основных пакетов и heldout

На исходном checkpoint 9bdb26c7 полные пакеты HIR, MIR2, Nanz и VIR прошли: 25,197 / 6,505 / 11,623 / 2,097 с. Компиляция поддерживаемых cmd, pkg и internal пакетов прошла с фильтром без запуска тестов. Это не проверка standalone scripts и не полный тест всех пакетов.

Отдельно повторены четыре заранее зафиксированных эксперимента: ParserProspectiveHeldout, HashmapMeasuredTrainingAndUnseenWorkload, PFCCOProspectiveHeldout и ArithmeticProspectiveHeldout. PASS за 25,483 с. Сохранены прежние source/workload freezes и независимые семантические оракулы. Арифметическое семейство сохраняет отрицательный результат для Grace: выбор original — корректный исход, не выигрыш преобразования.

Большой pipeline на cc502270 продолжает работать в исходном процессе. Его запуск предшествует ещё готовящемуся исправлению implicit flag boundary byte packing; PASS не заявляется. Дополнительный аудит и whole-main tokenizer работают отдельно.

Логи: docs/grace-integration-049. Все логи и эта сводка скопированы на /mnt/safe/minz/grace/2026-10-11/. Публикация Sites v61 имеет более ранний source checkpoint; текущие проверки не приписываются той публикации.
