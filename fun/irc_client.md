# IRC Client on Z80 — Architecture & Protocol Guide

**300 LOC Nanz → 1594 lines Z80 asm → live IRC on 3.5 MHz**

---

## Architecture: What Goes Where

```
┌─────────────────────────────────────────────────┐
│  IRC Server (irc.libera.chat:6667)              │
└───────────────┬─────────────────────────────────┘
                │ TCP socket
┌───────────────┴─────────────────────────────────┐
│  Host: MZV (Go)                                  │
│                                                   │
│  ┌─────────────┐   ┌──────────────┐              │
│  │ TCP client   │   │ VT100 term   │              │
│  │ ↕ port $30   │   │ ↕ port $23   │              │
│  └──────┬──────┘   └──────┬───────┘              │
│         │                  │                      │
│  ┌──────┴──────────────────┴───────┐              │
│  │       Z80 VM (emulator)         │              │
│  │       64KB RAM, 3.5 MHz         │              │
│  └─────────────────────────────────┘              │
└───────────────────────────────────────────────────┘
```

**Три слоя:**

| Слой | Что делает | Язык | LOC |
|------|------------|------|-----|
| IRC Server | Маршрутизация сообщений, управление каналами | — | — |
| MZV Host | TCP connect, DNS resolve, port I/O bridge | Go | ~30 |
| Z80 Client | IRC parsing, TUI rendering, user input | Nanz | ~300 |

**Принцип:** Z80 программа не знает про TCP/IP. Она видит один I/O порт:

```
Port $30:
  IN  A, ($30)  →  $00 = нет данных, $80|byte = байт от сервера
  OUT ($30), A  →  отправить байт серверу
```

Та же конвенция что console port $23. MZV делает всю сетевую работу.

---

## IRC Protocol: Что Нужно Знать

IRC — текстовый протокол. Строки заканчиваются `\r\n`. Максимум 512 байт на строку.

### Команды от клиента серверу

| Команда | Формат | Когда |
|---------|--------|-------|
| **NICK** | `NICK minz_user\r\n` | При подключении — задать ник |
| **USER** | `USER minz 0 * :MinZ IRC\r\n` | При подключении — регистрация |
| **JOIN** | `JOIN #channel\r\n` | Войти в канал |
| **PART** | `PART #channel :reason\r\n` | Покинуть канал |
| **PRIVMSG** | `PRIVMSG #channel :hello\r\n` | Отправить сообщение |
| **PONG** | `PONG :server\r\n` | Ответ на PING (keepalive) |
| **QUIT** | `QUIT :bye\r\n` | Отключиться |
| **NICK** | `NICK newnick\r\n` | Сменить ник |

### Команды от сервера клиенту

| Формат | Значение | Пример |
|--------|----------|--------|
| `PING :irc.libera.chat` | Keepalive — ОБЯЗАТЕЛЬНО ответить PONG | автоматически |
| `:nick!user@host PRIVMSG #chan :text` | Сообщение в канале | `<alice> привет` |
| `:nick!user@host PRIVMSG mynick :text` | Личное сообщение | `<bob> hey` |
| `:nick!user@host JOIN :#channel` | Кто-то вошёл в канал | `alice joined` |
| `:nick!user@host PART #chan :reason` | Кто-то вышел | `bob left` |
| `:nick!user@host QUIT :reason` | Кто-то отключился | `charlie left` |
| `:server 001 nick :Welcome` | Приветствие сервера | показываем raw |
| `:server 353 nick = #chan :nick1 nick2` | Список пользователей | `/names` ответ |
| `:server 372 nick :- MOTD text` | Message of the Day | показываем dim |

### Числовые коды (важные)

| Код | Значение |
|-----|----------|
| 001 | RPL_WELCOME — успешная регистрация |
| 332 | RPL_TOPIC — тема канала |
| 353 | RPL_NAMREPLY — список ников в канале |
| 366 | RPL_ENDOFNAMES — конец списка ников |
| 372 | RPL_MOTD — строка MOTD |
| 376 | RPL_ENDOFMOTD — конец MOTD |
| 433 | ERR_NICKNAMEINUSE — ник занят |

---

## Что Делает Наш Клиент

### Автоматически

1. **PONG** — отвечает на PING сервера (без этого сервер отключит через ~120с)
2. **NICK + USER** — регистрируется при старте
3. **Echo** — показывает свои сообщения локально (сервер не шлёт наши PRIVMSG обратно)

