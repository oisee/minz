# IRC на Z80: Почему Nanz, а не Frill или Lizp

*Апрель 2026*

---

У MinZ восемь фронтендов. Три из них — Nanz, Frill, Lizp — «свои» языки, спроектированные для компилятора. Все три компилируются в один и тот же MIR2 IR и в один и тот же Z80 бэкенд. Вопрос не «можно ли написать IRC клиент на Frill» — можно. Вопрос — на каком языке код будет *яснее*, абстракции *дешевле*, а Z80 asm *лучше*.

Мы выбрали Nanz. Вот почему.

---

## Задача

IRC клиент + сервер на Z80 (3.5 МГц, 64КБ ОЗУ). Хост (эмулятор MZV) берёт на себя TCP/TLS. Z80 программа видит I/O порты:

- **Порт $30** — данные: чтение/запись байт в сеть
- **Порт $31** — управление: подключение/отключение/статус
- **Порт $23** — консоль (VT100 терминал через TUI stdlib)

Z80 код должен:
- Парсить IRC протокол (текстовые строки, `:prefix COMMAND params :trailing`)
- Хранить состояние (ники, каналы, статусы подключений)
- Рисовать TUI (статус-бар, область сообщений, строка ввода)
- Обрабатывать клавиатуру (`/join`, `/msg`, `/quit`)
- Для сервера: мультиплексировать 8 клиентских подключений, рассылать сообщения

---

## Почему Nanz

### 1. Мутабельное состояние — естественно

IRC — это состояние. Ники меняются. У каналов есть списки участников. Буферы строк накапливают байты. Сервер отслеживает 8 клиентских слотов.

**Nanz:**
```nanz
enum ClientState { Empty, Connected, Registered }

global client_state: [u8; 8]
global srv_buf: [u8; 512]
global srv_pos: u16 = 0
```

Массивы, глобалы, мутабельные переменные — всё маппится 1:1 на память Z80. Никаких аллокаций, никакой косвенности. `client_state[slot]` компилируется в индексированную загрузку. `srv_pos = 0` — в `LD (srv_pos), 0`.

**Frill** (ML-стиль, чистый функциональный) — мутация требует `ref` или протаскивания состояния:

```frill
(* Каждое изменение = новое значение через все функции *)
let poll_network (state : ServerState) : ServerState =
    let b = net_read () in
    if b == 0 then state
    else { state with buf = append state.buf (b & 0x7F) }
```

Красиво в теории. Но на Z80 протаскивание структуры через каждую функцию — это копирование байт на стеке. Компилятор *может* оптимизировать. А может и нет. На 64КБ — «может» не годится.

**Lizp** (S-выражения, Lisp) — мутация поддерживается, но ощущается чужеродно:

```lizp
(define client-state (make-vector 8 'empty))
(vector-set! client-state slot 'registered)
```

Работает, но vector-операции тяжелее прямого индексирования массива. И кавычки вокруг символов (`'empty`, `'registered`) — это runtime-сравнение тегов вместо compile-time целых чисел.

### 2. Enum + Match = диспатч IRC команд

У IRC маленький набор команд (PRIVMSG, JOIN, PART, QUIT, PING, числовые ответы). Идеальный случай для enum:

**Nanz:**
```nanz
enum IrcCmd { Ping, Privmsg, Join, Part, Quit, Numeric, Unknown }

fun classify_irc(buf: ^u8, pos: u16) -> IrcCmd {
    if buf_starts_with(buf, "PING")          { return IrcCmd.Ping }
    if region_match(buf, pos, "PRIVMSG", 7)  { return IrcCmd.Privmsg }
    if region_match(buf, pos, "JOIN", 4)      { return IrcCmd.Join }
    return IrcCmd.Unknown
}
```

Enum компилируется в целочисленные константы (0-6). Classify — в цепочку сравнений строк. На Z80 каждый `region_match` — плотный цикл `CP (HL)`. Именно то, что написал бы человек руками.

**Frill** — алгебраические типы данных, *мощнее*:

```frill
type IrcCmd =
    | Ping
    | Privmsg of string * string   (* канал, сообщение *)
    | Join of string
```

Элегантно — payload (имя канала, текст) живёт внутри варианта. Но на Z80 конструкторы ADT аллоцируют tagged tuples. Для парсера протокола, обрабатывающего сотни строк в секунду, аллокация на каждую строку — расточительство. Enum в Nanz — целые числа, нулевая стоимость.

