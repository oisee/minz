;--------------------------------------------------------
; File Created by SDCC : free open source ANSI-C Compiler
; Version 4.2.0 #13081 (Linux)
;--------------------------------------------------------
	.module old_wrap_twice
	.optsdcc -mz80
	
;--------------------------------------------------------
; Public variables in this module
;--------------------------------------------------------
	.globl _execute
	.globl _wrap_twice
	.globl _wrap_min
	.globl _demo
	.globl _square
	.globl _min8
	.globl _clamp
	.globl _abs_diff
	.globl _add
	.globl _twice
	.globl _fact6
	.globl _factorial
	.globl _sum10
	.globl _sum_to
	.globl _fib10
	.globl _fibonacci
	.globl _a
	.globl _result
;--------------------------------------------------------
; special function registers
;--------------------------------------------------------
;--------------------------------------------------------
; ram data
;--------------------------------------------------------
	.area _DATA
_result::
	.ds 2
_a::
	.ds 2
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
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_wrap_twice.c:4: int fibonacci(int n) {
;	---------------------------------
; Function fibonacci
; ---------------------------------
_fibonacci::
	ex	de, hl
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_wrap_twice.c:5: int a = 0, b = 1;
	ld	hl, #0x0000
	ld	bc, #0x0001
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_wrap_twice.c:6: while (n > 0) { int t = a + b; a = b; b = t; n = n - 1; }
00101$:
	xor	a, a
	cp	a, e
	sbc	a, d
	jp	PO, 00117$
	xor	a, #0x80
00117$:
	jp	P, 00103$
	add	hl, bc
	push	hl
	pop	iy
	ld	l, c
;	spillPairReg hl
;	spillPairReg hl
	ld	h, b
;	spillPairReg hl
;	spillPairReg hl
	push	iy
	pop	bc
	dec	de
	jp	00101$
00103$:
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_wrap_twice.c:7: return a;
	ex	de, hl
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_wrap_twice.c:8: }
	ret
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_wrap_twice.c:9: int fib10(void) { return fibonacci(10); }
;	---------------------------------
; Function fib10
; ---------------------------------
_fib10::
	ld	hl, #0x000a
	jp	_fibonacci
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_wrap_twice.c:11: int sum_to(int n) {
;	---------------------------------
; Function sum_to
; ---------------------------------
_sum_to::
	ex	de, hl
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_wrap_twice.c:12: int total = 0, i = 0;
	ld	hl, #0x0000
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_wrap_twice.c:13: while (i < n) { total = total + i; i = i + 1; }
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
	add	hl, bc
	inc	bc
	jp	00101$
00103$:
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_wrap_twice.c:14: return total;
	ex	de, hl
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_wrap_twice.c:15: }
	ret
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_wrap_twice.c:16: int sum10(void) { return sum_to(10); }
;	---------------------------------
; Function sum10
; ---------------------------------
_sum10::
	ld	hl, #0x000a
	jp	_sum_to
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_wrap_twice.c:18: int factorial(int n) {
;	---------------------------------
; Function factorial
; ---------------------------------
_factorial::
	ld	c, l
	ld	b, h
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_wrap_twice.c:19: int r = 1;
	ld	hl, #0x0001
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_wrap_twice.c:20: while (n > 1) { r = r * n; n = n - 1; }
00101$:
	ld	a, #0x01
	cp	a, c
	ld	a, #0x00
	sbc	a, b
	jp	PO, 00117$
	xor	a, #0x80
00117$:
	jp	P, 00103$
	push	bc
	ld	e, c
	ld	d, b
	call	__mulint
	pop	bc
	ex	de, hl
	dec	bc
	jp	00101$
00103$:
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_wrap_twice.c:21: return r;
	ex	de, hl
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_wrap_twice.c:22: }
	ret
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_wrap_twice.c:23: int fact6(void) { return factorial(6); }
;	---------------------------------
; Function fact6
; ---------------------------------
_fact6::
	ld	hl, #0x0006
	jp	_factorial
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_wrap_twice.c:26: int twice(int x) { return x + x; }
;	---------------------------------
; Function twice
; ---------------------------------
_twice::
	add	hl, hl
	ex	de, hl
	ret
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_wrap_twice.c:27: int add(int a, int b) { return a + b; }
;	---------------------------------
; Function add
; ---------------------------------
_add::
	add	hl, de
	ex	de, hl
	ret
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_wrap_twice.c:30: unsigned char abs_diff(unsigned char a, unsigned char b) {
;	---------------------------------
; Function abs_diff
; ---------------------------------
_abs_diff::
	ld	c, a
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_wrap_twice.c:31: if (a > b) return a - b;
	ld	a, l
	sub	a, c
	jr	NC, 00102$
	ld	a, c
	sub	a, l
	ret
00102$:
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_wrap_twice.c:32: return b - a;
	ld	a, l
	sub	a, c
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_wrap_twice.c:33: }
	ret
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_wrap_twice.c:35: unsigned char clamp(unsigned char val, unsigned char lo, unsigned char hi) {
;	---------------------------------
; Function clamp
; ---------------------------------
_clamp::
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_wrap_twice.c:36: if (val < lo) return lo;
	ld	c, a
	sub	a, l
	jr	NC, 00102$
	ld	a, l
	jp	00105$
00102$:
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_wrap_twice.c:37: if (val > hi) return hi;
	ld	hl, #2
	add	hl, sp
	ld	a, (hl)
	sub	a, c
	jr	NC, 00104$
	ld	iy, #2
	add	iy, sp
	ld	a, 0 (iy)
	jp	00105$
00104$:
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_wrap_twice.c:38: return val;
	ld	a, c
00105$:
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_wrap_twice.c:39: }
	pop	hl
	inc	sp
	jp	(hl)
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_wrap_twice.c:41: unsigned char min8(unsigned char a, unsigned char b) {
;	---------------------------------
; Function min8
; ---------------------------------
_min8::
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_wrap_twice.c:42: if (a < b) return a;
	ld	c, a
	sub	a, l
	jr	NC, 00102$
	ld	a, c
	ret
00102$:
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_wrap_twice.c:43: return b;
	ld	a, l
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_wrap_twice.c:44: }
	ret
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_wrap_twice.c:47: int square(int x) { return x * x; }
;	---------------------------------
; Function square
; ---------------------------------
_square::
	ld	e, l
	ld	d, h
	jp	__mulint
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_wrap_twice.c:48: int demo(void) { return square(4) + twice(5); }
;	---------------------------------
; Function demo
; ---------------------------------
_demo::
	ld	hl, #0x0004
	call	_square
	push	de
	ld	hl, #0x0005
	call	_twice
	ex	de, hl
	pop	de
	add	hl, de
	ex	de, hl
	ret
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_wrap_twice.c:51: unsigned char wrap_min(unsigned char a, unsigned char b) { return min8(a, b); }
;	---------------------------------
; Function wrap_min
; ---------------------------------
_wrap_min::
	ld	c, a
	jp	_min8
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_wrap_twice.c:52: int wrap_twice(int x) { return twice(x); }
;	---------------------------------
; Function wrap_twice
; ---------------------------------
_wrap_twice::
	jp	_twice
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_wrap_twice.c:56: void execute(void) { result = wrap_twice(a); }
;	---------------------------------
; Function execute
; ---------------------------------
_execute::
	ld	hl, (_a)
	call	_wrap_twice
	ld	(_result), de
	ret
	.area _CODE
	.area _INITIALIZER
	.area _CABS (ABS)
