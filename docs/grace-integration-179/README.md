# Интеграция 179: исходные tuple-модули и полные манифесты

Root независимо повторил `TestOriginalTupleCorpusComplete` на `fe51f4f7`: PASS, точный лог рядом. Три неизменённых модуля проверяют 1 773 408 native CALL в трёх режимах, включая оба tuple результата, caller projections, исходный PC/SP и всю память вне наблюдавшегося стека. Ещё два пути March13 прочитаны отдельно и имеют идентичные SHA256 исходников March11; перенос доказательства основан на этих байтах, не похожих именах. Полный u16×u16 домен и divisorzero не заявляются.

Все 194 пути сохранены в каталоге: 65 independent/bounded и 72 pending-independent-oracle; остальные ограничения без изменения. Source assertions и обнаруженные ссылки не повышают статус сами по себе.

После `7b54eba9` манифест Lizp tokenizer полон: все 37 payload tracked и SHA256 проверены root локально и на шаре. Tuple packet: все 84 payload также проверены. R5RS packet57 и parser-reference51 проверены ранее. Отчёты178/177 фиксировали тогда ещё отсутствующие probe файлы; этот checkpoint их закрывает.

Полные CLI2612a266/pipelinecfaa7777 всё ещё отдельные live-прогоны. Новый focused PASS не превращает их в green fullsuite.
