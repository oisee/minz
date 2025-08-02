# Z80 Goroutine-Like Concurrency Design

## Overview

This document describes the design for goroutine-like cooperative concurrency in MinZ for Z80-based systems. Unlike preemptive multithreading, this system provides cooperative multitasking specifically optimized for Z80 constraints.

## Design Philosophy

### Core Principles
1. **Cooperative Scheduling**: Tasks voluntarily yield control (no preemption)
2. **Stack-Based Coroutines**: Each goroutine has its own stack space
3. **Zero-Copy Communication**: Message passing through shared memory regions
4. **Interrupt Integration**: Leverages Z80 interrupt system for I/O coordination
5. **Memory Efficient**: Designed for 64KB address space constraints

### Syntax Design

```minz
// Channel declaration
let ch = make_channel<u8>(10);  // Buffered channel with 10 slots

// Goroutine launch
go {
    // This code runs concurrently
    for i in 0..100 {
        ch.send(i);
        yield();  // Cooperative yield
    }
    ch.close();
}

// Main routine
fun main() -> u8 {
    let ch = make_channel<u8>(10);
    
    // Launch producer
    go producer(ch);
    
    // Consumer loop
    let sum = 0;
    loop {
        match ch.receive() {
            Some(value) => { sum += value; }
            None => break;  // Channel closed
        }
        yield();  // Be cooperative
    }
    
    sum
}

fun producer(ch: Channel<u8>) {
    for i in 0..10 {
        ch.send(i);
        yield();
    }
    ch.close();
}
```

## Architecture Components

### 1. Goroutine Control Block (GCB)

```asm
; Goroutine Control Block (8 bytes)
GCB_STRUCT:
    .WORD   STACK_PTR      ; Current stack pointer (2 bytes)
    .WORD   STACK_BASE     ; Stack base address (2 bytes)
    .BYTE   STATUS         ; Status: READY, RUNNING, WAITING, DEAD (1 byte)
    .BYTE   PRIORITY       ; Execution priority (1 byte)
    .WORD   NEXT_GCB       ; Linked list pointer (2 bytes)
```

### 2. Scheduler Implementation

```asm
; Round-robin cooperative scheduler
SCHEDULER:
    LD HL, CURRENT_GCB     ; Load current goroutine
    LD DE, GCB_NEXT        ; Get next goroutine
    ADD HL, DE
    LD E, (HL)
    INC HL
    LD D, (HL)             ; DE = next GCB
    
    ; Check if next goroutine is ready
    LD HL, DE
    LD BC, GCB_STATUS
    ADD HL, BC
    LD A, (HL)             ; Load status
    CP STATUS_READY
    JR Z, SWITCH_TO        ; If ready, switch to it
    
    ; Find next ready goroutine (simplified - linear search)
    ; In practice, use ready queue
    
SWITCH_TO:
    ; Save current context
    LD (CURRENT_SP), SP    ; Save stack pointer
    
    ; Load new context
    LD HL, DE              ; HL = new GCB
    LD BC, GCB_STACK_PTR
    ADD HL, BC
    LD E, (HL)
    INC HL
    LD D, (HL)             ; DE = new stack pointer
    EX DE, HL
    LD SP, HL              ; Switch stack
    
    ; Update current goroutine
    LD (CURRENT_GCB), DE
    
    RET                    ; Return to new goroutine
```

### 3. Channel Implementation

