# IRC on Z80: Why Nanz, and Why Not Frill or Lizp

*April 2026*

---

MinZ has eight frontends. Three of them — Nanz, Frill, Lizp — are "native" languages designed for the compiler. When we set out to write an IRC client and server for Z80, the question wasn't whether we *could* write it in each — they all compile to the same MIR2 IR and the same Z80 backend. The question was which one makes the code *clearest*, the abstractions *cheapest*, and the Z80 output *best*.

We chose Nanz. Here's why, and what Frill and Lizp would have looked like instead.

---

## The Task

An IRC client + server running on Z80 at 3.5 MHz, 64KB RAM. The host (MZV emulator) handles TCP/TLS. The Z80 program sees I/O ports:

- **Port $30** — data: read/write bytes to/from the network
- **Port $31** — control: connect/disconnect/status
- **Port $23** — console (VT100 terminal via TUI stdlib)

The Z80 code must:
- Parse IRC protocol (text lines, `:prefix COMMAND params :trailing`)
- Maintain state (nicks, channels, connection status)
- Render a TUI (status bar, message area, input line)
- Handle keyboard input with `/commands`
- For the server: multiplex 8 client connections, broadcast messages

---

## Why Nanz Won

### 1. Mutable State is Natural

IRC is stateful. Nicks change. Channels have member lists. Line buffers accumulate bytes. A server tracks 8 client slots. In Nanz, this is direct:

```nanz
enum ClientState { Empty, Connected, Registered }

global client_state: [u8; 8]
global client_nick:  [u8; 128]     // 8 × 16 chars
global srv_buf: [u8; 512]
global srv_pos: u16 = 0
```

Arrays, globals, mutable variables — they map 1:1 to Z80 memory. No allocation, no indirection, no GC. `client_state[slot]` compiles to an indexed load. `srv_pos = 0` compiles to `LD (srv_pos), 0`.

In **Frill** (ML-style, pure functional), mutation requires explicit `ref` or would need a state monad:

```frill
(* Frill doesn't have mutable globals naturally *)
(* Each state change = new value threaded through every function *)
let poll_network (state : ServerState) : ServerState =
    let b = net_read () in
    if b == 0 then state
    else { state with buf = append state.buf (b & 0x7F) }
```

Beautiful in theory. But on Z80, threading a struct through every function means copying bytes on stack. The compiler *might* optimize it away, but "might" isn't good enough when you have 64KB total.

In **Lizp** (S-expression Lisp), state is either mutation (which Lizp supports but feels wrong) or association lists:

```lizp
(define client-state (make-vector 8 'empty))
(vector-set! client-state slot 'registered)
```

Workable, but Lizp's vector operations are heavier than Nanz's direct array indexing. And the quotes around symbols (`'empty`, `'registered`) become runtime tag comparisons instead of compile-time enum values.

### 2. Enums + Match = IRC Command Dispatch

IRC has a small set of commands (PRIVMSG, JOIN, PART, QUIT, PING, numeric replies). This is a perfect match for enums:

```nanz
enum IrcCmd { Ping, Privmsg, Join, Part, Quit, Numeric, Unknown }

fun classify_irc(buf: ^u8, pos: u16) -> IrcCmd {
    if buf_starts_with(buf, "PING")          { return IrcCmd.Ping }
    if region_match(buf, pos, "PRIVMSG", 7)  { return IrcCmd.Privmsg }
    if region_match(buf, pos, "JOIN", 4)      { return IrcCmd.Join }
    if region_match(buf, pos, "PART", 4)      { return IrcCmd.Part }
    if region_match(buf, pos, "QUIT", 4)      { return IrcCmd.Quit }
    return IrcCmd.Unknown
}
```

The enum compiles to integer constants (0-6). The classify function compiles to a chain of string comparisons. On Z80, each `region_match` is a tight `CP (HL)` loop — exactly what you'd write by hand.

**Frill** has algebraic data types, which are *more* powerful:

```frill
type IrcCmd =
    | Ping
    | Privmsg of string * string   (* channel, message *)
    | Join of string               (* channel *)
    | Quit of string               (* reason *)
```

This is elegant — the payload (channel name, message text) lives inside the variant. But on Z80, ADT constructors allocate tagged tuples. For a protocol parser that processes hundreds of lines per second, we don't want allocation on every parsed line. Nanz enums are zero-cost integers.

**Lizp** would use symbols or tagged lists:

```lizp
(case (classify-command line)
  ('ping (handle-ping line))
  ('privmsg (handle-privmsg line))
  ('join (handle-join line)))
```

Clean, but `case` on symbols is a runtime string comparison in Lizp. Nanz's `match` on enums is a compile-time integer compare — `CP 3 / JR Z` on Z80.

### 3. Pointer Arithmetic = Protocol Parsing

IRC parsing is pointer work. Skip the prefix (`:nick!user@host`), find the command, extract the trailing message after `:`. In Nanz, this is natural:

