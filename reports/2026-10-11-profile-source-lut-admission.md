# Профиль: точный допуск source-derived LUT globals

Base `b0e1b456`. Два падения frozen full pipeline `8bf75e92` воспроизведены в current focused run: `TestProfileRealSourceCorpusPopcount` и valid values 5/10 в `TestProfileRangedScalarBoundsAndABI`. Полный frozen pipeline FAIL 2865.894 s не объявляется исправленным целиком этим пакетом.

Причина — не исходная глобальная переменная и не потерянный range ABI. `LUTGen` добавлял readonly `popcount_lut` (`[256]u8`, alignment 256) или `twice_lut` (`[6]u8`), которых нет в raw source MIR. Строгий список `raw.Globals == candidate.Globals` отвергал одобренное compiler-generated преобразование.

Проверка сохраняет точное равенство всего source-global prefix: name/type/init/storage/At/IsConst/vtable metadata/order. Единственный допустимый extension — полный набор LUT globals, независимо заново сгенерированный `LowerModule(source)` + `LUTGen`, с exact DeepEqual metadata и всех table bytes. Название `_lut`, IsConst или сам candidate не дают полномочий. Candidate с неизменённым raw global list остаётся допустимым; произвольные дополнительные constants отвергаются. Изменённые после оптимизации значения таблицы, не совпадающие с source regeneration, также отвергаются. Нового profile schema или расширения runtime authority нет.

Native runtime продолжает разрешать записи только в source globals/resources и bounded stack/spill; новые LUT bytes лежат вне writable ranges. Source snapshots не включают compiler-created LUT. Производственные memory-host admission правила этим исправлением не затронуты.

Проверка:

- Оба целевых теста PASS: all256 popcount inputs, independent bits.OnesCount8 oracle, raw/profile/native/selector path; 7168 T-states, 512 module bytes. Ranged args 4/11/255 по-прежнему отвергаются, 5/10 принимаются; подмена compiled param type u16 по-прежнему отвергается.
- 15 adversarial cases отвергают LUT init/type/alignment/const/order/name/storage/At/vtable, arbitrary extra и missing table, source-global init/type/const/name. Успешны exact regeneration и unchanged no-LUT baseline; u16 split tables и aligned u8 table проверены вместе.
- Affected `TestZ80ProfileWorkload.*` + оба targeted tests PASS 2.047 s.
- Full HIR PASS 32.952 s (`profile-global-full-hir.txt`).

Логи before/after/adversarial/affected сохранены в reports. Полный retained popcount evidence + workload/source manifests — `artifacts/profile-global/manifest.json`. Full pipeline на final commit здесь не запускался, остальных frozen failures пакет не касается.
