# Независимый review прямых глобальных word loads

Разрешён узкий прототип DE/BC по плану bd61562d. Это проверка дизайна, не измерение результата. Родительский compiler не изменён; VM/CPU повторно не запускались.

Matcher и emitter должны одинаково допускать только HL/DE/BC, ширину 16, точный OpAddrOf существующего глобального символа и единственного потребителя адреса. DE использует ED 5B, BC ED 4B. Existing emitNativeMemoryf(inst,0,2) сохраняет прежний symbol/offset/width; новых разрешений памяти нет.

Проверены memory_host_sidecar.go: ED op&0xcf==0x4b даёт direct address из байтов 2/3 и width2/read. memory_host_access_recipe.go формирует два события по возрастанию адреса, с u16 wrap. remogatto ld16rrnn читает низкий байт прежде высокого; ED4B/5B вызывают эту функцию. A/F/HL не меняются. invalidate(dst) удаляет знания destination pair и обеих его byte aliases; знания независимых регистров сохраняются.

Существенное уточнение: scanGlobalFieldPatterns считает useCount только внутри блока плюс его terminator. Поэтому нельзя описывать существующий useCount==1 как доказательство единственного потребителя во всей функции. Контрпример допустимого SSA: entry определяет addr=AddrOf(g), value=Load(addr,u16), затем Jmp(next); next непосредственно использует доминирующий addr в другом Load либо Return. В entry локальный счётчик равен 1, хотя удалённый AddrOf нужен дальше. Перед расширением matcher нужен консервативный function-wide use/liveness guard или отказ при внешнем использовании. Edge Args уже учитываются, но прямые cross-block uses ими не исчерпываются. Это прежняя проблема HL matcher; новые DE/BC случаи не должны её распространять.

Обязательные meaningful regressions владельцу: DE и BC с live AF/HL; последующее использование destination byte и store после загрузки, проверяющее cache invalidation; значения 0/255/256/65535; точные low/high data accesses без третьего чтения/записи; numeric pointer, same-block shared pointer и cross-block live AddrOf остаются материализованными. В closed runner wrong symbol/address/direction/width и второй байт за разрешённым root должны отвергаться, а не получать generic resource permission. Прямой load глобального хранилища не является заменой двух host peek для произвольного ресурса.

LD HL,(nn) с opcode 2A имеет 16T; ED6B имеет 20T. Прежний комментарий 20T для 2A неточен, дефекта измерителя не найдено. Оценка (1469+1444)*16=46608T относится лишь к двум старым последовательностям. Реальные whole-program T/bytes будут измерены владельцем после frozen source review.

Владельцы: goto_artifacts — два matcher/emitter файла и regression tests в global-word-load-direct-pairs; этот branch — только review artifacts. Сначала focused machine tests и проверка frozen diff; затем владелец выполняет full30 original и both один раз через прежний closed runner. Старые PC/evidence и CompilerID не перелицовываются.
