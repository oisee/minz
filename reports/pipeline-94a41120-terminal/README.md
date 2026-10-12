# Terminal FAIL старого pipeline94a41120

Сохранённый полный executable завершился exit2 с panic. FAIL: BDOS caller test (исправлен a1759a8f); obsolete Tetris runtime rejection expectation; ZX general probe panic после смены source font. Pipeline dd100284 продолжает работать отдельно; его свежие parser reference drift исправлены в470a884f.

ZX panic воспроизведён на текущем дереве. Старый probe ожидал u16font и комментарий ZXfontdata; после полного760byte font replacement и23строк текста (две с auto-wrap) егоscreen-only модель не соответствует программе. Общий тест source2 теперь запускает существующий frozen TestOriginalZSQLMaraColorMainNative с exact source/font/text, трёхрежимным native/fullRAM oracle и исходным DI/HALT. Whole TestOriginalZXAppsNative PASS3,887с. Native coverage не ослаблено. Reviewed SHA обновлён после replay; каталог проходит.

Nanz/VIR/importer integration checkpoint1e74e46f PASS20,335/2,423/1,529с. Он предшествует новому ZX test routing. Terminal old FAIL сохранён полностью; не объявляется PASS и не перезапускается. Следующий независимый шаг — устранить старое Tetris runtime admission expectation на текущем API после проверки фактического runtime behavior.
