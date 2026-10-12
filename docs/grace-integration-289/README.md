# Showcase explicit capacity audit

Checkpoint292d140c. MINZ_SHOWCASE_CHECK_KNOWN=1 действительно исполнил все194пути: **193PASS / 1FAIL**,17,892с. Единственный отказ — host-only ZVM story[131072], превышающий обычное16-bitZ80 адресное пространство. Это проверенный capacity отказ, не новый miscompile. Native u32 spill уже поддержан; полный host ZVM остаётся отдельной целью/границей. FAIL log сохраняется целиком.

Обычный showcase и полный frontend gate запущены отдельно; здесь их PASS не заявлен. Проверка сборки/assembler не заменяет независимое runtime поведение корпуса. Canvas/self-parser в независимых runtime проверках обнаружили коллизии default extern formal registers; fix ещё исследуется, не интегрирован.

Полный frontend gate292d140c завершён PASS: HIR26,615с, Nanz21,242с, Lanz0,020с, Lizp5,352с, PL/M0,048с, Pascal0,048с, C898,044с, Frill0,161с. Существующие тесты не покрывают найденные original extern multiarg runtime miscompile; они проверяются независимо и ещё не исправлены.

Default showcase завершён: {'PASS': 193, 'SKIP': 1}. Hostcapacity path имеет explicit known SKIP; explicit audit выше сохранил реальный отказ.
