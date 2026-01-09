# ZXSpeculator DZRP Integration Recommendations

**To:** DeanTheCoder (ZXSpeculator Developer)
**From:** MinZ Toolchain Team
**Date:** January 2026
**Subject:** Environment Variables and Configuration Consistency for DZRP Tools

## Overview

We've been developing DZRP-based tools for the MinZ compiler toolchain that integrate beautifully with ZXSpeculator. To improve the developer experience across the ecosystem, we'd like to propose some standardized environment variables and configuration options.

## Current MinZ Tools Using DZRP

| Tool | Purpose |
|------|---------|
| `mzrun` | Compile and run MinZ programs remotely |
| `mztap` | Load TAP files directly (bypassing tape emulation) |
| `mze --dzrp` | Connect built-in emulator to external debugger |

## Proposed Standard Environment Variables

We've implemented these in our tools and recommend ZXSpeculator support them as well:

### DZRP_HOST
**Default:** `localhost`
**Purpose:** Default host/IP for DZRP connections

```bash
export DZRP_HOST=192.168.1.100  # Remote machine running ZXSpeculator
```

### DZRP_PORT
**Default:** `11000`
**Purpose:** Default DZRP TCP port

```bash
export DZRP_PORT=11001  # If running multiple instances
```

### DZRP_SOCKET
**Default:** `tcp`
**Values:** `tcp`, `ws` (WebSocket)
**Purpose:** Connection transport type

```bash
export DZRP_SOCKET=ws  # For browser-based or tunneled connections
```

## Recommended ZXSpeculator Support

### 1. Command-Line Arguments
```bash
# Current (works great!)
ZXSpeculator --dzrp

# Proposed additions
ZXSpeculator --dzrp-port 11001
ZXSpeculator --dzrp-host 0.0.0.0      # Listen on all interfaces
ZXSpeculator --dzrp-ws                 # Enable WebSocket endpoint
ZXSpeculator --dzrp-ws-port 11080      # WebSocket on different port
```

### 2. Environment Variable Reading
ZXSpeculator could read these on startup:
```csharp
var port = Environment.GetEnvironmentVariable("DZRP_PORT") ?? "11000";
var host = Environment.GetEnvironmentVariable("DZRP_HOST") ?? "localhost";
```

### 3. Configuration File (~/.zxspeculator.json or similar)
```json
{
  "dzrp": {
    "enabled": true,
    "host": "0.0.0.0",
    "port": 11000,
    "webSocket": {
      "enabled": false,
      "port": 11080
    }
  }
}
```

## WebSocket Support Rationale

WebSocket transport would enable:

1. **Browser-based tools** - Web IDEs, online debuggers
2. **Tunneling through firewalls** - HTTP/WS proxies are common
3. **Cloud development** - Gitpod, Codespaces, etc.
4. **Cross-platform consistency** - Works everywhere HTTP works

### Proposed WebSocket Endpoint
```
ws://localhost:11080/dzrp
```

The protocol would be identical - just wrapped in WebSocket frames instead of raw TCP.

## Integration Example

With these standards, a developer's workflow becomes:

```bash
# ~/.bashrc or ~/.zshrc
export DZRP_HOST=192.168.1.100
export DZRP_PORT=11000

# On development machine - all tools "just work"
mzrun game.minz           # Compiles and runs on remote ZXSpeculator
mztap demo.tap            # Loads TAP instantly
code --with-dezog         # VS Code debugging works too
```

## Current Integration Status

### What Works Perfectly
- TCP DZRP on port 11000
- CMD_INIT, CMD_PAUSE, CMD_CONTINUE
- CMD_READ_MEM, CMD_WRITE_MEM
- CMD_GET_REGISTERS, CMD_SET_REGISTER
- CMD_STEP_INTO (single stepping)
- Notifications (breakpoints, pause events)

### Minor Observations
- Response to CMD_INIT could include emulator version/capabilities
- CMD_SET_REGISTER index mapping matches DeZog spec perfectly

## mztap: Instant TAP Loading

Our `mztap` tool demonstrates the power of DZRP for development:

```bash
# Instead of waiting for tape loading simulation...
mztap game.tap  # Loads 19KB in ~100ms!
```

It works by:
1. Parsing TAP file structure
2. Extracting CODE blocks with load addresses
3. Using CMD_WRITE_MEM to upload directly
4. Setting PC and continuing execution

This could potentially be built into ZXSpeculator as a "turbo load" feature.

## Thank You!

ZXSpeculator's DZRP implementation is excellent. These recommendations are meant to enhance the ecosystem consistency, not criticize the current implementation.

The MinZ toolchain + ZXSpeculator combination provides an amazing retro development experience - modern tooling with accurate emulation.

---

**Contact:** https://github.com/oisee/minz
**MinZ Toolchain:** mzrun, mztap, mza, mze, mzr
