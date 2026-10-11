# Интеграция CFG guard, typed_print и callable signatures

5ed02f2d→c23ba0cc: guard credit проверяет исполнение checker на каждом пути роста стека. Negative branch/conditional bypass admission и native runtime checks проходят. COM selfLanz остаётся побайтно прежним.

0b73c326→42817543: field scratch сохраняет живые HL/BC/DE; ordinary method receiver Move использует formal pointer width для struct address. Оригинальный typed_print/text.print, whole persistent main/probes и отдельные decimal capacity workloads проходят raw/finalVM/native; 48 структурных ABI проб отдельно PASS. В MultiCall обнаружено изменение прежнего widening, исправляется отдельно перед общим gate.

2140b87c→4ba526a3: Nanz сохраняет callable signatures и actual result Ty. Typed indirect word и byte consumers проверены. Root solver-dependent table test первоначально SKIP из-за отсутствующего env; повтор с явным сохранённым Z3 запущен отдельно. Этот SKIP не называется native PASS. Остальные focused integration runtime/typed print/callable tests PASS; pipeline 2,190 с, MIR2/Nanz отдельные времена в логах.

Публичный mze-cpm-517ecb запустил exact parser.com и tokenizer.com с -t cpm: exit 0, stderr 0, 135/146 stdout bytes побайтно совпадают с независимыми capture oracle. Это публичная CLI-цепочка с BDOS эмуляцией, не настоящая CP/M OS.

Повтор с VIR_TEST_Z3=/home/alice/dev/minz-artifacts/grace-tools/z3-python/bin/z3 завершился PASS за 0,430 с: реальный apply(triple,x) на всех 256 входах, 123 → 111 T; модуль 34 → 31 B.

MultiCall widening восстановлен 79c228bf→a3cc555f: prior formal type+lowerExprAs, ordinary struct pointer fix сохранён. Независимый 16-bit tuplecaller regression и весь typed_print/formatter/struct native набор PASS за 1,016 с; исходные ASM/BIN SHA не изменились. Новый полный core gate на a3cc555f запущен; результаты ещё не заявляются.
