# Интеграция 163: обязательные границы fixed indirect ABI

Независимый read-only разбор текущего struct-return WIP выявил три дополнительные границы. Это требования к реализации, не утверждение PASS.

1. Fixed ABI передаёт аргументы в canonical locations, затем однократный function-symbol prologue выполняет parallel copy в allocated homes, включая spills. Bridge внутри entry block неверен при backedge; hard pinning не гарантирует отсутствие spill. Direct calls к такой функции должны использовать тот же canonical contract, что typed indirect calls. Преобразования контрактов сохраняют derived attr даже после удаления исходного AddrOf.
2. `standardParamClass` прежде выбирал HL для каждого pointer независимо от позиции. Source word + hidden pointer, несколько pointer inputs или несколько return buffers могут конфликтовать. Требуется единое непересекающееся назначение на весь контракт, общие direct/indirect/entry mapping и oracle со смешанными word/pointer аргументами. Недостаточно исправить byte+один buffer случай.
3. Checked-local table candidate сейчас выводит body, не fixed-ABI prologue, и context привязан к Dump+Contract без FuncAttrs. Для fixed ABI кандидат должен учитывать bridge, attrs и полный cost; до этого он не допускается, обычный compiler остаётся fallback. Это исключение кандидата, не отказ компилировать функцию.

Владелец ABI получил точные findings. Новый focused WIP покрывает 30 frozen lifetime и 123 nested/recursive/indirect/loop/copy native cases в трёх режимах; root ещё не интегрировал этот WIP. Full core root0f0953d9 запущен отдельно, включает только завершённые callback/purity/directcall fixes.
