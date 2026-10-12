# Пять прежних frozen heldout после emitter исправлений

На source deb3f675 повторены ParserProspectiveHeldout, HashmapMeasuredTrainingAndUnseenWorkload, PFCCOProspectiveHeldout, PascalBubbleSortProspectiveHeldout и ArithmeticProspectiveHeldout. Frozen training/heldout и независимые оракулы не менялись. Terminal log рядом.

Это повтор прежних отложенных входов после byte-cycle/rotate изменений, не новые prospective входы для этих правок. Arithmetic сохраняет прежний отрицательный результат выбора. Полный pipeline1f2365be идёт отдельно и сообщил два parser числовых reference drift; forensic ещё не интегрирован.
