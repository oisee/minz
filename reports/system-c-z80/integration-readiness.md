# Nanz + C: готовность к интеграции

16 октября 2026 года. Это проверка требований
[исходной цели](objective.md), а не новая сокращённая цель.
Текущее слияние и required CI видны в [PR #162](https://github.com/oisee/minz/pull/162).

| Требование | Доказательство | Граница доказательства |
|---|---|---|
| Nanz → C и C → Nanz, headers, aliases, globals, structs, несколько модулей | Native и raw/final VM matrices; minimal CLI; source API C → C → Nanz → Nanz; private functions/globals/pointers и разные struct layouts | Авторы пишут C declarations; source interoperability, не чужой binary ABI |
| char8/int16/long32, promotions/conversions/storage/pointers | integer, scalar-conversion, pointer, enum, aggregate/union и native-wide-ABI tests | Used 64-bit/floating values, VLA и executable bit-fields явно отвергаются; полный C99/C17 не заявлен |
| volatile и аппаратная граница | Exact byte event traces глобальных scalar/aggregate устройств и импортированных полей; локальные traces; native PC/SP/IFF; restricted ASM diagnostics | Локальная новая трасса начинается после публикации адреса; ASM допускается по анализируемому контракту |
| Дисковый FAT12 пример | Реальный 1.44-MB образ с фрагментированным HELLO.TXT; независимый upstream FatFS oracle; восемь success/error workloads | Read-only, image-backed sector controller; физический FDC, запись и полный FatFS не заявлены |
| Raw/final VM и собранный Z80, оракулы, ASM/BIN, память | Матрицы сохраняют исходники, HIR, ASM/BIN, логи и SHA256; дисковый runner проверяет PC/SP и bytes; native load errors теперь fatal | PASS в VM или parse сам по себе не заменяет native proof; checks относятся к указанным входам |
| Примеры/документация/main | Guide в README, checked-in interop и FAT12 examples; строгие шесть required checks | До подтверждённого merge и post-merge launches пункт остаётся незакрытым |

Все 432 fast pipeline tests прошли в трёх процессах на source API коде
с усиленными проверками загрузки native BIN. Полный hardware/C/FAT12 focused
прогон прошёл: C89 25.486, MIR2 1.900, pipeline 119.352 секунды.
Локальная volatile matrix дополнительно проверена с INT=0 и INT=1;
private-symbol matrix расширена до восьми семейств и прошла отдельно.
Итоговый полный short core прошёл: HIR 28.321, MIR2 103.320, C89 22.614,
cparse 0.004, Nanz 25.949 и interop 0.002 секунды. Он собран до добавления
последнего struct-tags случая; вся расширенная private matrix имеет отдельный
PASS за 0.655 секунды. Глубокий release profile этим запуском не подменяется.

До окончания цели остаются подтверждённые шесть required checks финального
head против актуального main, merge и следующие команды уже на main:

```sh
go -C minzc run ./cmd/minzc ../examples/system-c/interop/main.nanz --asserts=z80 -o /tmp/system-c.a80
python3 examples/system-c/fat12/create_image.py /tmp/demo.img
go -C minzc run ../examples/system-c/fat12/run.go -image /tmp/demo.img -output /tmp/read.txt
cmp /tmp/demo.txt /tmp/read.txt
go -C minzc run ../examples/system-c/fat12/run.go -image /tmp/demo.img -output /tmp/read-size.txt -size
cmp /tmp/demo.txt /tmp/read-size.txt
```

Прежний a6d1fa98 прошёл все шесть required checks. Его advisory sweep имеет
только пять FAIL → PASS изменений exit status. Lint findings проверены:
ошибки LoadMemory обработаны; CPU memory methods имеют намеренный интерфейс.
Эти результаты не выдаются за зелёные проверки более нового head.
