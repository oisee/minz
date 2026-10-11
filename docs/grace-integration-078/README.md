# Showcase после ABI и runtime исправлений

На 76bbe91e полный TestShowcaseCompileAssemble завершился PASS: 193 PASS, 1 SKIP из 194 точных путей. Изменения callable signatures, struct pointer ABI, field scratch, multiply relocation и guarded runtime входят в этот source snapshot. Сборка здесь ordinary default backend; она не доказывает whole-program correctness каждого примера. Отдельная карта 33 VM/native bounded paths плюс 1 CPM native-only находится в integration 077.

Core a3cc555f также завершился PASS, полный более ранний pipeline 22e426de ещё выполняется. Исторические логи не переписываются. Исходная цель активна: remaining runtime/ASM/corpus proofs и полезный перенос оптимизаций продолжаются.
