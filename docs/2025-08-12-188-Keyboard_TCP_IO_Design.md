# Keyboard & TCP/IP I/O Design

## Overview

Comprehensive I/O design for keyboard scan-codes and TCP/IP networking via port interception, avoiding conflicts with ZX Spectrum peripheral legacy.

## ZX Spectrum Port Map (Avoiding Conflicts)

### Used Ports (Must Avoid)
```
0xFE     - ULA (keyboard, border, tape, speaker)
0x7FFD   - 128K memory paging
0xFFFD   - AY-3-8912 register select (128K)
0xBFFD   - AY-3-8912 data (128K)
0x1F     - Kempston joystick
0xDF     - Fuller joystick
0xEF     - ZX Interface 1 (Microdrive)
0xFF     - Timex video modes
0x3FF    - ULAplus palette
```

### Safe Port Ranges for Extensions
```
0x8000-0x8FFF - Generally safe for custom hardware
0x9000-0x9FFF - Used by some disk interfaces but mostly free
0xA000-0xAFFF - Often available
0xB000-0xBFFF - Often available  
0xC000-0xCFFF - Sometimes used by IDE interfaces
0xD000-0xDFFF - Generally available
0xE000-0xEFFF - Sometimes used by network cards
0xF000-0xF7FF - Generally available (avoid 0xF7FD for +3 disk)
```

## Keyboard Scan-Code System

### Direct Hardware Scan-Codes

ZX Spectrum keyboard matrix via port 0xFE:

```minz
module zx.keyboard;

// Keyboard half-rows (port 0xFE high byte)
enum KeyRow {
    SHIFT_V   = 0xFE,  // Shift, Z, X, C, V
    A_G       = 0xFD,  // A, S, D, F, G
    Q_T       = 0xFB,  // Q, W, E, R, T
    ONE_FIVE  = 0xF7,  // 1, 2, 3, 4, 5
    ZERO_SIX  = 0xEF,  // 0, 9, 8, 7, 6
    P_Y       = 0xDF,  // P, O, I, U, Y
    ENTER_H   = 0xBF,  // Enter, L, K, J, H
    SPACE_B   = 0x7F   // Space, Sym, M, N, B
}

// Read keyboard row (active low - 0 = pressed)
pub fun scan_row(row: KeyRow) -> u8 {
    return in(0xFE00 | row);
}

// Check specific key
pub fun is_pressed(row: KeyRow, bit: u8) -> bool {
    return (scan_row(row) & (1 << bit)) == 0;
}

// Get full keyboard matrix state
pub fun scan_matrix() -> [8]u8 {
    let matrix: [8]u8;
    let rows = [0xFE, 0xFD, 0xFB, 0xF7, 0xEF, 0xDF, 0xBF, 0x7F];
    
    for i in 0..8 {
        matrix[i] = in(0xFE00 | rows[i]);
    }
    
    return matrix;
}

// Convert to ASCII (basic mapping)
pub fun matrix_to_ascii(matrix: [8]u8) -> u8 {
    // Check each row and bit for pressed keys
    // Return ASCII code of first pressed key
    // This is simplified - real implementation needs
    // shift state handling, key repeat, etc.
}
```

### Enhanced Keyboard Input via MZE

For modern development, MZE can intercept and provide enhanced keyboard:

```minz
module mze.keyboard;

// MZE enhanced keyboard port (0x8000)
const PORT_KEYBOARD_STATUS = 0x8000;
const PORT_KEYBOARD_DATA   = 0x8001;
const PORT_KEYBOARD_MODE   = 0x8002;

enum KeyMode {
    SCANCODE,     // Raw scan codes
    ASCII,        // ASCII conversion
    UNICODE       // UTF-8 sequence
}

// Check if key available
pub fun key_available() -> bool {
    return (in(PORT_KEYBOARD_STATUS) & 0x01) != 0;
}

// Get next key (blocking)
pub fun get_key() -> u8 {
    while !key_available() {
        // Wait for key
    }
    return in(PORT_KEYBOARD_DATA);
}

// Get key with timeout
pub fun get_key_timeout(frames: u16) -> u8? {
    for i in 0..frames {
        if key_available() {
            return in(PORT_KEYBOARD_DATA);
        }
        wait_frame();
    }
    return null;  // Timeout
}

// Set keyboard mode
pub fun set_mode(mode: KeyMode) -> void {
    out(PORT_KEYBOARD_MODE, mode as u8);
}
```

## TCP/IP Networking Design

### Port-Based Network Interface

Using safe port range 0x9000-0x90FF for network operations:

