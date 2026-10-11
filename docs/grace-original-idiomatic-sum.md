# Original idiomatic array sum: full entry evidence

Проверены оба полных computational entry исходного
[`examples/nanz/02_sum_array_idiomatic.nanz`](../examples/nanz/02_sum_array_idiomatic.nanz):
`sum_array_idiomatic` и `sum_array_chain`. Алгоритмы и исходный файл не изменены.
Каждый вход вызван для всех 256 значений `n:u8` по двум адресам начала
(`0x9001`, `0x90ff`): 1024 вызова плюс отдельный наблюдатель стенда.
Каждая длина получает другой детерминированный массив; повторные вызовы
используют одну persistent машину. Переход через границу страницы покрыт,
переход через адрес `0xffff` этим packet не заявлен.

Семантика ожидания опирается на [Language Book v7](Nanz_Language_Book_v7.md):
§2.5 содержит `for x in buf[0..n]`, §5.1 определяет `ptr.forEach(λ,n)` как
«Execute λ for each of n elements», §5.3 описывает изменяемый захват `sum`.
Это последовательное чтение массива, отдельно от countdown-семантики
`range(lo..hi)` главы 6. Независимый Go-оракул складывает ровно `n` байтов
по модулю 256 и заранее строит упорядоченный список адресов/значений.

Для каждого вызова проверены результат, точный порядок всех чтений,
отсутствие записей и неизменность всех 514 байтов RAM (включая два guard).
Оракул сравнивается с raw MIR VM, final optimized MIR VM и фактически
собранным Z80. Native memory-host runner также проверяет разрешённые
доступы, целостность кода, исходный PC возврата и восстановленный SP.

## Явная граница linker adapter

Полный оригинал объявляет `@extern fun add_to_acc(x:u8)->void`, но ни один
вход его не использует. Generic memory-host admission требует допустимый
builtin contract даже для неиспользуемого extern; подходящего builtin нет.
Тест сначала lowering **полного оригинала**, запрещает любое обращение
к символу `add_to_acc` и любой indirect call во всём MIR, затем удаляет
только точную строку мёртвого объявления из compiled-source и добавляет
отдельный `peek`-наблюдатель. Алгоритмы не копируются и не переписываются.
Будущее появление использования callback завершит тест ошибкой.

Это проверка исходных алгоритмов с явной linker адаптацией, а не заявление
о запуске неизменённого исходника через public CLI или реализации callback.
`source.nanz`, `compiled-source.nanz`, SHA обоих и full original MIR сохранены;
`pre-host-link.a80` — код до добавления native builtin stub, а не ASM
неадаптированного оригинала. Физическое оборудование/CP/M OS не проверены.

## Результат

Focused packet и два прежних array/pointer oracle: **PASS 1.308s**.
Новый packet: **3 984 313 T-states**, **46 B**; это суммарная стоимость workload
в emulator, не сравнительный оптимизационный benchmark. BIN SHA256:
`82a1e4e76f5d3deebf9f53b6045c8fec39c5168dc3b3796b47148eff679f2d66`.
Production miscompile не обнаружен; compiler/backend не изменялись.

Catalog: **194 exact paths**, **36 с независимыми VM/native доказательствами**
и отдельный native-only CP/M runtime demo. Добавлена только эта exact source
строка; остальные ограничения и pending случаи сохранены. Catalog PASS 1.665s.
Полный slow pipeline этим packet повторно не запускался.

[Артефакты и SHA256](grace-original-idiomatic-sum/manifest.json) сохраняют
source, linker provenance, independent model, workload, raw/native observations,
MIR/ASM/BIN и focused/catalog logs. Крупные JSON/MIR упакованы gzip без
изменения содержимого. Тест: `TestOriginalIdiomaticSumReadOrderVMZ80`.
