# Новый Fibonacci oracle и исправленный каталог доказательств

Snapshot2cb4639c. Exact reports/showcase-src/2026-03-11/ex10b_fib_iter.nanz:
все256byte n, результатu16 по независимой iterative modulo65536 модели.
Raw/finalVM и native ordinary/GraceSpeed/GraceSize; RAM guards, отсутствие
memory effects, immutable code, возврат SP. Все три режима34B/2415644T
на суммарной нагрузке — нового speedup здесь нет.

Root newfib + прежний arithmetic prospective heldout + catalog tests:
pipeline PASS3.090с, Nanz PASS2.143с. Источник18_tail_recursion уже имел
отдельные16train/24heldout inputs и frozen native evidence; каталог пропускал
эту ссылку. Поэтому38VM/native paths =36previous +1newfib +1oldomission,
не два новых алгоритма. NativeCPMdemo остаётся отдельным proof.

[Scope и source semantics](../grace-original-fib-iter.md). Оба пакета SHA
проверены и скопированы на шаре; их caller/runtime ограничения сохранены.
Showcase compile gate не доказывает packed FAT correctness: oddwrite_fat12
native mismatch всё ещё исправляется отдельно.
