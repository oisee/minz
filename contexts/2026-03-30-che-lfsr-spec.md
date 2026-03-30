# Che LFSR Spec — verified 2026-03-30

## LFSR
- 32-bit Galois, polynomial **0xB4BCD35C**
- Init: `(seed << 16) | ((seg_id * 13 + 0xBEEF) & 0xFFFF)`
- Warmup: 8 steps before first point

## Per point
```
bit = state & 1; state >>= 1
if bit: state ^= 0xB4BCD35C
lx = (state & 0xFFFF) % rw
ly = ((state >> 16) & 0xFFFF) % rh
lx = (lx // block_size) * block_size  // grid-align
ly = (ly // block_size) * block_size
XOR block_size×block_size pixels at (rx+lx, ry+ly)
```

## Z80 optimal (72T avg per step)
```z80
SRL D; RR E; RR H; RR L    ; 32T
JR NC, skip                 ; 7/12T
LD A,D; XOR 0xB4; LD D,A   ; 15T
LD A,E; XOR 0xBC; LD E,A   ; 15T
LD A,H; XOR 0xD3; LD H,A   ; 15T
LD A,L; XOR 0x5C; LD L,A   ; 15T
skip:
```

## Data: segmented_che/seeds.txt
- 85 segments, each: (seed:u16, seg_id, rx, ry, rw, rh, block_size, num_points)
- Total: ~3500 points across 4 levels (blk=8,4,2,1)

## Verified
Python render matches level3_err3828.png exactly.