### По команде пользователя

| Ввод | Действие |
|------|----------|
| текст + Enter | `PRIVMSG #channel :текст` |
| `/join #chan` | `JOIN #chan` |
| `/nick newnick` | `NICK newnick` |
| `/quit` | `QUIT :MinZ IRC signing off` |

### Отображение

```
┌─ MinZ IRC v0.1 — #minz ─────────────────────────┐
│                                                    │  ← status bar (blue)
│ <alice> hello everyone                             │
│ <bob> hey alice! what are you running on?          │  ← messages area
│ <alice> Z80 at 3.5 MHz :)                          │     cyan nicks
│ charlie joined                                     │     green joins
│ dave left                                          │     red parts
│                                                    │
│                                                    │
│                                                    │
│> hello from minz_                                  │  ← input line
└────────────────────────────────────────────────────┘
```

---

## Что Нужно Доделать в MZV

### Port $30 Handler (~30 LOC Go)

```go
// В pkg/emulator/z80_remogatto.go или cmd/mzv/net.go

type NetPort struct {
    conn net.Conn
    buf  chan byte  // buffered channel, non-blocking read
}

func (n *NetPort) Start(addr string) error {
    conn, err := net.Dial("tcp", addr)
    if err != nil { return err }
    n.conn = conn
    n.buf = make(chan byte, 4096)
    go func() {  // read goroutine
        b := make([]byte, 1)
        for {
            if _, err := conn.Read(b); err != nil { return }
            n.buf <- b[0]
        }
    }()
    return nil
}

// IN A, ($30) — non-blocking read
func (n *NetPort) Read() byte {
    select {
    case b := <-n.buf:
        return 0x80 | b
    default:
        return 0x00
    }
}

// OUT ($30), A — write
func (n *NetPort) Write(b byte) {
    n.conn.Write([]byte{b})
}
```

### CLI Flag

```bash
mzv examples/nanz/irc_client.nanz --net irc.libera.chat:6667
```

MZV connects TCP, wires port $30, then launches the Z80 program.

---

## Возможные Расширения

### Фаза 2: Полезные фичи
- `/names` — показать список пользователей (parse 353)
- `/topic` — показать тему (parse 332)
- `/msg nick text` — личные сообщения
- Scrollback buffer (256 строк в памяти Z80)
- Timestamp per message

### Фаза 3: Красота
- Nick coloring — hash ника → ANSI цвет (8 цветов)
- Word wrap для длинных сообщений
- Status bar с topic + user count
- Activity indicator (мигающий символ при incoming data)

### Фаза 4: Frill Parser
Чистый функциональный парсер IRC строк на Frill:

```frill
let parse_irc (line : ^u8) : IrcMsg =
    line |> skip_prefix |> parse_command |> extract_trailing
```

### Фаза 5: Multi-channel
- Tab switching между каналами
- Per-channel scrollback
- `/list` — список каналов
- `/who` — info о пользователе

---

## Почему Это Красиво

1. **Нет TCP/IP стека на Z80.** Сетевой стек — это тысячи строк кода. Мы вынесли его в хост. Z80 видит один порт. Чистое разделение.

2. **IRC — текстовый протокол.** Идеально для 8-bit: парсинг строк, сравнение регионов, pointer arithmetic. Нет бинарных структур, нет endianness проблем.

3. **TUI stdlib уже есть.** VT100 escape sequences работают и в MZV, и в CP/M терминале. Один код — два runtime.

4. **300 LOC.** Рабочий IRC клиент на языке с ADT, match, closures, zero-cost interfaces — в 300 строках. На C это было бы 500-800 LOC без TUI.

5. **1594 строк Z80 asm.** Компилятор сгенерировал hand-quality assembly из high-level Nanz. Z3-PFCCO оптимизировал calling conventions для 25 функций одновременно.

---

## Тест-план

```bash
# 1. Compile
mz fun/irc_client.nanz -o build/irc_client.a80

# 2. Assemble
mza build/irc_client.a80 -o build/irc_client.com

# 3. Run with network (when MZV net port is ready)
mzv build/irc_client.com --net irc.libera.chat:6667

# 4. In the client:
#    /join #minz
#    hello from Z80!
#    /quit
```

---

*IRC on Z80. Because we can.*