```nanz
// Skip :prefix — find first space
if srv_buf[0] == 58 {   // ':'
    nick_start = 1
    pos = 1
    while srv_buf[pos] != 33 && srv_buf[pos] != 32 && srv_buf[pos] != 0 {
        pos = pos + 1
    }
    nick_len = pos - 1
}

// Extract trailing after ':'
while srv_buf[pos] != 58 && srv_buf[pos] != 0 { pos = pos + 1 }
if srv_buf[pos] == 58 { pos = pos + 1 }

// Print from position
print_from(&srv_buf, pos)
```

`srv_buf[pos]` is a pointer-indexed load: `LD HL, srv_buf / ADD HL, pos / LD A, (HL)`. `pos = pos + 1` is `INC`. This is the same code a C programmer would write, and it compiles to the same Z80.

**Frill** would need to express this functionally:

```frill
let skip_prefix (line : ^u8) (pos : u8) : u8 =
    if peek (offset line pos) == ' ' then pos
    else skip_prefix line (pos + 1)
```

Recursive descent is beautiful. But each recursive call is a function call on Z80 (17T CALL + 10T RET). The while loop is a 12T `JR NZ`. For a parser that runs on every incoming IRC line, 27T vs 12T per character adds up.

**Lizp** would use `string-ref` and recursion:

```lizp
(define (skip-prefix line pos)
  (if (= (string-ref line pos) #\space) pos
      (skip-prefix line (+ pos 1))))
```

Same overhead as Frill — recursive calls where a loop would suffice.

### 4. @extern + I/O Ports = Clean Hardware Interface

The Z80 talks to the network through I/O ports. Nanz's `@extern` declares these as foreign functions:

```nanz
@extern fun net_read() -> u8       // IN A, ($30)
@extern fun net_write(b: u8)       // OUT ($30), A
@extern fun ctl_write(b: u8)       // OUT ($31), A
```

MZV intercepts these as host functions. The Z80 code calls them like regular functions — `CALL net_read` — and MZV traps the call, does the I/O, returns the result. Zero ceremony.

**Frill** can do this too (`extern`), but the calling convention is less natural for side-effecting I/O in a pure functional language. You'd want an IO monad or effect system, which doesn't exist in Frill yet.

**Lizp** has FFI via `(foreign-call "net_read")`, but it's more verbose and the runtime overhead of the Lizp calling convention adds latency to every I/O operation.

### 5. TUI Stdlib is Written in Nanz

The TUI library (`stdlib/tui/render.nanz`) is Nanz. Importing it from Nanz is one line:

```nanz
import tui.render
```

From Frill or Lizp, we'd need cross-language imports (which work — all frontends compile to MIR2) but the type signatures don't match naturally. Frill expects curried functions; the TUI functions are multi-argument. Lizp expects list-based calling; TUI uses positional params.

Staying in Nanz means `tui_goto(x, y)` works as-is. No adapter layer, no calling convention mismatch.

### 6. for-in Loops = Polling Pattern

The IRC main loop polls 8 clients, reads up to 32 bytes per tick, iterates over user input. Nanz's `for-in` is perfect:

```nanz
for slot: u8 in 0..MAX_CLIENTS {
    if client_state[slot] != ClientState.Empty {
        poll_client(slot)
    }
}

for i: u8 in 0..32 {
    let b: u8 = net_read()
    if b == 0 { return }
    // ... accumulate
}
```

The `for i in 0..n` compiles to a DJNZ loop on Z80 — the tightest possible iteration. In Frill, you'd use `List.iter` or recursion. In Lizp, `do` loop or `for-each`. Both work, but neither produces DJNZ as naturally.

---

## Wait — Nanz Has Pipes Too!

A common misconception: "Frill has `|>`, Nanz doesn't." Wrong. Nanz has **five** forms of function composition — more than Frill:

```nanz
// 1. Value pipe (same as Frill |>)
5 |> double |> inc                           // → 11

// 2. Pipe with partial application (Frill CAN'T do this)
5 |> add(3) |> mul(2)                        // → 16

// 3. Named pipe declaration (unique to Nanz)
pipe doubled { map(|x: u8| x + x) }
range(0..5).apply(doubled).fold(0, add_acc)  // → 30

// 4. Trans — pipe composition (unique to Nanz)
trans doubled_inc { use doubled; map(|x| x + 1) }

// 5. UFCS chains
buf.map(|x| x*2).filter(|x| x > threshold).forEach(|x| process(x), n)
```

Z80 output — **every pipe chain with constant args const-folds to one instruction:**

```z80
test_basic_pipe:       LD A, 11 / RET    ; 5 |> double |> inc
test_pipe_with_args:   LD A, 16 / RET    ; 5 |> add(3) |> mul(2)
```

