# Пять Agon-примеров: проверка сборочного продукта

Все пять точных исходников examples/agon собраны через HIR → MIR2 → eZ80 ADL в ordinary, Grace Size и Grace Speed. Все 15 продуктов имеют разрешённый main внутри BIN, .ASSUME ADL=1 и origin 040045; assembler errors отсутствуют. Root PASS 0,572 с.

| Исходник | BIN, байты во всех трёх режимах |
| --- | ---: |
| hello_agon | 119 |
| hello_world | 197 |
| nc_agon | 1940 |
| tui_demo | 1420 |
| tui_hello | 1030 |

Это проверка продукта, не исполнения: ADL/MOS/VDP требуют подходящего CPU/runtime. Категория agon-mos-runtime сохраняется. Дополнительного Grace выигрыша по размеру в этих 15 продуктах нет; такты не измерялись. Исторический TestAgonHelloMOSProductsAndBoundary имеет явно mocked MOS и обычный Z80 caller — он не заменяет ADL runtime.

freeze.json связывает исходники с SHA256. products содержит все исходники, ASM/BIN и evidence.json с SHA, origin/main и явным execution=not verified. Регрессия: minzc/pkg/pipeline/agon_corpus_product_test.go::TestAgonCorpusADLProducts. Полный pipeline не перезапускался; этот тест появился после snapshot350149ad.