```minz
module net.tcp;

// Network interface ports
const PORT_NET_CMD    = 0x9000;  // Command register
const PORT_NET_STATUS = 0x9001;  // Status register
const PORT_NET_DATA   = 0x9002;  // Data transfer
const PORT_NET_ADDR_L = 0x9003;  // Address low byte
const PORT_NET_ADDR_H = 0x9004;  // Address high byte
const PORT_NET_PORT_L = 0x9005;  // Port low byte
const PORT_NET_PORT_H = 0x9006;  // Port high byte
const PORT_NET_LEN_L  = 0x9007;  // Length low byte
const PORT_NET_LEN_H  = 0x9008;  // Length high byte

// Commands
enum NetCmd {
    CONNECT    = 0x01,  // Connect to host:port
    LISTEN     = 0x02,  // Listen on port
    SEND       = 0x03,  // Send data
    RECEIVE    = 0x04,  // Receive data
    CLOSE      = 0x05,  // Close connection
    STATUS     = 0x06,  // Get status
    SET_IP     = 0x10,  // Set IP address
    SET_DNS    = 0x11   // Set DNS server
}

// Status bits
enum NetStatus {
    CONNECTED  = 0x01,
    DATA_READY = 0x02,
    SEND_READY = 0x04,
    ERROR      = 0x80
}

struct Connection {
    handle: u8,
    local_port: u16,
    remote_port: u16,
    remote_ip: [4]u8
}

// Connect to server
pub fun connect(ip: [4]u8, port: u16) -> Connection? {
    // Set target IP
    for i in 0..4 {
        out(PORT_NET_ADDR_L + i, ip[i]);
    }
    
    // Set port
    out(PORT_NET_PORT_L, port & 0xFF);
    out(PORT_NET_PORT_H, port >> 8);
    
    // Issue connect command
    out(PORT_NET_CMD, NetCmd.CONNECT);
    
    // Wait for connection
    let timeout = 1000;
    while timeout > 0 {
        let status = in(PORT_NET_STATUS);
        if status & NetStatus.CONNECTED {
            return Connection {
                handle: in(PORT_NET_DATA),
                local_port: 0,  // Assigned by stack
                remote_port: port,
                remote_ip: ip
            };
        }
        if status & NetStatus.ERROR {
            return null;
        }
        wait_ms(10);
        timeout -= 10;
    }
    
    return null;  // Timeout
}

// Send data
pub fun send(conn: Connection, data: []u8) -> bool {
    // Set connection handle
    out(PORT_NET_DATA, conn.handle);
    
    // Set data length
    out(PORT_NET_LEN_L, data.len & 0xFF);
    out(PORT_NET_LEN_H, data.len >> 8);
    
    // Send data byte by byte
    out(PORT_NET_CMD, NetCmd.SEND);
    
    for byte in data {
        // Wait for ready
        while !(in(PORT_NET_STATUS) & NetStatus.SEND_READY) {}
        out(PORT_NET_DATA, byte);
    }
    
    return true;
}

// Receive data
pub fun receive(conn: Connection, buffer: []u8) -> u16 {
    // Set connection handle
    out(PORT_NET_DATA, conn.handle);
    
    // Check for data
    out(PORT_NET_CMD, NetCmd.RECEIVE);
    
    if !(in(PORT_NET_STATUS) & NetStatus.DATA_READY) {
        return 0;
    }
    
    // Get length
    let len_l = in(PORT_NET_LEN_L);
    let len_h = in(PORT_NET_LEN_H);
    let len = (len_h << 8) | len_l;
    
    // Read data
    let to_read = min(len, buffer.len);
    for i in 0..to_read {
        buffer[i] = in(PORT_NET_DATA);
    }
    
    return to_read;
}
```

### High-Level HTTP Client

```minz
module net.http;

import net.tcp;

struct HttpRequest {
    method: str,
    path: str,
    host: str,
    headers: []str,
    body: []u8
}

struct HttpResponse {
    status_code: u16,
    headers: []str,
    body: []u8
}

pub fun get(url: str) -> HttpResponse? {
    // Parse URL
    let host = parse_host(url);
    let path = parse_path(url);
    let port = parse_port(url) ?? 80;
    
    // Resolve hostname (via MZE DNS)
    let ip = resolve_host(host)?;
    
    // Connect
    let conn = tcp.connect(ip, port)?;
    
    // Build request
    let request = format("GET {} HTTP/1.0\r\n", path);
    request += format("Host: {}\r\n", host);
    request += "Connection: close\r\n";
    request += "\r\n";
    
    // Send request
    tcp.send(conn, request.as_bytes());
    
    // Receive response
    let buffer: [4096]u8;
    let received = tcp.receive(conn, buffer);
    
    // Parse response
    return parse_response(buffer[0..received]);
}
```

## MZE Emulator Integration

### Network Interceptor in Go

