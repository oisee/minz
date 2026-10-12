# C89: вложенные блоки при struct-return promotion

Аудит всех39 examples/c89/*.c после e136abcc:38 valid HIR/lowered, minmax_pair.c invalid. Самостоятельный hir.Block внутри ветви не обходился ни анализом eligibility, ни rewriteReturns; callee был promoted, но вложенный return r оставался scalar ptr. Добавлен обход самостоятельных блоков при анализе/переписывании returns, call sites и dead struct cleanup/reference scanning.

После исправления39/39 проходят parse, hir.Verify и hir.LowerModule. Постоянный TestC89ExampleHIRContracts не скрывает parse/verify/lowering failures. Этот PASS не доказывает native behavior всех39 файлов.

Оригинальный minmax_pair.c отдельно: smaller/larger независимо сверяются с min/max. Полные65536 byte-pairs, обе функции, raw VM и final VM трёх режимов; native проверяет49 word-boundary pairs ×2 wrappers ×3 modes,294 native assertions. Исходные six mir2 assertions сохранены. Все PASS0.866s включая designated regression. Полный C89 package PASS6.190s. Native full16-bit Cartesian не заявлен; Z3 solver corpus ещё не повторён.

Pipeline b6b4e0c7 продолжает свой immutable checkpoint; эти исправления в него не входят. Исторические before/after JSONL содержат полный список39 исходных путей.
