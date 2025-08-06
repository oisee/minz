# Visual Performance Comparison

## String Print Performance (T-states)

```
Traditional Loop    MinZ Direct
===============    ===========
5 chars:  ~145     ~90  (-38%)
8 chars:  ~224     ~144 (-36%)
10 chars: ~280     ~280 (loop)
20 chars: ~560     ~560 (loop)
```

## Code Size Comparison

```
Feature          C     Assembly   MinZ
===========================================
Hello World     8KB    ~100B      ~2KB*
Fibonacci       12KB   ~150B      ~4KB*
String ops      16KB   ~200B      ~3KB*

* Includes runtime library
```