Named pipes with `range().apply().fold()` produce DJNZ loops. Nanz doesn't trade away functional composition for imperative performance. It has both.

---

## What Frill Would Have Been Good For

**ADT destructuring.** If IRC parsing were a bigger part of the codebase (like a protocol analyzer, not a client), Frill's algebraic types would shine:

```frill
type IrcMsg =
    | Ping of string
    | Privmsg of { sender: string, target: string, body: string }
    | Join of { nick: string, channel: string }
    | Raw of string

let parse_irc (line : ^u8) : IrcMsg =
    line |> split_prefix |> match_command |> extract_trailing

let display (msg : IrcMsg) : unit = match msg with
    | Privmsg { sender; body; _ } -> printf "<%s> %s" sender body
    | Join { nick; channel } -> printf "%s joined %s" nick channel
    | Ping s -> send_pong s
    | Raw s -> print_dim s
```

The pipe chain `split_prefix |> match_command |> extract_trailing` is more readable than nested while loops. The destructuring `Privmsg { sender; body; _ }` is cleaner than manual offset tracking.

But: each `Privmsg { ... }` allocates on Z80. For a one-shot parser that processes one line then throws it away, Nanz's zero-allocation pointer approach is faster.

**Hybrid approach (future):** Write the parser in Frill, compile to MIR2, call from Nanz main loop. Best of both worlds. Cross-language function calls already work.

---

## What Lizp Would Have Been Good For

**Configuration and scripting.** If IRC commands were data-driven (user-definable aliases, auto-join lists, scripted responses), Lizp's homoiconic syntax would be natural:

```lizp
(define-alias "/hi" (lambda ()
    (send-privmsg current-channel "Hello from Z80!")))

(define auto-join '("#minz" "#z80" "#retro"))

(for-each join-channel auto-join)
```

Code-as-data means the IRC client could load `.lizprc` config files and execute them. Nanz doesn't have `eval` — it's a compiled language.

But: for a 64KB Z80, eval + garbage collection + symbol interning is heavy. A fixed set of `/commands` in Nanz is smaller and faster.

---

## The Numbers

| Aspect | Nanz | Frill | Lizp |
|--------|------|-------|------|
| Mutable state | Native (globals, arrays) | Awkward (ref/threading) | Native but heavy (vectors) |
| Enum dispatch | Integer compare (4T) | ADT tag check (4T + alloc) | Symbol compare (runtime) |
| Pointer parsing | Direct (HL + offset) | Functional (recursive calls) | string-ref (indirect) |
| I/O ports | @extern (zero overhead) | extern (needs IO wrapper) | foreign-call (verbose) |
| TUI import | Native (same language) | Cross-language adapter | Cross-language adapter |
| for-in → DJNZ | Direct | List.iter → call chain | do/for-each → call chain |
| **IRC client LOC** | **~350** | ~450 (est.) | ~500 (est.) |
| **IRC server LOC** | **~350** | ~500 (est.) | ~550 (est.) |

---

## What We Built

### Client (`examples/nanz/irc_client.nanz`)
- 4 enums: `ConnStatus`, `IrcCmd`, `UserCmd`, `Color`
- Match dispatch for IRC commands and keyboard input
- Control port $31 for autonomous connect (TCP + TLS via host)
- TUI with status bar, colored nicks, input line
- `/join`, `/part`, `/msg`, `/nick`, `/me`, `/topic`, `/quit`, `/connect`, `/tls`, `/raw`
- 40 functions → 2212 lines Z80 asm

### Server (design, next to implement)
- 8 client slots via multiplexed port $40-$42
- NICK/USER registration, JOIN with names/topic, PRIVMSG broadcast
- PING/PONG keepalive, QUIT notification
- ~350 LOC Nanz, ~60 LOC Go (MZV listener)

### MZV Host Additions
- Port $30: data (read/write bytes, non-blocking)
- Port $31: control (connect/disconnect/TLS, status)
- Port $40-$42: server multiplexer (select slot, read/write, events)
- `--net host:port` CLI flag for pre-connected mode
- `--listen :port` CLI flag for server mode
- TLS transparent via Go `crypto/tls`

---

## The Punchline

We chose Nanz because IRC is a systems programming task: mutable state, byte-level parsing, I/O polling, fixed-size buffers. Nanz handles all of this with zero overhead — every abstraction (enums, match, for-in, @extern) compiles to the exact Z80 code you'd write by hand.

Frill is for when the problem is *transformational* — pipelines, pattern matching, composition. The IRC parser could be beautiful in Frill.

Lizp is for when the problem is *dynamic* — config, scripting, eval. An IRC bot with user-defined commands would be natural in Lizp.

The right answer for a real project: **Nanz for the core, Frill for the parser, Lizp for the scripting layer.** Three languages, one IR, one binary. That's what eight frontends are for.

---

*IRC on Z80. Nanz for the engine, Frill for the beauty, Lizp for the soul.*
