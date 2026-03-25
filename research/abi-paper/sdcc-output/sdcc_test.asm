;--------------------------------------------------------
; File Created by SDCC : free open source ANSI-C Compiler
; Version 4.2.0 #13081 (Linux)
;--------------------------------------------------------
	.module sdcc_test
	.optsdcc -mz80
	
;--------------------------------------------------------
; Public variables in this module
;--------------------------------------------------------
	.globl _square
	.globl _triple
	.globl _double_val
	.globl _gcd
	.globl _abs_diff
	.globl _fibonacci
	.globl _max8
	.globl _add16
	.globl _sub8
	.globl _add8
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
;/tmp/sdcc_test.c:2: unsigned char add8(unsigned char a, unsigned char b) { return a + b; }
;	---------------------------------
; Function add8
; ---------------------------------
_add8::
	add	a, l
	ret
;/tmp/sdcc_test.c:3: unsigned char sub8(unsigned char a, unsigned char b) { return a - b; }
;	---------------------------------
; Function sub8
; ---------------------------------
_sub8::
	sub	a, l
	ret
;/tmp/sdcc_test.c:4: unsigned int add16(unsigned int a, unsigned int b) { return a + b; }
;	---------------------------------
; Function add16
; ---------------------------------
_add16::
	add	hl, de
	ex	de, hl
	ret
;/tmp/sdcc_test.c:5: unsigned char max8(unsigned char a, unsigned char b) { return a > b ? a : b; }
;	---------------------------------
; Function max8
; ---------------------------------
_max8::
	ld	c, a
	ld	a, l
	sub	a, c
	jr	NC, 00103$
	ld	a, c
	ret
00103$:
	ld	a, l
	ret
;/tmp/sdcc_test.c:6: unsigned int fibonacci(unsigned char n) {
;	---------------------------------
; Function fibonacci
; ---------------------------------
_fibonacci::
	ld	c, a
;/tmp/sdcc_test.c:7: unsigned int a = 0, b = 1, t;
	ld	de, #0x0000
	ld	hl, #0x0001
;/tmp/sdcc_test.c:8: while (n--) { t = a + b; a = b; b = t; }
00101$:
	ld	a, c
	dec	c
	or	a, a
	ret	Z
	ld	a, l
	add	a, e
	ld	e, a
	ld	a, h
	adc	a, d
	ld	b, e
	ex	de, hl
	ld	l, b
;	spillPairReg hl
;	spillPairReg hl
	ld	h, a
;	spillPairReg hl
;	spillPairReg hl
;/tmp/sdcc_test.c:9: return a;
;/tmp/sdcc_test.c:10: }
	jr	00101$
;/tmp/sdcc_test.c:11: unsigned char abs_diff(unsigned char a, unsigned char b) {
;	---------------------------------
; Function abs_diff
; ---------------------------------
_abs_diff::
	ld	c, a
;/tmp/sdcc_test.c:12: return a > b ? a - b : b - a;
	ld	a, l
	sub	a, c
	jr	NC, 00103$
	ld	a, c
	sub	a, l
	ret
00103$:
	ld	a, l
	sub	a, c
;/tmp/sdcc_test.c:13: }
	ret
;/tmp/sdcc_test.c:14: unsigned char gcd(unsigned char a, unsigned char b) {
;	---------------------------------
; Function gcd
; ---------------------------------
_gcd::
	ld	c, a
;/tmp/sdcc_test.c:15: while (b) { unsigned char t = b; b = a % b; a = t; }
00101$:
	ld	a, l
	or	a, a
	jr	Z, 00103$
	ld	b, l
	push	bc
	ld	a, c
	call	__moduchar
	pop	bc
	ld	l, e
;	spillPairReg hl
;	spillPairReg hl
	ld	c, b
	jr	00101$
00103$:
;/tmp/sdcc_test.c:16: return a;
	ld	a, c
;/tmp/sdcc_test.c:17: }
	ret
;/tmp/sdcc_test.c:18: unsigned char double_val(unsigned char x) { return x + x; }
;	---------------------------------
; Function double_val
; ---------------------------------
_double_val::
	add	a, a
	ret
;/tmp/sdcc_test.c:19: unsigned char triple(unsigned char x) { return x * 3; }
;	---------------------------------
; Function triple
; ---------------------------------
_triple::
	ld	c, a
	add	a, a
	add	a, c
	ret
;/tmp/sdcc_test.c:20: unsigned int square(unsigned char x) { return (unsigned int)x * x; }
;	---------------------------------
; Function square
; ---------------------------------
_square::
	ld	l, a
;	spillPairReg hl
;	spillPairReg hl
	ld	h, #0x00
;	spillPairReg hl
;	spillPairReg hl
	ld	e, l
	ld	d, h
	jp	__mulint
	.area _CODE
	.area _INITIALIZER
	.area _CABS (ABS)
