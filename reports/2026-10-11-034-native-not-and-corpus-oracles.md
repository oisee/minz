# Z80 NOT и явное покрытие корпуса

На `ff11ba69` интегрирован исправленный typed NOT. До исправления оригинальный
Pascal bubble sort сортировал на VM, но на Z80 возвращал массив после одного
прохода. Backend выполнял CPL над A, игнорируя реальные Src/Dst, и неверно
инвертировал однобитный bool. Теперь allocated homes учитываются, bool использует
XOR 1, целые — побайтовый complement.

744 прямых случая и реальные workloads bubble sort, Arena, PL/M arithmetic и
Pascal records прошли VM и собранный Z80. Root повторил интегрированные NOT,
новые native workloads и каталог: MIR2 0.606 s, pipeline 0.636 s, Nanz 0.051 s,
exit 0. Итоговые массивы и состояния проверяет независимая модель.

Каталог содержит 193 точных пути с source SHA, entry, extern и ASM-контрактами.
Семь файлов имеют привязанное здесь ограниченное независимое покрытие; это не
утверждение, что остальные никогда не тестировались. Полный compile gate на
cb587fe4 дал 192 PASS и один явный native-capacity отказ host ZVM. Исполнение
всего корпуса ещё не доказано. Полный pipeline handle 2903 остаётся живым;
terminal результат пока отсутствует.

Пакет: [описание](../docs/grace-showcase-native-oracles.md),
[manifest](../docs/grace-showcase-native-oracles/manifest.json),
[root repeat](../docs/grace-showcase-not-integrated.txt).
14 файлов пакета скопированы на `/mnt/safe/minz/grace/2026-10-11/compiler-docs/`
и сверены SHA256; отдельный `integration-034-sha256.json` сохраняет результат.
Обновлена общая [сводка](../docs/grace-current-state.md). Изменения находятся
в feat/grace-hir-vm; новая публикация Sites пока не выполнялась.

Следующая работа: checked-local таблицы в обычном CLI, независимое native
покрытие дополнительных алгоритмов, effect-safe миграция memory fusion.
Общая цель остаётся активной.
