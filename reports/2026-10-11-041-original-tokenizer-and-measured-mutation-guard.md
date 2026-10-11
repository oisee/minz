# Исходный tokenizer и неизменность измеренных кандидатов

Интегрированы `919b54c2` и `c3782f21`. Selector после финального factory sentinel
перепроверяет full HIR, семантические metadata MIR, dump/ABI, allocation и ASM
каждого измеренного кандидата. Контрпример с поздним изменением диапазона типа
теперь отвергается; прежние численные результаты не переписываются.
Root повтор admission + tail/PFCCO heldout: PASS, 3.858 s.

Оригинальный self_tokenizer.nanz не изменён. Init и 30 sequential inputs
проверены на raw VM и native Z80: все 8192 байта string arena, 4096 байт tokens,
4096 байт source и 47 globals совпадают с независимой моделью.
Замер: 1916274 T / 5466 B. Root repeat PASS 2.498 s.
Стандартный print исходника использует OUT 0x23; main и CP/M вывод этим
computational workload не проверялись. Сохранены source quirks, не исправлялась
семантика исходника ради оракула.

Private native runner ранее отвергал настоящий `str_len EQU peek`: alias
с одинаковым PC затенял reachable peer. Теперь группа equal-PC допускается,
если достижим хотя бы один alias; следующий distinct unreachable function
снова закрывает доступ. Operand interiors и DB также отвергаются.
Root alias regression PASS 0.010 s. Карта содержит 23 bounded-covered paths.

[Tokenizer](../docs/grace-original-tokenizer-native/README.md),
[повтор](../docs/grace-original-tokenizer-integrated.txt),
[selector](../docs/grace-late-mode-mutation-integrated.txt).
Пакеты сохранены на шаре с SHA256. Общая цель активна.