```go
// pkg/emulator/network_interceptor.go
package emulator

import (
    "net"
    "sync"
)

type NetworkInterceptor struct {
    connections map[byte]*net.Conn
    nextHandle  byte
    mu          sync.Mutex
}

func (ni *NetworkInterceptor) HandleIN(port uint16) byte {
    switch port {
    case 0x9001: // Status
        return ni.getStatus()
    case 0x9002: // Data
        return ni.readData()
    // ... other ports
    }
    return 0xFF
}

func (ni *NetworkInterceptor) HandleOUT(port uint16, value byte) {
    switch port {
    case 0x9000: // Command
        ni.executeCommand(value)
    case 0x9002: // Data
        ni.writeData(value)
    // ... other ports
    }
}

func (ni *NetworkInterceptor) executeCommand(cmd byte) {
    switch cmd {
    case 0x01: // CONNECT
        go ni.connect()
    case 0x03: // SEND
        go ni.send()
    case 0x04: // RECEIVE
        go ni.receive()
    }
}

// Bridge to host networking
func (ni *NetworkInterceptor) connect() {
    addr := ni.getTargetAddress()
    conn, err := net.Dial("tcp", addr)
    if err != nil {
        ni.setError()
        return
    }
    
    ni.mu.Lock()
    handle := ni.nextHandle
    ni.nextHandle++
    ni.connections[handle] = &conn
    ni.mu.Unlock()
    
    ni.setConnected(handle)
}
```

### Keyboard Interceptor

```go
// pkg/emulator/keyboard_interceptor.go
package emulator

import (
    "github.com/nsf/termbox-go"
)

type KeyboardInterceptor struct {
    buffer    []byte
    mode      KeyMode
    matrixMap map[termbox.Key][2]byte // [row, bit]
}

func (ki *KeyboardInterceptor) HandleIN(port uint16) byte {
    switch port {
    case 0x8000: // Status
        if len(ki.buffer) > 0 {
            return 0x01
        }
        return 0x00
        
    case 0x8001: // Data
        if len(ki.buffer) > 0 {
            key := ki.buffer[0]
            ki.buffer = ki.buffer[1:]
            return key
        }
        return 0
        
    default:
        // Check for ZX keyboard matrix read
        if (port & 0xFF) == 0xFE {
            row := byte(port >> 8)
            return ki.getMatrixRow(row)
        }
    }
    return 0xFF
}

// Map modern keyboard to ZX matrix
func (ki *KeyboardInterceptor) updateMatrix(event termbox.Event) {
    if pos, ok := ki.matrixMap[event.Key]; ok {
        row, bit := pos[0], pos[1]
        // Update matrix state
    }
    
    // Also buffer for enhanced mode
    if ki.mode == ASCII {
        ki.buffer = append(ki.buffer, byte(event.Ch))
    }
}
```

## Benefits of This Design

1. **Non-Conflicting**: Uses safe port ranges (0x8000+, 0x9000+)
2. **Backward Compatible**: Original ZX keyboard still works
3. **Modern Features**: TCP/IP, enhanced keyboard, DNS
4. **Platform Native**: Can work on real hardware with network card
5. **Emulator Bridge**: MZE provides host networking
6. **Type Safe**: MinZ ensures correct protocol usage

## Usage Examples

### Chat Client

```minz
import net.tcp;
import mze.keyboard;

fun chat_client(server_ip: [4]u8) -> void {
    // Connect to chat server
    let conn = tcp.connect(server_ip, 6667)?;
    
    print("Connected to chat server!\n");
    print("Type messages, press Enter to send\n");
    
    let input: [256]u8;
    let input_len = 0;
    
    while true {
        // Check for incoming messages
        let buffer: [512]u8;
        let received = tcp.receive(conn, buffer);
        if received > 0 {
            print(str.from_bytes(buffer[0..received]));
        }
        
        // Check for keyboard input
        if keyboard.key_available() {
            let key = keyboard.get_key();
            
            if key == '\n' {
                // Send message
                tcp.send(conn, input[0..input_len]);
                input_len = 0;
            } else {
                input[input_len] = key;
                input_len++;
                print_char(key);  // Local echo
            }
        }
    }
}
```

### Game High Score Server

```minz
import net.http;

struct HighScore {
    name: [3]u8,
    score: u32
}

fun submit_score(score: HighScore) -> bool {
    let json = format("{\"name\":\"{}\",\"score\":{}}", 
                      score.name, score.score);
    
    let response = http.post(
        "http://scores.minz-games.com/submit",
        "application/json",
        json.as_bytes()
    )?;
    
    return response.status_code == 200;
}

fun get_top_scores() -> []HighScore {
    let response = http.get("http://scores.minz-games.com/top10")?;
    
    if response.status_code == 200 {
        return parse_json_scores(response.body);
    }
    
    return [];
}
```

## Implementation Phases

1. **Phase 1**: Basic keyboard scan-code reading
2. **Phase 2**: MZE enhanced keyboard buffer
3. **Phase 3**: TCP connect/send/receive
4. **Phase 4**: HTTP client library
5. **Phase 5**: WebSocket support
6. **Phase 6**: UDP for games

## Testing Strategy

```bash
# Test keyboard input
mze keyboard_test.a80 --enable-keyboard --keyboard-mode=enhanced

# Test network
mze tcp_test.a80 --enable-network --network-bridge

# Test with real server
mze chat_client.a80 --network-server=irc.libera.chat:6667
```

---

This design provides modern I/O capabilities while respecting ZX Spectrum's legacy hardware! 🚀