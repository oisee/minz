# PFCCO на исходном self_tokenizer: меньше BIN, медленнее keyword rendering

На неизменённом `examples/nanz/self_tokenizer.nanz`, база `40897f43`, обычный `ContractOpt=true, OptSize=true, AssertMode=none`, runtime без CP/M/guard, стратегия PBQP уменьшает полный обычный BIN **5474 → 5462 байта**. Это **size gain 12 байт**, не ускорение токенизатора.

Фиксированная до замера последовательность: init_keywords; tokenize_all первого входа; print_all_tokens; tokenize_all второго входа; print_all_tokens. Два новых входа и исходный SHA записаны в `fixed-workload.json`. Опции и код одинаковы, меняется только ContractStrategy `"" → "pbqp"`. Входы выбраны до timing, но сам consumer уже выбран по предыдущему size scan: это независимые контрольные нагрузки, не prospective profile-selected validation и не доказательство пользы на любых входах.

| Стадия | greedy T | PBQP T |
|---|---:|---:|
| init_keywords | 15 790 | 15 790 |
| tokenize input0 (keywords) | 134 565 | 134 565 |
| print input0 | 21 896 | 22 172 |
| tokenize input1 (identifiers/operators) | 128 403 | 128 403 |
| print input1 | 18 512 | 18 512 |
| **Вся фиксированная последовательность** | **319 166** | **319 442** |

PBQP покупает 12 байт за +276 T на этой последовательности. **Строгая цель Speed должна выбрать greedy**, цель Size — PBQP. Текущий измеренный selector эти стратегии ещё не перебирает; этот отчёт не утверждает, что выбор уже автоматизирован. Default greedy сохранён.

Причина видна в ASM: изменился контракт `tok_name` — str_ptr BC → DE. У десяти keyword comparisons вместо четырёх LD половинок используются PUSH/EX/POP; это немного короче, но дороже по тактам. Основное сканирование, interning и init не изменились. Сохранения вокруг calls и реальные переносы параметров входят в бинарник и замеры; никаких поправок на «предполагаемую стоимость» нет.

Независимый lexer/interner model проверяет каждый вход: полный string arena 8192 байта, token arena 4096, source buffer 4096 и все globals на MIR2 VM и Z80. Отдельный host renderer выводит ожидаемый stdout из model tokens. Проверка не принимает только одинаковый счётчик: сравниваются все bytes, intern pointers, keywords и восстановление SP. Неизменность code/string literals проверена; исключения — точные mutable module-global spans и только точные assembler-listing bytes `_spill_` slots, без исключения соседнего кода. Guards проверены у 8fff/d000/e000/f000. Native model явно отключает ZX ROM write protection для тестовой RAM-сборки ORG 0400.

В исходнике peek/poke — externs. Обычная CLI ASM/BIN сохранены отдельно от native target: одинаковые адаптеры `peek: LD A,(HL); RET`, `poke: LD (HL),C; RET` добавляют 4 байта обеим сборкам, сохраняя исходный контракт HL/C и весь generated code. **5474/5462 — обычные BIN; 5478/5466 — адаптированные исполняемые BIN**. Runtime guard улучшения в сравнение не входят. VM исполняет scanner через независимые peek/poke heap handlers; rendering проверен native + независимым host renderer, VM output intrinsic здесь не служит оракулом.

Обычная CLI greedy/PBQP PASS: ASM каждой стратегии побайтно совпадает с соответствующим API ASM. Команды:

```sh
minzc examples/nanz/self_tokenizer.nanz --Osize --asserts none \
  --contract-strategy greedy -o greedy.a80
minzc examples/nanz/self_tokenizer.nanz --Osize --asserts none \
  --contract-strategy pbqp -o pbqp.a80
```

Focused test PASS 1.582s. Повтор вместе с arena и прежними ContractStrategy source/admission controls PASS 2.148s. Полный pipeline в этой ветке не запускали. Production не изменён; появился активный тест size/speed tradeoff. Артефакты: `reports/artifacts/pfcco-tokenizer/`, SHA manifest, точный source/workload, ordinary+native ASM/BIN, stdout/full-state captures и CLI/native/regression logs.

Read-only аудит следующего шага: `MeasureCompilerModeVariants` и replay фиксируют четыре ContractOpt/UseGrace комбинации с default greedy; MemoryHostPolicySearch перечисляет Legacy/PreserveForSpeed × arena/append masks и ограничен историческим exact-source SHA. Ни один не перебирает ContractStrategy PBQP. Добавлять стратегию надо через новый явно versioned candidate-set/schema, с native oracle, exact options IDs и Speed/Size выбором; старые manifests/replay расширять молча нельзя. Opaque memory hosts/render output не покрываются обычным ProfileWorkload mode API, поэтому для self_tokenizer также требуется явный workload-domain contract. До такого изменения blind call/CFG/table admission не расширяли.
