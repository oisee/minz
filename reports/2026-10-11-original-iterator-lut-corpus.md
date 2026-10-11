# Исходные LUT/forEach/mapInPlace: найден и исправлен miscompile Grace

Основа `fe51f4f7`. Запрошенного March11 `ex5_lut.nanz` нет: проверен фактический March10 источник. March10 ex5/ex6/ex7 и March13 копии прочитаны отдельно; полные SHA256 равны попарно. Источники не изменены. Домены и модели зафиксированы в `frozen-domain.json` до исполнения.

В исходном `max_chain` Grace ошибочно удалял сравнение maximum с очередным байтом. Empty-block elimination сводил обе ветки к одному join, но передавал разные значения: новый байт и прежний maximum. Правило trivial-branch сравнивало только имя target, затем безусловно выбирало ThenArgs. Для входа `[0,73,146,219,36]` длины 5 raw VM возвращала 219, final VM и Z80 — 36. Этот before log и ASM/MIR сохранены отдельно.

Исправлены predicate и action: branch можно заменить jump только при одинаковом target **и одинаковой последовательности аргументов**. Проверены оба порядка различных аргументов, различный порядок/длина векторов, допустимые одинаковые векторы и nil/empty без аргументов. После исправления исходный контрпример даёт 219 на всех трёх исполнителях.

Оракулы — непосредственный исходник и Nanz Book v7: popcount через независимый битовый счётчик, сумма по модулю 256, max с начальным 0, mapInPlace добавляет 2 по модулю 256. Все 256 byte inputs popcount; для sum_chain/max_chain/add2_inplace все 256 seeds × все 256 длин. Вход упорядочен как `(seed+73*i) mod256`, значения и длины пересекают 0, 127/128 и 255. У каждого входа проверены input bytes до/после диапазона.

Каждый source entry исполняется на raw VM, final VM и собранном Z80 через настоящий CALL, во всех трёх режимах ordinary/Grace Size/Grace Speed. Для iterator memory проверяются точные ordered OpLoad/OpStore события VM и реальные byte-load/store инструкции Z80: n чтений, для mapInPlace ещё n записей на те же последовательные адреса. Captured sum/max state и результат проверены; callback-код не заменялся другой helper-функцией. Это конечный byte/length domain, не все возможные массивы длины 255.

589 824 iterator CALL и 768 popcount CALL за три режима (590 592 всего). После домена сверены все 64 KiB RAM вне setup input и фактически занятого stack interval; каждый input и guards сверяются отдельно на каждом вызове. Код и LUT bytes неизменны. Попcount BIN 512 B во всех режимах (обычный pipeline создал LUT); foreach BIN 61 B ordinary и 71 B Grace, mapInPlace 22 B во всех режимах. Победитель оптимизации не выбирался.

Cumulative gas limit повторно используемого VM достигался от количества последовательных корректных вызовов; test-only MaxSteps=0 убирает общий лимит, native остаётся ограничен 200 000 шагами на CALL. Исходный workload и ожидания не сужены. Retained executable, исходники, ABI, ASM/BIN, raw/final MIR, CALL drivers, результаты и gate logs включены в manifest. Общие pipeline/CLI gates не повторялись.

Поднятая lambda_0 из mapInPlace сохранена в raw MIR и в исходном полном snapshot; production lowering встраивает её в original add2_inplace и затем удаляет. Отдельный native вызов lambda_0 не используется вместо исходной функции. Full MIR2 package PASS 70,708 s.

Финальный retained gate с native access-order проверками PASS (около 94 s; wall time зависит от нагрузки host), full MIR2 package PASS.
