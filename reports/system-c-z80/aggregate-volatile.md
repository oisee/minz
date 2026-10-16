# Volatile-объекты, копирование и импорт C в Nanz

16 октября 2026 года, `feat/system-c-z80`. Итоговый аудит и интеграция в main
продолжаются. Практическое руководство: [Nanz + C](../../docs/Nanz_C_Interop.md).

## Что исправлено

Обычный aggregate copy выдавал Load/Store без признака volatile. Квалификация
объекта или отдельного поля не доходила до этого пути, хотя поэлементные
обращения уже сохраняли её. Теперь copy отдельно учитывает source и destination,
сохраняет все source bytes до записи destination и отмечает наблюдаемые байты.

StructField сохраняет volatile-квалификатор. Он переносится через вложенные
поля, массивы, union layout, semantic type clone и HIR Dump/Parse. Копирование
сохраняет квалифицированные байты, включая перекрытия. Struct→tuple promotion
и структурные HIR Grace-преобразования не применяются к таким объектам.
Обычные scalar MIR-преобразования продолжают работать.

При by-value вызове и возврате квалифицированный объект материализуется до
перехода через адресный ABI. Иначе callee видел адрес без информации о source.
Общие MIR struct-return buffers тоже сохраняют квалификацию поля.
Nanz получает volatile-факт при доступе к импортированному полю, даже если
сам enclosing global не volatile.

Отдельный путь initializer локальной переменной раньше перечитывал target
после assignment и pre/post update. Он теперь использует общий механизм
expression lowering: значение сохраняется один раз, повторного чтения нет.

## Что именно доказано

К исходным 24 C-сценариям добавлены 15: whole-object read/write, by-value
argument/return, volatile member, array member, вложение, union и шесть
assignment/update initializer. Все 39 × default/Grace/Size проверяют
117 native и 234 raw/final VM исполнения с точными байтовыми трассами,
значениями и native PC/SP. Локальное хранилище в старых local/parameter
сценариях имеет проверку MIR access count и результата; его полный native
трафик не следует автоматически из трасс глобального устройства.

Новый импорт поля в Nanz: три native и шесть VM исполнений. Проверены две
записи и два чтения, итог 1287, сохранность соседнего 16-битного tail и PC/SP.
Предыдущий импорт scalar/array также сохраняет свой отдельный сценарий.
Clone и HIR layout roundtrip проверяют сохранность field qualifier.

Baseline 944933af с новыми whole-object тестами отвергал отсутствие volatile
метаданных. Отдельная проба, пропускающая только этот metadata gate, прошла
численный результат и трассу двух выбранных whole-object сценариев.
Следовательно, эти две пробы показывают потерю контракта, а не доказанный
неверный результат старого Z80. Нельзя выдавать metadata FAIL за miscompile.
Новые tests сверяют реальные события после оптимизаций и дополняют контракт.

```sh
go -C minzc test ./pkg/c89 -run '^Test(CVolatileAccessNativeVM|NanzImportedVolatileFieldNativeVM)$' -count=1
go -C minzc test ./pkg/hir ./pkg/mir2 -run 'TestHIRExplicitOverlappingLayoutRoundTrip|TestCallABIMetadataCloneOwnsNestedTypes' -count=1
```

Экспорты через `SYSTEM_C_VOLATILE_EXPORT`, логи и SHA256:
`/home/alice/dev/minz-artifacts/grace-reports/2026-10-16/system-c-z80/aggregate-volatile/`.
На итоговом memory-коде HIR/MIR2/C89/cparse/interop прошли:
38.036/88.449/31.614/0.004/0.002 секунды. В этом запуске Nanz нашёл
assert alias и изменение каталога; после исправлений весь короткий Nanz
повторно прошёл за 39.578 секунды. Промежуточный FAIL сохранён рядом с
исправленным итогом, а не выдан за общий PASS.
FAT12 повторно прошёл 24 native и 48 VM сценариев на итоговом memory-коде.
Полный fast pipeline memory checkpoint прошёл все 432 проверки в трёх
процессах. Он был собран до последующего исправления assert alias; этот
parser change имеет отдельный полный Nanz PASS и новый required CI после push.

## Прямые assert на импорт C

Новый CLI-пример выявил разницу между обычным вызовом и assert: assert хранил
локальное имя функции, не разрешая alias импорта. Теперь выбранный import,
переименование, qualified name и glob используют действительное HIR-имя;
диагностический Source сохраняет исходное имя из программы.

Четыре формы × три режима × четыре scalar/bool assert: 48 native и 96 VM
исполнений. Сам checked-in example имеет независимые ожидания в
TestSystemCInteropExampleNativeVM: ещё шесть native и 12 VM исполнений.
Каталог намеренно расширен с 198 до 199 путей; это точный учёт нового примера,
не перенос доказательства со сходного синтетического исходника.

```sh
go -C minzc test ./pkg/nanz -run 'TestCImportedAssertNativeVM|TestSystemCInteropExampleNativeVM' -count=1
go -C minzc run ./cmd/minzc ../examples/system-c/interop/main.nanz --asserts=z80 -o /tmp/system-c.a80
```

Руководство явно отделяет источник C от чужого binary ABI, минимальные headers
от полного libc, supported C subset от полного C17 и image controller от FDC.
Standalone C CLI пока не подключает cross-language resolver; документированный
двунаправленный сценарий начинается с Nanz. Этот API-стык остаётся пунктом
итогового аудита перед закрытием системной цели.
