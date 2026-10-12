# Лимит исходного пула предложений до deduplication

CLI checked-proposals теперь проверяет исходный JSON array count до LoadJSON: повторные sig не могут скрыть превышение proposal-limit. Byte limit1MiB проверяется до JSON decoding. При ошибке output не публикуется. Raw duplicate negative control и реальная importer/compiler/native all256weighted цепочка PASS1,846с, выбранный BIN20байт.

API лимит текущего RegAllocTable по-прежнему ограничивает retained candidates; CLI дополнительно ограничивает исходный envelope. Production compilation table lookup не меняется. Полный core905b33e2 и retainedpipeline dd100284 ещё наблюдаются отдельно.
