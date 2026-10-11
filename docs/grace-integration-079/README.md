# Исходный IX load/store: алиасы и порядок эффектов

b877c9f4 интегрирован как 8ab1864a. Неизменённый 07_ix_load_store.nanz: 1536 roundtrip + 6 sum вызовов+observer, все byte values и 6 alias layouts. Independent oracle выполняет store перед loads и сверяет ordered write/read, все 259 RAM bytes, PC/SP/code. Focused root PASS за 0,109 с; 218507 T / 38 B — весь workload, не скорость одной операции. Source и backend не менялись. Каталог после этого шага содержит 34 bounded VM/native paths плюс 1 native CPM demo из 194.

Negative checked word XOR packet интегрирован отдельно: production patterns не меняются, hash16 не получил измеренного выигрыша. Existing current core PASS сохраняет свой контекст; full pipeline 22e426de ещё live.
