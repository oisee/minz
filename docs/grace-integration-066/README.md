# Первый публичный CP/M runtime: интеграция

f617aad9 интегрирован как ffe0f887. В конфликте CLI сохранены обе проверки: strategy и runtime. Фокусированные CLI runtime и strategy admission PASS за 0,043 с. Обычная команда создаёт 524-байтовый COM; собранный Z80 с CALL5-handler выводит `CP/M 53 OK\n`, сохраняет result54, регистры и флаги через BDOS и восстанавливает incoming SP.

Это первый leaf-срез с resident 256-byte stack, не доказательство полного selfLanz и не исполнение настоящей CP/M OS. Stack usage внутри BDOS зависит от системы; приложение отдельно проверяет свой стек. Исходник, полный ASM/COM, executable SHA и full MIR2 proof предыдущего контекста сохранены в соседнем grace-cpm-leaf-runtime.

Следующий runtime шаг: оригинальные tokenizer/selfLanz, явные byte-memory bindings, user calls и ограничение стека. Общая цель активна.