```asm
; Channel structure (16 bytes for u8 channel)
CHANNEL_STRUCT:
    .WORD   BUFFER_PTR     ; Pointer to circular buffer (2 bytes)
    .BYTE   BUFFER_SIZE    ; Size of buffer (1 byte)
    .BYTE   HEAD           ; Head index (1 byte)
    .BYTE   TAIL           ; Tail index (1 byte)
    .BYTE   COUNT          ; Current item count (1 byte)
    .BYTE   ELEM_SIZE      ; Size of each element (1 byte)
    .BYTE   STATUS         ; OPEN/CLOSED (1 byte)
    .WORD   WAITING_SEND   ; Queue of goroutines waiting to send (2 bytes)
    .WORD   WAITING_RECV   ; Queue of goroutines waiting to receive (2 bytes)
    .WORD   RESERVED       ; Reserved for future use (2 bytes)

; Channel send operation
CHANNEL_SEND:
    ; Check if channel is full
    LD A, (HL + CH_COUNT)
    LD B, (HL + CH_SIZE)
    CP B
    JR Z, SEND_BLOCK       ; Channel full, block goroutine
    
    ; Add item to buffer
    LD A, (HL + CH_TAIL)
    ; ... circular buffer logic ...
    
    ; Wake up waiting receivers
    ; ... scheduling logic ...
    
    RET

SEND_BLOCK:
    ; Add current goroutine to waiting queue
    ; Mark goroutine as WAITING
    ; Call scheduler to switch to another goroutine
    CALL SCHEDULER
    ; When resumed, try again
    JR CHANNEL_SEND
```

## Memory Layout Strategy

### Stack Allocation

```
Memory Layout for 4 Goroutines in 64KB Z80 System:

$0000-$3FFF: ROM/System
$4000-$7FFF: Main Program Code
$8000-$8FFF: Goroutine 0 Stack (4KB)
$9000-$9FFF: Goroutine 1 Stack (4KB)  
$A000-$AFFF: Goroutine 2 Stack (4KB)
$B000-$BFFF: Goroutine 3 Stack (4KB)
$C000-$CFFF: Channel Buffers (4KB)
$D000-$DFFF: Scheduler Data (4KB)
$E000-$FFFF: System/Interrupt vectors
```

### Dynamic Stack Allocation

```minz
// Compile-time stack size calculation
const GOROUTINE_STACK_SIZE = 1024;  // 1KB per goroutine
const MAX_GOROUTINES = 8;           // System limit

// Runtime stack allocator
fun allocate_goroutine_stack() -> *mut u8 {
    static mut NEXT_STACK_ADDR: u16 = 0x8000;
    
    if NEXT_STACK_ADDR + GOROUTINE_STACK_SIZE > 0xC000 {
        panic("Out of stack space for goroutines");
    }
    
    let stack_base = NEXT_STACK_ADDR;
    NEXT_STACK_ADDR += GOROUTINE_STACK_SIZE;
    stack_base as *mut u8
}
```

## Integration with MinZ Language

### 1. Compiler Support

The MinZ compiler will need to:

1. **Transform `go` blocks**: Convert to goroutine creation calls
2. **Channel type checking**: Ensure type safety for channel operations  
3. **Yield point insertion**: Automatically insert cooperative yield points
4. **Stack analysis**: Calculate maximum stack usage per goroutine

### 2. Runtime Library

```minz
// Core goroutine runtime (stdlib/goroutines.minz)
module goroutines;

pub struct Channel<T> {
    handle: *mut ChannelImpl,
}

pub fun make_channel<T>(size: u8) -> Channel<T> {
    // Allocate channel structure
    // Initialize circular buffer
    // Return wrapped handle
}

pub fun go(f: fun()) {
    // Allocate new goroutine stack
    // Create GCB
    // Add to scheduler queue
    // Set up initial stack frame for function f
}

pub fun yield() {
    // Save current state
    // Call scheduler
    // Resume here when scheduled again
}

impl<T> Channel<T> {
    pub fun send(self, value: T) -> Result<(), ChannelError> {
        // Channel send implementation
    }
    
    pub fun receive(self) -> Option<T> {
        // Channel receive implementation
    }
    
    pub fun close(self) {
        // Mark channel as closed
        // Wake all waiting goroutines
    }
}
```

### 3. Interrupt Integration

