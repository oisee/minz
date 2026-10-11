# signed_arithmetic.nanz: исходные функции на VM и Z80

Base `3e91d690` после исправлений u32 logic, полного dynamic shift count и non-HL constant SHL. Проверен неизменённый `examples/nanz/signed_arithmetic.nanz`, SHA256 `7a5860570b48508e67af293b9cc34ab86b66fc32b1f278efd34b382908b59f45`. Весь исходник заново разобран и собран ordinary `CompileHIRSteps` с `ContractOpt=true, AssertMode=none`; asserts в файле не служат оракулом этой проверки.

Все девять source функций проверены независимыми Go width/sign выражениями, raw MIR VM, final MIR VM и native assembled Z80. В native используются реальные выбранные param homes и return contract, фиксированный PC возврата 0xf100 и восстановленный SP 0xf000. После всех запусков сравниваются все байты кода и литералов; в этом модуле нет mutable globals или spill slots.

Всего **266 240 случаев**:

| Функции | Область | Случаев |
|---|---|---:|
| sar, logical | каждый byte input × каждый byte count 0..255 | 131072 |
| quotient, remainder | каждый byte input × каждый ненулевой byte divisor | 130560 |
| sar_one, div_two, mod_two, widen | каждый byte input | 1024 |
| sar16 | 14 signed/width boundary inputs × каждый count 0..255 | 3584 |

Оракулы signed arithmetic используют int8/int16 sign extension, truncation к declared width и signed деление с округлением к нулю; Book v7 §3.2 описывает stored bits/sign extension. В том числе проверены overflow-result bits `-128 / -1`, отрицательный remainder, sar16 count >=16 и полный byte count domain. Для out-of-width shift counts книга не задаёт отдельную норму: тест сохраняет existing VM semantics и независимый width oracle, не приписывая книге modulo-count.

Делитель 0 исключён из value-result oracle: trap/error semantics — отдельный контракт и отдельная работа, не скрытая проверка результата. u32 SHA256 workload здесь не проверялась.

PASS: `signed-source-native.txt`, финальное время 0.827 s. Источник/полный ASM/BIN и SHA256 — `artifacts/signed-source/manifest.json`. Полный pipeline повторно не запускался; full MIR2 PASS 25.915 s относится к предыдущему production fix, а этот коммит добавляет original-source доказательство без изменения backend.
