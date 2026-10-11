# Frozen source release 1f85d740

Независимый readonly review закрыт: разрешён один actual full30 эксперимент владельцу через неизменённый closed runner. Production matcher/emitter не изменялись после a149. Четыре owned файла: два production, MIR2 machine regression, pipeline ED recipe regression. Родительский compiler не изменён этим branch.

Дополнительный actual test создаёт старую cache-связь A↔E/C, загружает direct word в DE/BC, затем trunc low byte в A, store в low_out и возвращает word. Десять значений проходят, выявляя отсутствие pair-byte invalidation. Inline-asm address consumer учитывается Inst.Uses. Numeric/shared/successor consumers не оптимизируются. Fetch-only исключение в access audit ограничено четырьмя байтами actual instruction; вся другая память проверяется, включая обе global bytes.

Closed ED4B/ED5B recipe tests проверяют low/high pending budget consumption, 20T, AF/HL, little-endian value. Forged first address, width1 и wrong direction отказывают до execution; generic permission не добавлена. Это tests существующего runtime, не его расширение. Owner focused-frozen terminal: MIR2 0.027s PASS; pipeline 0.014s PASS. Reviewer CPU не запускал повторно.

На frozen source конкретных remaining correctness holes не найдено. Прежняя HL branch остаётся со своей точной emitted строкой; DE/BC получают консервативный function-wide use1 и minimum two-byte global storage guard. Реальный gain, размеры и полный host/native state устанавливает следующий actual caller experiment, не этот source review.
