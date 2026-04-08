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

**Принцип:** Z80 программа не знает про TCP/IP. Она видит два I/O порта:

```
Port $30 — Data port (передача данных):
  IN  A, ($30)  →  $00 = нет данных, $80|byte = байт от сервера
  OUT ($30), A  →  отправить байт серверу

Port $31 — Control port (управление подключением):
  OUT ($31), A  →  записать байт в command buffer
  IN  A, ($31)  →  прочитать status/response
```

Та же конвенция что console port $23. MZV делает всю сетевую работу.

### Control Port $31 — Протокол Управления

Z80 программа может сама устанавливать подключения через control port.
Команды пишутся побайтно, завершаются нулём (0x00). Ответ читается через IN.

**Команды:**

| Cmd | Формат (OUT $31 побайтно) | Действие |
|-----|---------------------------|----------|
| `C` | `C host:port\0` | **Connect** — DNS resolve + TCP connect |
| `D` | `D\0` | **Disconnect** — закрыть текущее соединение |
| `S` | `S\0` | **Status** — запросить статус |

**Пример: подключение из Z80 кода:**

```nanz
// Подключиться к irc.libera.chat:6667
fun connect_irc() -> u8 {
    // Пишем команду "Circ.libera.chat:6667\0" в control port
    ctl_write(67)   // 'C'
    send_ctl_str("irc.libera.chat:6667")
    ctl_write(0)    // null terminator = execute

    // Ждём ответ
    var status: u8 = 0
    while status == 0 {
        status = ctl_read()  // IN A, ($31)
    }
    // status: 1 = connected, 2 = DNS error, 3 = connection refused
    return status
}
```

**Ответы (IN $31):**

| Байт | Значение |
|------|----------|
| $00 | Busy — команда ещё выполняется |
| $01 | OK — подключение установлено |
| $02 | DNS error — имя не резолвится |
| $03 | Connection refused |
| $04 | Timeout |
| $05 | Already connected (disconnect first) |
| $FF | No pending command |

**Последовательность для полностью автономного клиента:**

```
Z80 boot → OUT $31: "Circ.libera.chat:6667\0"
         → IN  $31: wait for $01 (connected)
         → OUT $30: "NICK minz_user\r\n"        (data port)
         → OUT $30: "USER minz 0 * :MinZ\r\n"
         → main loop: IN $30 / OUT $30 / keyboard
```

**Или через CLI (pre-connected):**

```bash
mzv irc_client.com --net irc.libera.chat:6667
# MZV connects BEFORE launching Z80, port $30 already active
# Control port $31 reports $01 (connected) on first IN
```

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

### NetPort Handler (~80 LOC Go)

```go
// cmd/mzv/net.go — network I/O ports for Z80 programs

type NetPort struct {
    conn   net.Conn
    rxBuf  chan byte      // data port read buffer
    ctlBuf []byte         // control port command accumulator
    status byte           // last status for IN $31
    mu     sync.Mutex
}

// ── Data Port $30 ──────────────────────────────────────────

// IN A, ($30) — non-blocking read from server
func (n *NetPort) DataRead() byte {
    if n.conn == nil { return 0x00 }
    select {
    case b := <-n.rxBuf:
        return 0x80 | b
    default:
        return 0x00
    }
}

// OUT ($30), A — write byte to server
func (n *NetPort) DataWrite(b byte) {
    if n.conn == nil { return }
    n.conn.Write([]byte{b})
}

// ── Control Port $31 ───────────────────────────────────────

// OUT ($31), A — accumulate command byte, execute on \0
func (n *NetPort) CtlWrite(b byte) {
    if b != 0 {
        n.ctlBuf = append(n.ctlBuf, b)
        return
    }
    // \0 = execute command
    if len(n.ctlBuf) == 0 { return }
    cmd := n.ctlBuf[0]
    arg := string(n.ctlBuf[1:])
    n.ctlBuf = n.ctlBuf[:0]

    switch cmd {
    case 'C': // Connect
        go n.doConnect(arg)
    case 'D': // Disconnect
        n.doDisconnect()
    case 'S': // Status
        // status already readable via IN
    }
}

// IN A, ($31) — read status
func (n *NetPort) CtlRead() byte {
    n.mu.Lock()
    defer n.mu.Unlock()
    s := n.status
    if s != 0x00 { n.status = 0xFF } // consume: return $FF after first read
    return s
}

func (n *NetPort) doConnect(addr string) {
    n.mu.Lock()
    n.status = 0x00 // busy
    n.mu.Unlock()

    conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
    n.mu.Lock()
    defer n.mu.Unlock()
    if err != nil {
        if strings.Contains(err.Error(), "no such host") {
            n.status = 0x02 // DNS error
        } else if strings.Contains(err.Error(), "refused") {
            n.status = 0x03 // connection refused
        } else {
            n.status = 0x04 // timeout / other
        }
        return
    }
    n.conn = conn
    n.rxBuf = make(chan byte, 4096)
    n.status = 0x01 // connected

    go func() { // read pump
        b := make([]byte, 1)
        for {
            if _, err := conn.Read(b); err != nil { return }
            n.rxBuf <- b[0]
        }
    }()
}

func (n *NetPort) doDisconnect() {
    n.mu.Lock()
    defer n.mu.Unlock()
    if n.conn != nil {
        n.conn.Close()
        n.conn = nil
    }
    n.status = 0xFF
}
```

### Wire Into Emulator

```go
// In ReadPort:
case 0x30: return netPort.DataRead()
case 0x31: return netPort.CtlRead()

// In WritePort:
case 0x30: netPort.DataWrite(b)
case 0x31: netPort.CtlWrite(b)
```

### CLI

```bash
# Pre-connected (MZV connects before Z80 boot):
mzv irc_client.com --net irc.libera.chat:6667

# Autonomous (Z80 connects via control port):
mzv irc_client.com
# Z80 program sends "Circ.libera.chat:6667\0" to port $31
```

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