**Lizp** — символы или тегированные списки:

```lizp
(case (classify-command line)
  ('ping (handle-ping line))
  ('privmsg (handle-privmsg line)))
```

Чисто, но `case` на символах — runtime-сравнение строк. `match` в Nanz на enum — compile-time целочисленное сравнение: `CP 3 / JR Z` на Z80.

### 3. Указатели = парсинг протокола

Парсинг IRC — это работа с указателями. Пропустить префикс (`:nick!user@host`), найти команду, извлечь trailing-сообщение после `:`.

**Nanz:**
```nanz
if srv_buf[0] == 58 {   // ':'
    pos = 1
    while srv_buf[pos] != 33 && srv_buf[pos] != 32 {
        pos = pos + 1
    }
    nick_len = pos - 1
}
```

`srv_buf[pos]` — загрузка по указателю: `LD HL, srv_buf / ADD HL, pos / LD A, (HL)`. `pos = pos + 1` — `INC`. Тот же код, что написал бы C-программист, та же Z80 инструкция.

**Frill** — рекурсивный спуск:

```frill
let skip_prefix (line : ^u8) (pos : u8) : u8 =
    if peek (offset line pos) == ' ' then pos
    else skip_prefix line (pos + 1)
```

Красиво. Но каждый рекурсивный вызов — CALL на Z80 (17T + 10T RET). While-цикл — 12T `JR NZ`. Для парсера, работающего на каждой входящей IRC строке, 27T против 12T на символ имеет значение.

### 4. @extern = чистый интерфейс к железу

Z80 общается с сетью через I/O порты. `@extern` в Nanz объявляет их как внешние функции:

```nanz
@extern fun net_read() -> u8       // IN A, ($30)
@extern fun net_write(b: u8)       // OUT ($30), A
@extern fun ctl_write(b: u8)       // OUT ($31), A
```

MZV перехватывает вызовы как host-функции. Z80 код вызывает `CALL net_read` — MZV делает I/O, возвращает результат. Нулевая церемония.

**Frill** — тоже может (`extern`), но побочные эффекты в чистом функциональном языке без IO-монады чувствуются чужеродно.

**Lizp** — `(foreign-call "net_read")`. Более многословно, плюс overhead Lizp calling convention на каждой I/O операции.

### 5. TUI stdlib написана на Nanz

Библиотека TUI (`stdlib/tui/render.nanz`) — это Nanz. Импорт — одна строка:

```nanz
import tui.render
```

Из Frill или Lizp нужны кросс-языковые импорты (работают — все фронтенды компилируются в MIR2), но сигнатуры типов не совпадают естественно. Frill ожидает каррированные функции; TUI — многоаргументные. Lizp — list-based вызовы; TUI — позиционные параметры.

Оставаясь в Nanz, `tui_goto(x, y)` работает как есть.

### 6. for-in → DJNZ

Главный цикл IRC поллит 8 клиентов, читает до 32 байт за тик, итерирует ввод. `for-in` в Nanz — идеально:

```nanz
for slot: u8 in 0..MAX_CLIENTS {
    if client_state[slot] != ClientState.Empty {
        poll_client(slot)
    }
}
```

`for i in 0..n` компилируется в DJNZ — самую плотную итерацию на Z80. В Frill — `List.iter` или рекурсия. В Lizp — `do`/`for-each`. Оба работают, но ни один не даёт DJNZ так естественно.

---

## Стоп — У Nanz тоже есть Pipes!

Распространённое заблуждение: «У Frill есть `|>`, у Nanz нет». Неправда. У Nanz **пять** форм композиции функций — больше чем у Frill:

```nanz
// 1. Value pipe (как Frill |>)
5 |> double |> inc                           // → 11

// 2. Pipe с частичным применением (Frill ТАК НЕ МОЖЕТ)
5 |> add(3) |> mul(2)                        // → 16

// 3. Именованный pipe (уникально для Nanz)
pipe doubled { map(|x: u8| x + x) }
range(0..5).apply(doubled).fold(0, add_acc)  // → 30

// 4. Trans — композиция pipe'ов (уникально для Nanz)
trans doubled_inc { use doubled; map(|x| x + 1) }

// 5. UFCS цепочки
buf.map(|x| x*2).filter(|x| x > threshold).forEach(|x| process(x), n)
```

Z80 выход — **каждая pipe-цепочка с константными аргументами сворачивается в одну инструкцию:**

