# Runtime compiler и MZE на safe share

16 frozen executables проверены SHA256 в dated compiler-checkpoints. Новые: mz-cpm-recursive-final, mz-cpm-control-flow-final, mze-cpm-517ecb; источник и exact COM/ASM доказательства привязаны к своим контекстам. Прежний manifest сохранён в checkpoint-manifests.

Core gate a3cc555f: HIR PASS за 23,944 с, MIR2 PASS за 8,916 с, Nanz PASS за 13,170 с, VIR PASS за 2,100 с. CLI ещё выполняется; общий core exit 0 пока не объявляется. Full pipeline 22e426de также live, до последующих runtime/ABI исправлений.
