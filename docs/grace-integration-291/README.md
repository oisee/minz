# Сохранённые associations перед extern ABI fix

Checkpointef077bd2. Все TestMemoryHostPolicyAssociation* повторно PASS5,158с: canonical policy/direct correspondence, strict envelope, fresh closed replay refusal для подменённых counts/owned-input drift и preflight. ABI пока прежний, это baseline для сравнения после pendingfix. Никакие inputs/expected не менялись.

Old fullpipeline1f2365be остаётся live. Он сообщил три числовых parserref drift и старый Tetris sourcefreeze mismatch; текущие explicit baselines/historicalsource snapshots уже проверены отдельно. Старый FAIL не переписан. Новый targetCLI focused gate запущен отдельно после publicCPM runtime integration.

Focused CLI FAILED0,057с на regression: runtime closure заранее отклонял поддержанные peek/poke. Исправление возвращает вызов существующего validateCPMBytePrimitive, не допускает unknown/fixedextern. Все пять CLI tests послеfix PASS0,066с. BeforeFAIL сохранён.
