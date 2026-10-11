# PFCCO: польза на существующем arena allocator

На неизменённом `examples/nanz/08_arena_allocator.nanz` обычная CLI стратегия `--contract-strategy pbqp` уменьшает **полный BIN 1193 → 1184 байт**. Реальный `arena_split` ускоряется **220 → 190 T**, исходный `test_split` вместе с вызовами и сохранениями — **3896 → 3808 T**. База `9b1637ef`, `--Osize`, обычный runtime. Производственный код и допуск checked tables не менялись: это независимая проверка пользы уже подключённого PFCCO.

Перед выбором проверены девять исходных corpus examples: tiny, strref, bool returns, rotate sled, promoted arithmetic, signed arithmetic, arena, impl showcase, self tokenizer. Checked tables нового выигрыша кроме уже известного triple не дали. У arena стратегия PFCCO дала −9 байт, у self_tokenizer −12 байт; второе наблюдение пока только размер, не доказательство native корректности/скорости. Scan с названиями отказов сохранён. Из примеров выбрали arena для полного native исследования; это post-selection benchmark, не prospective heldout нагрузка.

Почему меняется код: greedy контракт `Arena_init(self=HL, base=BC, size=DE)` использует стек и копирование для сложения. PBQP выбирает `base=DE, size=BC`, позволяя `EX DE,HL; ADD HL,BC; EX DE,HL`. У `arena_split` аргумент start переезжает IX → DE, size DE → BC; сокращается работа с половинками IX и дорогие переносы пар. Все аргументы caller переставляет по настоящему контракту, caller saves входят в ASM/BIN и измерения. Это конкретный улучшенный вариант, без заявления глобального оптимума PFCCO.

Активный native тест компилирует точный исходник с default greedy и opt-in PBQP, затем сравнивает VM и машинный Z80 код с независимыми ожиданиями исходных assert/sandbox. Шесть исходных функций возвращают 0; пять последовательных shared-state вызовов проверяют 0x3f00 свободных байт, три выделения по 64 байта с адресами C000/C040/C080 и остаток 0x3e40. Совокупная стоимость этих **11 конкретных вызовов 13100 → 13012 T**; это не взвешенный профиль приложения.

Дополнительно `arena_split` проверен по независимому арифметическому/памятному оракулу на 100 сочетаниях start/size из `{0,1,255,256,7fff,8000,c000,ff00,fffe,ffff}` для каждой стратегии: возврат `(start+size) mod 65536`, структура содержит start и конечный адрес в little endian. Указатель D000, память отделена от кода и стека. Реальные физические locations трёх аргументов берутся из контракта и allocation. Проверены все четыре записанных байта на VM/Z80 и SP=f000 при возврате. Language Book v7 §3.1 подтверждает ширину u16, §2.7 — byte-accurate layout; модульное сложение явно задано как Z80 width oracle.

Ordinary CLI greedy/PBQP PASS; обе выдачи ASM побайтно совпадают с API evidence. Команды:

```sh
minzc examples/nanz/08_arena_allocator.nanz --Osize --asserts none \
  --contract-strategy greedy -o greedy.a80
minzc examples/nanz/08_arena_allocator.nanz --Osize --asserts none \
  --contract-strategy pbqp -o pbqp.a80
```

Focused native test PASS 0.149s; regression admission/identity + два прежних source-native controls + arena записан в packet. Полный pipeline не запускали. Default greedy сохранён, никакого blind call/CFG admission нет. Следующий отдельно исследуемый кандидат — self_tokenizer: уменьшение BIN уже наблюдается, требуется независимый native workload до вывода о пользе.

Артефакты `reports/artifacts/pfcco-arena/`: неизменённый source, оба ASM/BIN, CLI ASM/команды/логи, native 11calls +100boundarypairs результаты, scan и SHA256 manifest.
