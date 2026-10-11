# Original outparam_swap: полный byte-domain

Неизменённый `examples/paper/outparam_swap.nanz` проверен целиком: все 65 536 пар u8 для `swap`, столько же для `use_swap` и исходный `main()` — 131 073 вызова в каждом из трёх режимов, всего 393 219 native CALL. Исходник и домен зафиксированы до исполнения в `b67a4fe8`, base `ad092573`.

Модель независима от IR: `swap(x,y)` возвращает **оба** значения `(y,x)` в исходном порядке; `use_swap(x,y)` возвращает `(x+y)&255`, включая переполнение; `main()` возвращает 10. Book v7 §2.2 содержит ровно пример swap с `(b,a)`; §7.5 требует проверки каждого элемента tuple; §3.1 задаёт 8-bit u8. Не используются устаревшие архитектурные утверждения книги. Режимы: ordinary, HIR+MIR Grace Size, HIR+MIR Grace Speed, ContractOpt=true.

Каждый вызов отдельно сопоставлен с исходной MIR VM, final MIR VM и assembled Z80. Native тест действительно исполняет CALL оригинального символа, читает каждую Contract.Returns physical location и останавливается только на правильном PC возврата с восстановленным SP. У main исходный тип u8/RET; DI/HALT-startup ему не приписан. HALT есть только после внешнего проверочного CALL.

Код модуля во всех режимах ровно 6 байт:

```asm
main:      LD A,10
           RET
swap:      RET
use_swap:  ADD A,C
           RET
```

У swap вход a находится в B, b в A, выходы соответственно A и B: RET уже сохраняет правильный обмен. use_swap в final code больше не вызывает swap — tuple projection оптимизирована до ADD; raw VM исполняет исходную композицию. Main constant-folded. Поэтому пакет подтверждает все исходные значения и контракт, но **не** заявляет сохранённый внутренний CALL swap в native или новый выигрыш Grace. Во всех трёх режимах BIN одинаков.

| Entry | Calls/mode | T/call включая внешний CALL | Максимальная глубина stack |
|---|---:|---:|---:|
| swap | 65 536 | 27 | 2 B |
| use_swap | 65 536 | 31 | 2 B |
| main | 1 | 34 | 2 B |

VM memory-access observer подтверждает отсутствие data effects. Native инструкция каждого шага проходит узкую проверку реально сгенерированного набора (CALL, RET, LD A,n, ADD A,C). Единственная возможная запись — CALL: проверены точный SP-2 и оба байта адреса возврата сразу после шага. Итоговое сравнение всех 64 KiB запрещает изменения вне этих stack bytes и объявленных spill homes; spills здесь отсутствуют. Code, guard, вся остальная RAM неизменны. Не обобщаем opcode whitelist на другие программы.

Финальный запуск точного сохранённого executable PASS (лог `native-tests.txt`). Production изменений не потребовалось. Прежний ex13 tuple пакет не дублируется. ASM/BIN/raw/final MIR/ABI/allocation/символы и результаты сохранены в `actual`, frozen executable сжат; SHA manifest покрывает все payloads. Шара: `/mnt/safe/minz/grace/2026-10-11/outparam-swap-native/REPORT.md`. Полный pipeline не запускался.
