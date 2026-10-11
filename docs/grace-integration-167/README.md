# Интеграция 167: distinct return ABI и cross-result overlap

Root161e47b2/bcec5de4 интегрируют5074763d/ab7a0884: единое deterministic непересекающееся назначение canonical indirect return slots для definition/caller. Копирование нескольких возвращаемых объектов сначала читает все source slots, затем записывает назначения; исключает повреждение следующего source при взаимном overlap.

Root gate **PASS** HIR0,609с/MIR2 0,018с/pipeline0,346с. Включены реальные VM+assembled Z80 cross-slot alias случаи с полной64KiB RAM и wrapffff, hiddenbuffers/PC/SP/code; canonicalparam/return metadata checks; прежний HIR indirect regression теперь проверяет actual native go(3,1000,9)=5667 вместо устаревшего expected refusal. Остальные struct lifetime/recursive/loop cases остаются.

Original Widemath memory observer failure ещё расследуется отдельно, поэтому current generalABI/fullcorpusPASS не заявляем. Public profile/object adapters и исчерпывающая nested layout/full gate остаются в scope. FullCLI568b7d08 ещё running, previous checkpoint.
