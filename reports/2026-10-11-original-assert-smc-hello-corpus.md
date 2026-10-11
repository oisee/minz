# Исходные assert, SMC и hello: область независимой проверки

Сохранены неизменённые `reports/showcase-src/2026-03-11/ex12_assert.nanz`,
`ex15_smc_sprite.nanz`, `ex16_hello_mze.nanz`. Все версии 2026-03-13
прочитаны отдельно: соответствующие SHA совпадают. Обе датированные копии
сохранены; один набор байтов не считается двумя разными программами.
Источники, SHA, модели и домены зафиксированы до выполнения в
`original-assert-smc-hello-corpus/frozen-domain.json`.

Ex12: все исходные entries (`double`, `clamp`, `abs_diff`), все девять
исходных assertions проходят обычную компиляцию. Независимые оракулы:
`2*n mod256`, математическое `abs(a-b)`, clamp с точным порядком условий
`if v<lo return lo; else if v>hi return hi; else return v`.
Double: все256 byte-входов; abs_diff: все65 536 пары; clamp: все16 777 216
byte-тройки, включая lo>hi. Поэтому необычные обратные границы не заменены
удобной симметричной clamp-моделью. Source control flow и Book v7 числовые
типы/§2.5 задают ожидания, исходные assertions остаются в оригинале.

Ex15: native проверяет именно source functions `draw_row`, `draw_row4`
и сгенерированные patchers `draw_row_set_r0`, `draw_row4_set_r0`.
Book v7 §12.2 задаёт HL-параметр patcher и два immediate-байта в машинном
коде. Настоящий CALL patcher меняет эти два байта, следующий CALL draw
пишет соответственно C3/3C либо FF/81/81/FF.
PTR=0/1/0x4000/0xfffc/0xfffd/0xfffe/0xffff, адреса каждого byte-store
заворачиваются modulo65536 по ADR-0045. Lower RAM доступна для записи,
ROM-защита отключена. Отдельный phase-check запрещает изменения за пределами
текущего immediate-поля или текущего sprite destination; итоговый full RAM
контроль защищает остальной код и память. Native работает с реальными
побайтными патчами; ни ABI helper, ни draw-функция не заменены моделью.

VM не исполняет Z80-инструкции и не имеет physical immediate encoding.
OpPatchSlot в VM — heap-slot abstraction. В пакете отдельно сохранены
raw/final VM outcomes каждого original draw; успех VM не выдаётся за
доказательство native SMC или динамической репатчиваемой семантики.
Это явная граница модели, а не зеленая synthetic подмена.

Ex16: `abs_diff` проверен на всех byte-парах raw/final VM/native.
`main` проверен целиком: VM печать перехватывается из stdout,
native OUT(0x23) использует console model MZE. Ожидаемые байты:

```text
=== Nanz Z80 ===
abs_diff(10, 3) = 7
abs_diff(200, 55) = 145
Done!
```

Output сохраняется отдельно для raw VM, final VM и native.
Это console-путь original MZE example, не демонстрация CP/M BDOS runtime.

Во всех случаях ordinary/Grace Size/Grace Speed, ContractOpt=true.
Native начинается истинным CALL и заканчивается на точном PC возврата
с исходным SP; full RAM/code и profiler-write guard допускают только
наблюдаемый стек и явные SMC operands/destinations. Source pure math не
имеет разрешённых memory operations в VM. ASM/BIN, raw/final MIR, ABI,
символы, источники, CALL drivers и retained executable сохраняются с SHA.
Полный общий pipeline/CLI gate этим локальным пакетом не заменяется.

Frozen-domain исторически называет assert chapter11; правильная ссылка —
Book v7 chapter7 (compile-time assertions). Замороженные bytes не изменены
после измерения. `first-smc-hello.txt` — предварительный native/VM-outcome
прогон; окончательный phase write-guard и полный clamp домен выполняются
сохранённым исполняемым файлом в `complete-test.txt`.

Retained executable SHA256:
`64da7e0fe37977c524ba34e0fdb8fcc2b45233aaee4629f75c7e744c8925594b`.
Воспроизведение из minzc/pkg/pipeline:

```sh
ASSERT_SMC_HELLO_EXPORT=/absolute/new-output ../../../reports/original-assert-smc-hello-corpus/pipeline.test \
  -test.run '^TestOriginal(AssertCorpusComplete|SMCHelloCorpus)$' -test.count=1 -test.v
```

Итог retained gates: assert **PASS, 113,80 s**, SMC/hello **PASS, 0,42 s**.
Всего50 725 719 native CALL во всех трёх режимах: ex12 по16 843 008,
SMC по28 (patch+draw), hello по65 537. Модули соответственно19/42/297
байт во всех режимах; стек2/2/4 байта. Ex12 cost844 774 912 T-states
на одинаковый полный домен в каждом режиме. Native/code/state проверки
прошли, miscompile не найдено; Grace здесь не добавляет выигрыш.
