;--------------------------------------------------------
; File Created by SDCC : free open source ISO C Compiler
; Version 4.5.0 #15242 (Linux)
;--------------------------------------------------------
	.module old_demo
	
	.optsdcc -mz80 sdcccall(1)
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
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_demo.c:4: int fibonacci(int n) {
;	---------------------------------
; Function fibonacci
; ---------------------------------
_fibonacci::
	ex	de, hl
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_demo.c:5: int a = 0, b = 1;
	ld	hl, #0x0000
	ld	bc, #0x0001
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_demo.c:6: while (n > 0) { int t = a + b; a = b; b = t; n = n - 1; }
00101$:
	xor	a, a
	cp	a, e
	sbc	a, d
	jp	PO, 00121$
	xor	a, #0x80
00121$:
	jp	P, 00103$
	add	hl, bc
	push	hl
	pop	iy
	ld	l, c
	ld	h, b
	push	iy
	pop	bc
	dec	de
	jr	00101$
00103$:
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_demo.c:7: return a;
	ex	de, hl
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_demo.c:8: }
	ret
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_demo.c:9: int fib10(void) { return fibonacci(10); }
;	---------------------------------
; Function fib10
; ---------------------------------
_fib10::
	ld	hl, #0x000a
	jp	_fibonacci
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_demo.c:11: int sum_to(int n) {
;	---------------------------------
; Function sum_to
; ---------------------------------
_sum_to::
	ex	de, hl
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_demo.c:12: int total = 0, i = 0;
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_demo.c:13: while (i < n) { total = total + i; i = i + 1; }
	ld	hl, #0x0000
	ld	b, h
	ld	c, l
00101$:
	ld	a, c
	sub	a, e
	ld	a, b
	sbc	a, d
	jp	PO, 00121$
	xor	a, #0x80
00121$:
	jp	P, 00103$
	add	hl, bc
	inc	bc
	jr	00101$
00103$:
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_demo.c:14: return total;
	ex	de, hl
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_demo.c:15: }
	ret
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_demo.c:16: int sum10(void) { return sum_to(10); }
;	---------------------------------
; Function sum10
; ---------------------------------
_sum10::
	ld	hl, #0x000a
	jp	_sum_to
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_demo.c:18: int factorial(int n) {
;	---------------------------------
; Function factorial
; ---------------------------------
_factorial::
	ex	de, hl
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_demo.c:19: int r = 1;
	ld	hl, #0x0001
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_demo.c:20: while (n > 1) { r = r * n; n = n - 1; }
00101$:
	ld	a, #0x01
	cp	a, e
	ld	a, #0x00
	sbc	a, d
	jp	PO, 00121$
	xor	a, #0x80
00121$:
	jp	P, 00103$
	push	de
	call	__mulint
	ex	de, hl
	pop	de
	dec	de
	jr	00101$
00103$:
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_demo.c:21: return r;
	ex	de, hl
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_demo.c:22: }
	ret
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_demo.c:23: int fact6(void) { return factorial(6); }
;	---------------------------------
; Function fact6
; ---------------------------------
_fact6::
	ld	hl, #0x0006
	jp	_factorial
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_demo.c:26: int twice(int x) { return x + x; }
;	---------------------------------
; Function twice
; ---------------------------------
_twice::
	add	hl, hl
	ex	de, hl
	ret
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_demo.c:27: int add(int a, int b) { return a + b; }
;	---------------------------------
; Function add
; ---------------------------------
_add::
	add	hl, de
	ex	de, hl
	ret
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_demo.c:30: unsigned char abs_diff(unsigned char a, unsigned char b) {
;	---------------------------------
; Function abs_diff
; ---------------------------------
_abs_diff::
	ld	c, a
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_demo.c:31: if (a > b) return a - b;
	ld	a, l
	sub	a, c
	jr	NC, 00102$
	ld	a, c
	sub	a, l
	ret
00102$:
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_demo.c:32: return b - a;
	ld	a, l
	sub	a, c
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_demo.c:33: }
	ret
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_demo.c:35: unsigned char clamp(unsigned char val, unsigned char lo, unsigned char hi) {
;	---------------------------------
; Function clamp
; ---------------------------------
_clamp::
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_demo.c:36: if (val < lo) return lo;
	ld	c, a
	sub	a, l
	jr	NC, 00102$
	ld	a, l
	jr	00105$
00102$:
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_demo.c:37: if (val > hi) return hi;
	ld	iy, #2
	add	iy, sp
	ld	a, 0 (iy)
	sub	a, c
	jr	NC, 00104$
	ld	a, 0 (iy)
	jr	00105$
00104$:
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_demo.c:38: return val;
	ld	a, c
00105$:
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_demo.c:39: }
	pop	hl
	inc	sp
	jp	(hl)
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_demo.c:41: unsigned char min8(unsigned char a, unsigned char b) {
;	---------------------------------
; Function min8
; ---------------------------------
_min8::
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_demo.c:42: if (a < b) return a;
	cp	a, l
	ret	C
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_demo.c:43: return b;
	ld	a, l
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_demo.c:44: }
	ret
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_demo.c:47: int square(int x) { return x * x; }
;	---------------------------------
; Function square
; ---------------------------------
_square::
	ld	e, l
	ld	d, h
	jp	__mulint
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_demo.c:48: int demo(void) { return square(4) + twice(5); }
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
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_demo.c:51: unsigned char wrap_min(unsigned char a, unsigned char b) { return min8(a, b); }
;	---------------------------------
; Function wrap_min
; ---------------------------------
_wrap_min::
	jp	_min8
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_demo.c:52: int wrap_twice(int x) { return twice(x); }
;	---------------------------------
; Function wrap_twice
; ---------------------------------
_wrap_twice::
	jp	_twice
;/home/alice/dev/minz-artifacts/grace-workspaces/c89-old-claims-replay/research/c89-benchmark/old-claims/cases/old_demo.c:56: void execute(void) { result = demo(); }
;	---------------------------------
; Function execute
; ---------------------------------
_execute::
	call	_demo
	ld	(_result), de
	ret
	.area _CODE
	.area _INITIALIZER
	.area _CABS (ABS)
