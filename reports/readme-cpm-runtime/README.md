# README hello CP/M: exact original sources

Заморозка источников/ожидаемого вывода3eb6a3f5; база91737dbd. Оба исходных putch до исправления отвергались RuntimeCPM (16d7988b). Отказы/полный MIR и allocation сохранены в before/. Production исправление8b01bf07.

Допуск узкий: единственный u8 param фактически в A, void leaf, единственный ASM с `LD E,A; LD C,2; CALL literal5`, полный AllClasses clobber. Asm.Ins может отсутствовать только при этом проверенном физическом контракте. Значения CALL5/0x0005 принимаются как одна literal entry; symbols/expressions/extra instructions/wrongphysicalinput отказаны. Исходные функции и ASM не переписаны. Машинная позиция полного адаптера повторно проверяется after emission.

**hello_cpm:** точный `Hello!\r\n`; COM1178B, 2503 CPU T, SHA5f33e61ddb22be8e8f9084c23aeaae7c90535e0dea1da14249db0754da3d11ad.

**hello_cpm_fib:** точный `Hello Frill!\r\nfib(7)=13\r\ngcd(12,8)=4\r\n`; COM1523B,11635 CPU T,SHA4eb0dc0a225bd8e310a8a073a2330ba78d38c36a155a714f176aabff132c24a5.

Три режима ordinary/GraceSize/GraceSpeed × preserving/full-primary-register+flags-clobber BDOS2 host:12wholeCOM executions PASS. ORG0100,OS RET PC0000/SPfe00, private linked stack1024, guardA5/status0, stackhighwater10B. Every actual PUSH/CALL word verified, incoming OSreturnword preserved; full64KiB/code immutable outside exactnamed runtime/spill regions. T costs exclude synthetic BDOS service. Modes sameBIN, noGracebenefit claimed.

Source assertions fib7=13/gcd12,8=4 preserved and separately PASS rawVM/finalVM/assembledZ80 all3modes. Successful constant calls in whole main are folded; main evidence does **not** prove recursive runtime fib traversal. Native assertion path calls original fib/gcd and covers those exact inputs; no fullbytefibdomain claim. VM ASMstdout not faked.

Literal negative and existingCPM verifier tests PASS.042s. README +originalimpl admission/main +originalLanz/tokenizer focusedPASS1.573s. Frozen compiled test replay log included. No long/fullpipeline rerun. This is emulator runtime evidence, not real physical CP/MOS.

Public CLI (from repository root):
```
/mnt/safe/minz/grace/2026-10-12/readme-cpm-runtime/executables/minzc examples/nanz/hello_cpm_fib.nanz -t cpm --runtime cpm --runtime-stack 1024 --asserts none -o hello.com
```
For hello use hello_cpm.nanz. Add `--grace --hir-grace --Osize` for Size or `--grace --hir-grace` for Speed. Runtime compile opts assertnone because CP/Mruntime assertions need separate invocation; they are tested separately above. All6 CLI COM files match saved emulator-tested bytes. Exact commands in cli/, sources/options/ABI/raw+finalMIR/ASM/BIN/BDOS stdout ledgers retained. Frozen executablepaths/SHA in frozen-executables.json; test replay from minzc/pkg/pipeline: `pipeline.test -test.run '^(TestReadmeCPMWholeMain|TestReadmeCPMSourceAssertions)$' -test.v -test.count=1`.
