# ZVDB Implementation Requirements for MinZ

## Overview

This document analyzes the feasibility of implementing ZVDB (Z80 Vector Database) in MinZ and identifies missing language features.

## Current MinZ Capabilities

### ✅ Already Supported
- Structs and enums
- Fixed-size arrays
- Bit manipulation (AND, OR, XOR, shifts)
- Inline assembly
- Direct memory access
- Module system
- Shadow registers
- Self-modifying code optimization
- Compile-time metaprogramming (Lua)

### ❌ Critical Missing Features

#### 1. **File I/O**
ZVDB needs to persist data. MinZ needs basic file operations:
```minz
// Proposed std.file module
module std.file;

pub type FileHandle = u16;

pub fn open(path: *u8, mode: u8) -> FileHandle;
pub fn read(fh: FileHandle, buf: *mut u8, len: u16) -> i16;
pub fn write(fh: FileHandle, buf: *u8, len: u16) -> i16;
pub fn seek(fh: FileHandle, offset: i32, whence: u8) -> i32;
pub fn close(fh: FileHandle) -> void;
```

#### 2. **Dynamic Memory Allocation**
For variable-sized data structures:
```minz
// Proposed std.alloc module
module std.alloc;

pub fn malloc(size: u16) -> *mut u8;
pub fn realloc(ptr: *mut u8, size: u16) -> *mut u8;
pub fn free(ptr: *mut u8) -> void;
```

#### 3. **String Operations**
Basic string handling:
```minz
// Proposed std.string module
module std.string;

pub fn strlen(s: *u8) -> u16;
pub fn strcpy(dst: *mut u8, src: *u8) -> void;
pub fn strcmp(a: *u8, b: *u8) -> i8;
pub fn strcat(dst: *mut u8, src: *u8) -> void;
```

#### 4. **Fixed-Point Arithmetic**
Since Z80 lacks floating-point, we need fixed-point math:
```minz
// Proposed std.fixmath module
module std.fixmath;

pub type Fix16 = i16;  // 8.8 fixed-point
pub type Fix32 = i32;  // 16.16 fixed-point

pub fn fix16_from_int(n: i16) -> Fix16;
pub fn fix16_mul(a: Fix16, b: Fix16) -> Fix16;
pub fn fix16_div(a: Fix16, b: Fix16) -> Fix16;
pub fn fix16_sqrt(a: Fix16) -> Fix16;
```

#### 5. **Error Handling**
Basic error propagation:
```minz
// Proposed error handling approach
pub type Result<T> = enum {
    Ok(T),
    Err(u8),  // Error code
};
```

## ZVDB-Z80 Minimal Implementation

Given current MinZ capabilities, we can implement a basic ZVDB with:

### 1. **Fixed-Size Vectors**
```minz
struct Vector256 {
    bits: [u8; 32],  // 256 bits = 32 bytes
}

impl Vector256 {
    fn hamming_distance(self: *Vector256, other: *Vector256) -> u8 {
        let mut dist: u8 = 0;
        for i in 0..32 {
            let xor = self.bits[i] ^ other.bits[i];
            dist = dist + popcount(xor);
        }
        return dist;
    }
}
```

### 2. **Simple Database Structure**
```minz
const MAX_VECTORS: u16 = 1000;

struct VectorDB {
    vectors: [Vector256; MAX_VECTORS],
    count: u16,
    metadata: [u16; MAX_VECTORS],  // Associated data
}
```

### 3. **Search Implementation**
```minz
fn search_nearest(db: *VectorDB, query: *Vector256, k: u8) -> [u16; 10] {
    let mut results: [u16; 10];
    let mut distances: [u8; 10];
    
    // Initialize with max distance
    for i in 0..10 {
        distances[i] = 255;
    }
    
    // Brute force search
    for i in 0..db.count {
        let dist = query.hamming_distance(&db.vectors[i]);
        
        // Insert into results if better than worst
        if dist < distances[k-1] {
            insert_sorted(results, distances, i, dist, k);
        }
    }
    
    return results;
}
```

## Proposed Implementation Plan

### Phase 1: Core Library Extensions
1. Implement basic file I/O for Z80 (using CP/M or custom routines)
2. Add simple memory allocator
3. Create fixed-point math library

### Phase 2: ZVDB Core
1. Implement 1-bit quantization
2. Create vector storage format
3. Build search algorithms

### Phase 3: Optimizations
1. Use SMC for frequently accessed vectors
2. Implement SIMD-style operations using Z80 block instructions
3. Add simple indexing structures

## Alternative: Hybrid Approach

Use MinZ for performance-critical parts and interface with external storage:

```minz
// MinZ handles vector operations
@smc_optimize
fn vector_search_kernel(vectors: *Vector256, count: u16, query: *Vector256) -> u16 {
    // Optimized search implementation
}

// External system handles I/O and persistence
declare fn load_vectors(filename: *u8, buffer: *mut Vector256) -> u16;
declare fn save_vectors(filename: *u8, buffer: *Vector256, count: u16) -> bool;
```

## Conclusion

While MinZ lacks some features for a full ZVDB implementation, it has sufficient capabilities for a basic vector database focused on:
- Fixed-size vectors (256-bit)
- Hamming distance calculations
- Simple brute-force search
- Basic persistence (with added file I/O)

The language's strength in low-level optimization (SMC, inline assembly) makes it well-suited for the performance-critical vector operations that are the heart of ZVDB.