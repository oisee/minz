# Перепроверка исторических register assignments и CUDA audit

Snapshot `ae8a058b`, importer `c736ab72`, CUDA report `ae8a058b`.
Root заново выполнил importer unit test и весь opt-in corpus test с сохранённым
образцом первых 4096 ENRT4v records: VIR PASS 0.012 с, pipeline PASS 1.407 с.
Точное время VIR см. в логе; pipeline содержит реальные all256 native проверки
imported double/low_nibble, а не только skip export-теста.

[Импорт](../../reports/2026-10-11-legacy-candidate-import.md) принимает старые
распределения как предложения; ABI, ISA, interference, scratch и стоимость
проверяются заново через AddChecked. Текущий LookupChecked не ослаблен.
Из образца получены допустимые записи; все пять проверенных модулей сохраняют
baseline BIN size. Пример первых records не доказывает полный indexed lookup.
Следующий шаг — адресовать реальные shape records по проверенному enum layout.

[CUDA audit](../../reports/2026-10-11-063-cuda-z80-screen-audit.md): найденные
partial ALU evaluators, сохранённый executable и 512 GPU/CPU сравнений.
Это предварительный arithmetic screen, не полноценный Z80 VM и не новый
доказанный выигрыш скорости стенда. Полные SHA есть для небольших таблиц;
для больших файлов сохранены размеры и SHA заголовков, это разные доказательства.

Оба manifest проверены root, каждый payload отслеживается Git; точные копии
сохранены в canonical compiler-reports на `/mnt/safe/minz/grace/2026-10-11`.
Полный параллельный CLI и старый полный pipeline ещё не завершены.
