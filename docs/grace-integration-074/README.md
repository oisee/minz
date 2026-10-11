# Интеграция CFG guard, typed_print и callable signatures

5ed02f2d→c23ba0cc: guard credit проверяет исполнение checker на каждом пути роста стека. Negative branch/conditional bypass admission и native runtime checks проходят. COM selfLanz остаётся побайтно прежним.

0b73c326→42817543: field scratch сохраняет живые HL/BC/DE; ordinary method receiver Move использует formal pointer width для struct address. Оригинальный typed_print/text.print, whole persistent main/probes и отдельные decimal capacity workloads проходят raw/finalVM/native; 48 структурных ABI проб отдельно PASS. В MultiCall обнаружено изменение прежнего widening, исправляется отдельно перед общим gate.

2140b87c→4ba526a3: Nanz сохраняет callable signatures и actual result Ty. Typed indirect word и byte consumers проверены. Root solver-dependent table test первоначально SKIP из-за отсутствующего env; повтор с явным сохранённым Z3 запущен отдельно. Этот SKIP не называется native PASS. Остальные focused integration runtime/typed print/callable tests PASS; pipeline 2,190 с, MIR2/Nanz отдельные времена в логах.

Публичный mze-cpm-517ecb запустил exact parser.com и tokenizer.com с -t cpm: exit 0, stderr 0, 135/146 stdout bytes побайтно совпадают с независимыми capture oracle. Это публичная CLI-цепочка с BDOS эмуляцией, не настоящая CP/M OS.
