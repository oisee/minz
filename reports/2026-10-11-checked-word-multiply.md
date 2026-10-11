# Checked local tables: реальная польза константного word multiply

Добавлено ограниченное lowering `u16/i16 * RHS constant 2..255` в pure VIROps ADD/MOVE перед формированием локальной задачи. Исходный MIR/source и полный ABI/context остаются неизменны. Это **новое local lowering плюс table allocation**, не повторное использование исторической multiply recipe или доказательство GPU-оптимальности. Assignment здесь получен существующим Z3 solver и независимо проверен AddChecked.

Оригинальный `examples/nanz/mul16_gpu_test.nanz` с теми же options ContractOpt=true/OptSize=true/AssertMode=none: полный module112→63B. Native all65536 значений каждого original helper:

| Helper | Baseline T | Selected T | Выбор |
|---|---:|---:|---|
| mul_by_3 |74|74|baseline; raw candidate82T больше |
| mul_by_10 |96|96|baseline; raw candidate104T больше |
| mul_by_100 |1085|148|checked table |
| mul_by_7 |1085|104|checked table |
| mul_by_255 |1115|214|checked table |

T считаются entry→return sentinel, включаяRET без input setup. Все raw admitted candidates также проверены nativeall65536, включая невыбранные3/10. Оракул — `x*k mod65536`, rawHIR→VM дополнительно проверен для x<256 и x кратных257 (не всяматрицаVM). Native SP/returnPC и codeimage неизменны.

Новый caller-source fixture `mul7_caller(x)=mul_by_7(x)+x` компилятор инлайнит; егоmodule149→75B показывает пользу на исходной fullprogram, но **не является проверкой CALL**. Поэтому отдельно frozen assembly wrapper `PUSH HL; CALL mul_by_7; POP DE; ADD HL,DE; RET` действительно сохраняет x черезCALL. В нём все 65 536inputs проходят, actual fullcallee+wrapper1144→163T; wrapper занимает7B, поэтому полная originalmodule+wrapper119→70B. Это explicit native harness, не ordinarysource noinline feature.

## Почему новый домен ограничен

Binary Horner chain имеет не более14 ADD на word constant<=255. Каждый shift представлен ADD, исходный x сохраняется explicitMOVE, returnadapter остаётся explicitMOVE. HL forced destinations и BC save делают требуемую physical constraint явной. Без save constraint solver выбирал дешёвый `EX DE,HL`, уничтожающий живой исходныйHL; независимый AddChecked корректно отклонил это предложение. Verifier не ослаблялся.

Save destinations BC применяются только в задачах с word MUL; если другой live operand/alias не позволяетBC, Generate/AddChecked может отказаться и baseline сохраняется. Динамическое умножение, RHS0/1/256/negative, byteMUL не допускаются новым правилом. Signed word semantics — та же modulo-битовая арифметика; native корпус здесь unsigned. Неподдержанныйsource ABI или опcodes остаются за предыдущими guards.

Self moves для word сейчас emit PUSH HL/POP HL; два таких адаптера включены в148/104/214T. Их устранение этимcommit не выполнялось, claimedcost не уменьшался вручную. Полезные случаи выиграли и с этими накладными расходами. Для3/10 size gate честно оставилbaseline. Оригинальный defaultbackend без checkedtable не меняется.

## Использование

```sh
# из minzc/, новые outputs
go run ./cmd/minzc ../examples/nanz/mul16_gpu_test.nanz --Osize --asserts none \
  --generate-checked-local-table /path/new-word-table.json \
  --checked-local-solver /path/to/z3

go run ./cmd/minzc ../examples/nanz/mul16_gpu_test.nanz --Osize --asserts none \
  --checked-local-table /path/new-word-table.json -o /path/selected.a80
```

Генератор теперь понимает этот оператор; всё равно создаёт exactfunction/ABI/ISA-bound assignment certificate. Исторические ENRTwordtables не выдаются за использованные данным замером.

## Проверки/evidence

`TestLocalWordMultiplyBoundsAndBinding`: constant bounds/dynamic RHS/original MIR immutable/temp SSA unique/changed multiplierновый ключ/signed word preparation. `TestCheckedActualWordMultiplication`: exactoriginalsource плюс явноизменённыйcallerfixture, все 65 536native каждогоhelper/rawcandidate иactualCALLwrapper; wholemoduleBINсравнение/negative3/10.

Exact source/ASM/BIN/table/notes/native logs и SHA manifest — `reports/artifacts/checked-word-mul-evidence.tar.xz`. FullVIR и focusedchecks проходят отдельно; fullpipelinePASS не заявляется. Это расширение полезного productioncheckedtables domain на реальномsource, не общее утверждение о переносе всех LIR/VIRоптимизаций.