```asm
; Z80 Interrupt handler for I/O events
INTERRUPT_HANDLER:
    ; Save all registers
    PUSH AF
    PUSH BC
    PUSH DE
    PUSH HL
    EXX
    PUSH BC
    PUSH DE
    PUSH HL
    EX AF, AF'
    PUSH AF
    
    ; Handle I/O event
    CALL HANDLE_IO_EVENT
    
    ; Check if any goroutines became ready
    CALL CHECK_WAITING_GOROUTINES
    
    ; Restore registers
    POP AF
    EX AF, AF'
    POP HL
    POP DE
    POP BC
    EXX
    POP HL
    POP DE
    POP BC
    POP AF
    
    EI                     ; Re-enable interrupts
    RET
```

## Performance Characteristics

### Context Switch Cost
- **Register save/restore**: ~40 T-states
- **Stack pointer switch**: ~20 T-states  
- **Scheduler overhead**: ~100 T-states
- **Total per switch**: ~160 T-states (≈80μs @ 2MHz)

### Memory Overhead
- **Per goroutine**: 8 bytes (GCB) + 1KB (stack) = 1032 bytes
- **Per channel**: 16 bytes + buffer size
- **Total for 4 goroutines + 2 channels**: ~4.2KB

### Throughput
- **Context switches/second**: ~12,500 @ 2MHz Z80
- **Channel operations/second**: ~50,000 send/receive pairs

## Example Applications

### 1. Producer-Consumer Pattern

```minz
fun main() -> u8 {
    let data_ch = make_channel<u8>(5);
    let result_ch = make_channel<u8>(1);
    
    // Producer goroutine
    go {
        for i in 1..=10 {
            data_ch.send(i);
            yield();
        }
        data_ch.close();
    }
    
    // Consumer goroutine  
    go {
        let sum = 0;
        loop {
            match data_ch.receive() {
                Some(value) => { sum += value; }
                None => break;
            }
            yield();
        }
        result_ch.send(sum);
    }
    
    // Main waits for result
    result_ch.receive().unwrap_or(0)
}
```

### 2. I/O Multiplexing

```minz
fun keyboard_handler() {
    let key_ch = make_channel<u8>(10);
    
    // Keyboard goroutine
    go {
        loop {
            let key = wait_for_keypress();  // Yields until key available
            key_ch.send(key);
            if key == 27 { break; }  // ESC key
        }
        key_ch.close();
    }
    
    // Main processing
    loop {
        match key_ch.receive() {
            Some(key) => process_key(key),
            None => break,
        }
        yield();
    }
}
```

## Implementation Phases

### Phase 1: Core Runtime (2 weeks)
- [ ] Basic goroutine creation and scheduling
- [ ] Simple channel implementation (unbuffered)
- [ ] Cooperative yield mechanism
- [ ] Stack management

### Phase 2: Advanced Features (2 weeks)  
- [ ] Buffered channels
- [ ] Channel closing and error handling
- [ ] Goroutine priorities
- [ ] Deadlock detection

### Phase 3: Integration (1 week)
- [ ] MinZ compiler integration
- [ ] Standard library implementation
- [ ] Example applications
- [ ] Performance benchmarks

### Phase 4: Optimization (1 week)
- [ ] Assembly-optimized critical paths
- [ ] Memory pool allocation
- [ ] Interrupt-driven I/O integration
- [ ] Profiling and tuning

## Future Extensions

1. **Preemptive Scheduling**: Timer-based preemption using Z80 CTC
2. **Network Channels**: Message passing between Z80 systems
3. **Goroutine Pools**: Reusable goroutine instances
4. **Async/Await Syntax**: Modern async programming patterns
5. **Green Threads**: Even lighter-weight concurrency primitives

## Conclusion

This design provides a practical, efficient concurrency model for Z80 systems that:

- Fits within 64KB memory constraints
- Provides type-safe message passing
- Integrates naturally with MinZ syntax
- Offers predictable performance characteristics
- Enables modern concurrent programming patterns

The cooperative nature eliminates the complexity of preemptive scheduling while still providing the benefits of concurrent programming for I/O-bound and structured applications.