```z80
test_basic_pipe:       LD A, 11 / RET    ; 5 |> double |> inc
test_pipe_with_args:   LD A, 16 / RET    ; 5 |> add(3) |> mul(2)
```

Именованные pipe'ы с `range().apply().fold()` дают DJNZ циклы. Nanz не жертвует функциональной композицией ради императивной производительности. У него есть и то, и другое.

---

## Где Frill был бы лучше

**Деструктуризация ADT.** Если бы парсинг IRC был бо́льшей частью кода, паттерн-матчинг Frill и ADT бы сияли:

```frill
let display (msg : IrcMsg) : unit = match msg with
    | Privmsg { sender; body; _ } -> printf "<%s> %s" sender body
    | Join { nick; channel } -> printf "%s joined %s" nick channel
    | Ping s -> send_pong s
```

Pipe-цепочка `split_prefix |> match_command |> extract_trailing` читабельнее вложенных while-циклов. Деструктуризация `Privmsg { sender; body; _ }` чище ручного отслеживания смещений.

**Гибридный подход (будущее):** парсер на Frill, main loop на Nanz. Кросс-языковые вызовы уже работают.

---

## Где Lizp был бы лучше

**Конфигурация и скриптинг.** IRC бот с пользовательскими алиасами, авто-джойном, скриптовыми ответами:

```lizp
(define-alias "/hi" (lambda ()
    (send-privmsg current-channel "Hello from Z80!")))

(define auto-join '("#minz" "#z80" "#retro"))
(for-each join-channel auto-join)
```

Код-как-данные: IRC клиент может загружать `.lizprc` конфиги и исполнять их. Nanz не имеет `eval` — это компилируемый язык.

---

## Что мы построили

### Клиент (examples/nanz/irc_client.nanz)
- 4 enum: `ConnStatus`, `IrcCmd`, `UserCmd`, `Color`
- Match-диспатч для IRC команд и клавиатурного ввода
- Control port $31: автономное подключение (TCP по умолчанию, TLS опционально)
- TUI: статус-бар, цветные ники, строка ввода
- Команды: `/join`, `/part`, `/msg`, `/nick`, `/me`, `/topic`, `/quit`, `/connect`, `/tls`, `/raw`
- 40 функций → 2212 строк Z80 asm

### Сервер (дизайн, следующая реализация)
- 8 клиентских слотов через мультиплексированный порт $40-$42
- Регистрация NICK/USER, JOIN с names/topic, рассылка PRIVMSG
- PING/PONG keepalive, уведомления QUIT
- ~350 LOC Nanz, ~60 LOC Go (MZV listener)

### MZV Host
- Порт $30: данные (чтение/запись, неблокирующий)
- Порт $31: управление (connect/disconnect, TCP по умолчанию, TLS через `T`)
- Порт $40-$42: серверный мультиплексор
- `--net host:port` для клиента (pre-connected)
- `--listen :port` для сервера
- TLS прозрачно через Go `crypto/tls`

---

## Цифры

| | Nanz | Frill | Lizp |
|---|------|-------|------|
| Мутабельное состояние | Нативно | Неуклюже (ref/threading) | Нативно но тяжело |
| Enum → Z80 | CP N / JR Z (4T) | ADT tag + alloc | Symbol compare (runtime) |
| Парсинг указателями | LD A,(HL) / INC HL | Рекурсия (CALL 27T) | string-ref (indirect) |
| I/O порты | @extern (0 overhead) | extern + IO wrapper | foreign-call (verbose) |
| TUI импорт | Один язык | Кросс-языковой адаптер | Кросс-языковой адаптер |
| for-in → DJNZ | Прямой | List.iter → call chain | do → call chain |
| **IRC клиент LOC** | **~350** | ~450 (оценка) | ~500 (оценка) |
| **IRC сервер LOC** | **~350** | ~500 (оценка) | ~550 (оценка) |

---

## Вывод

Nanz выбран потому что IRC — это *системная* задача: мутабельное состояние, побайтовый парсинг, поллинг I/O, буферы фиксированного размера. Nanz обрабатывает всё это с нулевым overhead — каждая абстракция (enum, match, for-in, @extern) компилируется в ровно тот Z80 код, который написал бы человек руками.

Правильный ответ для реального проекта: **Nanz для движка, Frill для парсера, Lizp для скриптового слоя.** Три языка, один IR, один бинарник. Для этого восемь фронтендов и существуют.

---

*IRC на Z80. Nanz — двигатель, Frill — красота, Lizp — душа.*
