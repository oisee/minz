# Повторное использование двухрегистровой исторической формы для weighted

Историческая ENRT4v-запись №5 теперь даёт проверенное назначение для всей функции `weighted` из неизменённого `06_pbqp_weighted.nanz`, хотя текущая подготовленная задача содержит 13 SSA-регистров. Сохранение входного `x`, все арифметические операции и return move остаются в исходной задаче. Историческое решение разворачивается на все 13 переменных; `AddChecked` проверяет полную текущую задачу и создаёт полный сертификат.

Это свёртка **переменных поиска**, не изменение программы и не расширение call/CFG admission. Все byte ALU destinations, реальный вход A и return A уже принудительно находятся в A. Если эти разные значения не имеют одновременных live edges, они образуют одну переменную назначения A. Сохранённый `x` образует вторую. Получается историческая форма `[A, any-GPR8]` с одним interference edge. Реальная запись №5 предлагает A/C. Прочитано 172 байта файла; SHA префикса `18db326cf50ec63dfc1653a9835cc0edfec7b169a4284ce0574d402530025052`, это не SHA полного ENRT.

Почему было 13: вход, девять последовательных add-result, финальный add-immediate, return adapter и одна действительно нужная копия входа. Пиковое число одновременно живых значений значительно меньше общего числа SSA-регистров. Удалять копию входа было бы неверно: первая destructive операция затирает A, а x нужен далее.

## Результат на native

Полная программа с неизменённой исходной функцией и явно добавленным caller `weighted(x)+x`, сохраняющим x через CALL:

| Программа | Полный BIN | Helper с bootstrap | Caller с bootstrap |
|---|---:|---:|---:|
| Оригинальный factor10 | 44 → 20 B | 182 → 86 T | 238 → 142 T |
| Контроль factor9: одно сложение удалено | 40 → 19 B | 166 → 82 T | 222 → 138 T |

Каждая проверена на всех 256 входах через raw HIR→VM, оптимизированную VM и native с независимым ожиданием `factor*x+1 mod256`, а caller — `(factor+1)*x+1 mod256`. Native SP и весь скомпилированный image неизменны. Factor9 — явно изменённая контрольная программа, не замена оригинального результата: она использует ту же историческую запись, но получает свежий текущий полный ключ.

Для оригинальной программы без caller текущий обычный CLI получает 12-байтное тело. Её prior baseline 36 байт; table-selected 12 байт. Это уже достигалось свежим solver generation; новая польза состоит в повторном применении **готовой исторической формы**, без Z3 и без перебора перестановок назначения.

## Допуск и границы

Сливаются только значения с одним и тем же принудительным физическим byte-регистром. Непринудительные переменные остаются отдельными. Внутреннее ребро жизни, конфликт singleton hints/ABI, pair location, непредставимый byte ABI, неверная ширина или неподдержанный opcode отклоняют quotient. A/C представлены историческими singleton locsets; другие точные ABI здесь не расширяются. Hints с несколькими locations консервативно отклоняются.

Ни одна VIROp не удаляется и не переназывается. Полный current context, ABI, op identities, immediates, pattern scratch/flags и стоимость moves остаются в checked certificate. Старый cost не принимается. `LookupChecked`, дефолтный pipeline и обычный прямой ENRT import не меняются. Calls и CFG в локальный допуск не добавлены. Общая оптимальность не заявляется.

## Как использовать

Из `minzc/`:

```sh
go run ./cmd/import-enrt --quotient \
  --source ../examples/nanz/06_pbqp_weighted.nanz \
  --enrt /mnt/safe/z80-compiler/tables/enriched_4v.enr \
  --out /path/new-table.json --evidence /path/new-evidence.json

go run ./cmd/minzc ../examples/nanz/06_pbqp_weighted.nanz \
  --Osize --asserts none --checked-local-table /path/new-table.json \
  -o /path/weighted.a80
```

`--quotient` явно включает этот способ поиска вместо direct full-vreg shape. Артефакты содержат группы исходных vreg, полную проверенную таблицу и причины отказов. Один прогретый запуск: подготовка original source ≈1.7 ms, import вместе с полной independent admission ≈7.1 ms; это один host wall-time sample, не benchmark памяти/cold-I/O.

## Проверки и evidence

`TestIndexedQuotientForcedDomainsAndLifetime`: настоящий dying/live input, внутреннее ребро merged-group, конфликт instruction domain, unbound ABI, pair alias, непредставимый byte ABI, многозначный hint, stale return ABI. `TestIndexedQuotientWeightedNative`: оригинал и factor9, helper/caller all256, original Ops byte-for-byte неизменны, строго меньшие полные BIN/native T.

Полный VIR suite и целевые indexed tests запускались отдельно. Полный pipeline PASS этим пакетом не заявляется. Exact исходники/ASM/BIN/tables/provenance/logs сохранены в `reports/artifacts/weighted-quotient-evidence.tar.xz` с SHA manifest; compiler executable не включён в компактный repo packet.
