# Исправлено переполнение входа ZSQL ZX

Исходный буфер расширен до явного массива 61 байт при лимите 60 символов. Реальные native вызовы readline на границах длины и полный SQL main проходят в трёх режимах, с точным RAM и экраном. [Разбор и сохранённое переполнение](../../reports/zsql-zx-capacity/README.md).

Состав и категории всех 194 путей не изменены: VM/Z80 independent/bounded 153, native ASM 17, native CP/M 10, SMC 2, inline ASM audit 4, Agon 5, pending oracle 2, иной memory model 1. Freeze прежнего исходника сохранён отдельно; reviewed SHA обновлён только для исправленного ZSQL ZX. Полный pipeline94a41120 ещё исполняется на прежнем executable.
