# C89: volatile-источник и обычный результат структуры

16 октября 2026 года. Дополнительное доказательство границы декомпозиции;
проверка дополняет механизм scalar ABI, интегрированный через PR #170.

```c
struct Pair { unsigned char a, b; };
struct Pair make(void) {
    volatile struct Pair p;
    published = (unsigned int)&p;
    ready = 1;
    p.a = 3;
    p.b = 5;
    return p;
}
```

`published` и `ready` отмечают адрес и начало наблюдений теста. Они нужны
для проверки конкретного source-объекта, а не являются API возврата структуры.
После инициализации возврат должен прочитать каждый byte поля один раз.
Caller складывает `a*256+b` и получает 773.

C lowering сначала копирует volatile-объект в обычное значение. У source
остаются адресуемое хранилище и volatile обращения. Обычное значение уже
может вернуться через два result slots. Требование обязательно сохранять
буферный ABI для этого результата было бы избыточным: volatile относится
к обращениям к исходному объекту, а не ко всем последующим копиям значения.
Поле/параметр, остающиеся volatile, проверяются другими memory-fallback cases.

Новая строка `TestCLocalVolatileTrafficNativeVM/aggregate_return` требует:

- materialized storage в `make` на raw и final MIR;
- точную последовательность source traffic: write a=3, write b=5, read a=3,
  read b=5, без пропущенных, дополнительных или переставленных обращений;
- результат 773 на raw/final VM и собранном Z80;
- native return PC/SP и исходное interrupt state для IFF=0 и IFF=1.

Три режима × два VM этапа дают 6 VM вызовов; три режима × два IFF состояния —
6 native запусков. PASS, 0.045 s. Прежние volatile access, local traffic,
aggregate parameter fallback и aggregate behavior tests — PASS 1.773 s.
Нормативные ссылки: C90 5.1.2.3, 6.5.3 и правила aggregate value return.

```sh
cd minzc
go test ./pkg/c89 -run '^TestCLocalVolatileTrafficNativeVM/aggregate_return/' -count=1 -v
```

`SYSTEM_C_VOLATILE_EXPORT` сохраняет C, ASM и BIN. Source traffic наблюдается
при исполнении; сохранённый ASM сам по себе не доказывает число обращений.
Первое экспериментальное утверждение о mandatory caller-buffer было отклонено
по HIR и семантике copy. Это исправление тестового требования, не найденный
miscompile; итоговые ожидания обращений и результатов не ослаблялись.
