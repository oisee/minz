# HIR: известный callback должен соответствовать вызову потребителя

База `ba87d35e`, prerequisite original Lizp packet `24303423`
(локальный cherry-pick `e1e6a33d`). Собственный change — HIR verifier и
`consumed_callbacks.go`; runtime ABI и frontend syntax не расширены.

До исправления HIR принимал функцию `word(u16)->u16` как callback для
`apply(ptr,u8)->u8`. В apply косвенный вызов использует A input/result, а
word ожидает DE input/HL result. Probe(5) давал raw/final VM261, nativeA5;
это неподдержанное ABI-смешение, не утверждение желаемого u8 результата.
[Замороженный before](lizp-functional-callback-gap/manifest.json) не переписан.

Теперь pointer-параметр, реально вызываемый в CallIndirect, получает внутренний
consumed signature из точных типов аргументов и результата. Известная функция,
переданная такому параметру, должна совпадать по arity/Params/Returns через
sameHIRSignature; implicit width adapter не предполагается. Контракт переносится
через immutable pointer aliases и прямые forwarding wrappers до fixed point,
независимо от порядка функций. Несогласованные callsites одного параметра
отклоняются. Явная Signature не может скрыть иной actual argument width.

Не вызываемые data pointers остаются вне inference; truly anonymous raw pointers
остаются unresolved. Pointer mutation не выдаётся за доказанную alias identity.
Capture admission прежний. Известный FallibleCY callback явно отклоняется:
текущий CallSignature не несёт CY/error-return ABI, поэтому только совпадения
числовых Params/Returns недостаточно. StructReturnBuffers metadata — отдельный
координируемый ABI шаг; этот пакет не добавляет поля и не заявляет его завершённым.

Проверки:

- HIR units: good byte/word, wrong arity/argument width/return width/signedness,
  conflicting sites, pointer-alias forwarding, explicit-width mismatch,
  noncallback data pointer, anonymous raw pointer и fallibleCY rejection.
  [Focused PASS0.010s](consumed-callback-signatures/focused.txt).
- Exact Lizp mismatch отклонён **до ASM**, Assembly пуст;
  [сообщение](consumed-callback-signatures/refusal.txt).
- Explicit compatible word indirect callback проверяется VM/native с результатом261
  и restored SP. Это положительный контроль против blanket word-callback ban.
- Original unchanged functional.lizp повторно проверен: all8entries,
  raw/finalVM/native398595executions, actualindirect trace, PC/SP/fullRAM.
  [Retained gate PASS3.76s](consumed-callback-signatures/gate.txt).
- [Full HIR PASS29.305s](consumed-callback-signatures/full-hir-final.txt).
  Initial fullHIR FAIL27.406s был только diagnosticordering: existing malformed
  Signature arity guard ожидал слово arity, новый exactABIguard сообщил adapter.
  Исправлен порядок проверки, не ослаблено admission; первоначальный лог сохранён.

Retained pipeline.test SHA256
`c2929587ce1bc440a74fbd91b0f1fe5009d3f7c7c2482a6a95ff2f4df7fb8bfc`,
Go1.26.8. [Replay](consumed-callback-signatures/replay.txt),
[точные original-after source/ASM/BIN/MIR/ABI](consumed-callback-signatures/original-after/),
[SHA256](consumed-callback-signatures/manifest.json).
Исторический opt-in диагностический test теперь принимает явное callback refusal
с проверкой причины; его старый accepted-code/log пакет остаётся immutable.
Общий pipeline/CLI и неизвестные mutable callback lifetimes этим gate не доказаны.
