# Original ex20 Arena: методы, wrap, OOM и persistent sandbox

Проверен именно `reports/showcase-src/2026-03-14/ex20_arena_allocator.nanz`,
SHA256 `886e1b3fb2e92c1f16ebf585a03a81f6fd1b2d0bcd2ed24559b4a9584adf21d9`.
Исходник не изменён; результат другого arena example сюда не переносится.
До выполнения сохранены источник, SHA, модели и домены в frozen-domain.json.
Все восемь entries вызываются: Arena_init/alloc/reset/remaining, arena_split,
test_split_and_alloc, init_a, alloc_enemy. Source assertions и sequential_alloc
sandbox сохраняются при компиляции, включая явное `via mir2` исходника.

Модель независима от MIR: Arena — четыре little-endian byte ptr/end.
Init: ptr=base, end=(base+size) mod65536. Alloc: result=ptr,
next=(ptr+n) mod65536; unsigned next>end означает return0 без записей;
иначе ptr=next и return result. Reset меняет только ptr, remaining=end-ptr
mod65536. Обоснование — исходные поля/порядок операций и unsigned widths
Language Book v7, struct layout и ADR-0045 wrap каждого byte-address.
Это исходный modular allocator: он не гарантирует запрет overflow.
Например wrapped next может пройти guard и обновить ptr; оракул не заменён
более строгим overflow-safe allocator.

Места четырёхбайтного объекта: 0x4000/0xfffe/0xffff, включая wrap полей.
14 значений границ: 0/1/2/7/127/128/254/255/256/257/32767/32768/65534/65535.
Init/split: все14×14 base/size на каждом месте. Alloc: все14³ ptr/end/n;
remaining: все14² ptr/end; reset: все14³ ptr/end/newbase.
Это конечный boundary domain, не все u16 states или все последовательности.

Original wrapper точно проверяет весь perm/level/frame: конечные пары
(C008,C100)/(C100,C900)/(C920,CD00), включая исходную reset/alloc последовательность.
Persistent init_a→258 alloc_enemy выполняется на одном raw VM, одном final VM
и одном native image: исходные три результата C000/C004/C008, далее exhaustion
до C400 и два OOM. Setup не может восстанавливать испорченное a перед alloc:
любой mismatch состояния до очередного вызова останавливает тест.

Во всех трёх режимах (ordinary/Grace Size/Speed, ContractOpt=true) проверены
raw/final MIR VM и настоящий assembled Z80 CALL по исходным ABI.
Exact PC возврата/SP, весь Arena, обе adjacent guards, неизменность code/fullRAM
вне точных исходных globals, harness object bytes, typed spills и наблюдаемого
стека. Every ordered byte-store и значение сравниваются с source-field model.
Оставшиеся plain-memory loads должны наблюдать актуальное simulated field;
избыточные чтения могут законно удаляться forwarding. Native write-profiler
не допускает записей вне этой области, включая wrap в low RAM (ROM отключена).

55 464 native CALL всего, 18 488 на режим. Все режимы имеют тот же BIN464 байта
и суммарную стоимость3 827 976 T-states на фиксированный домен.
Максимум стека12 байт в test_split_and_alloc. Miscompile не найдено,
Grace не даёт дополнительного выигрыша в этом пакете.

ASM/BIN, raw/final MIR, source/ABI/options/symbols, CALL drivers, results,
логи и retained test executable сохранены с SHA в manifest.
Это локальный gate одного original module, не общий pipeline/CLI gate.

Полный retained gate **PASS, 0,70 s**. Retained executable SHA256:
`eee2bfa8aea9b3c9515b986a3e1316255b0c41581d3ba593c626ea8d3fa9ab32`.
`first-test.txt` — предварительный прогон; complete-test.txt включает
full VM RAM, guards и fail-closed сохранение состояния перед alloc_enemy.
Воспроизведение из minzc/pkg/pipeline:

```sh
ARENA20_EXPORT=/absolute/new-output ../../../reports/original-arena20-corpus/pipeline.test \
  -test.run '^TestOriginalArena20Corpus$' -test.count=1 -test.v
```
