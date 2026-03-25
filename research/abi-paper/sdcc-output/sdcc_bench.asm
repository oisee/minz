;--------------------------------------------------------
; File Created by SDCC : free open source ANSI-C Compiler
; Version 4.2.0 #13081 (Linux)
;--------------------------------------------------------
	.module bench
	.optsdcc -mz80
	
;--------------------------------------------------------
; Public variables in this module
;--------------------------------------------------------
	.globl _clamp8
	.globl _sum_to
	.globl _abs_diff
	.globl _max
	.globl _add
	.globl _twice
;--------------------------------------------------------
; special function registers
;--------------------------------------------------------
;--------------------------------------------------------
; ram data
;--------------------------------------------------------
	.area _DATA
;--------------------------------------------------------
; ram data
;--------------------------------------------------------
	.area _INITIALIZED
;--------------------------------------------------------
; absolute external ram data
;--------------------------------------------------------
	.area _DABS (ABS)
;--------------------------------------------------------
; global & static initialisations
;--------------------------------------------------------
	.area _HOME
	.area _GSINIT
	.area _GSFINAL
	.area _GSINIT
;--------------------------------------------------------
; Home
;--------------------------------------------------------
	.area _HOME
	.area _HOME
;--------------------------------------------------------
; code
;--------------------------------------------------------
	.area _CODE
;examples/c89/bench.c:3: int twice(int x) {
;	---------------------------------
; Function twice
; ---------------------------------
_twice::
;examples/c89/bench.c:4: return x + x;
	add	hl, hl
	ex	de, hl
;examples/c89/bench.c:5: }
	ret
;examples/c89/bench.c:7: int add(int a, int b) {
;	---------------------------------
; Function add
; ---------------------------------
_add::
;examples/c89/bench.c:8: return a + b;
	add	hl, de
	ex	de, hl
;examples/c89/bench.c:9: }
	ret
;examples/c89/bench.c:11: int max(int a, int b) {
;	---------------------------------
; Function max
; ---------------------------------
_max::
;examples/c89/bench.c:12: if (a > b) return a;
	ld	a, e
	sub	a, l
	ld	a, d
	sbc	a, h
	jp	PO, 00110$
	xor	a, #0x80
00110$:
	ret	P
	ex	de, hl
;examples/c89/bench.c:13: return b;
;examples/c89/bench.c:14: }
	ret
;examples/c89/bench.c:16: unsigned char abs_diff(unsigned char a, unsigned char b) {
;	---------------------------------
; Function abs_diff
; ---------------------------------
_abs_diff::
	ld	c, a
;examples/c89/bench.c:17: if (a > b) return a - b;
	ld	a, l
	sub	a, c
	jr	NC, 00102$
	ld	a, c
	sub	a, l
	ret
00102$:
;examples/c89/bench.c:18: return b - a;
	ld	a, l
	sub	a, c
;examples/c89/bench.c:19: }
	ret
;examples/c89/bench.c:21: int sum_to(int n) {
;	---------------------------------
; Function sum_to
; ---------------------------------
_sum_to::
	ex	de, hl
;examples/c89/bench.c:22: int total = 0;
	ld	hl, #0x0000
;examples/c89/bench.c:24: while (i < n) {
	ld	bc, #0x0000
00101$:
	ld	a, c
	sub	a, e
	ld	a, b
	sbc	a, d
	jp	PO, 00117$
	xor	a, #0x80
00117$:
	jp	P, 00103$
;examples/c89/bench.c:25: total = total + i;
	add	hl, bc
;examples/c89/bench.c:26: i = i + 1;
	inc	bc
	jr	00101$
00103$:
;examples/c89/bench.c:28: return total;
	ex	de, hl
;examples/c89/bench.c:29: }
	ret
;examples/c89/bench.c:31: unsigned char clamp8(unsigned char val, unsigned char lo, unsigned char hi) {
;	---------------------------------
; Function clamp8
; ---------------------------------
_clamp8::
;examples/c89/bench.c:32: if (val < lo) return lo;
	ld	c, a
	sub	a, l
	jr	NC, 00102$
	ld	a, l
	jr	00105$
00102$:
;examples/c89/bench.c:33: if (val > hi) return hi;
	ld	hl, #2
	add	hl, sp
	ld	a, (hl)
	sub	a, c
	jr	NC, 00104$
	ld	iy, #2
	add	iy, sp
	ld	a, 0 (iy)
	jr	00105$
00104$:
;examples/c89/bench.c:34: return val;
	ld	a, c
00105$:
;examples/c89/bench.c:35: }
	pop	hl
	inc	sp
	jp	(hl)
	.area _CODE
	.area _INITIALIZER
	.area _CABS (ABS)
