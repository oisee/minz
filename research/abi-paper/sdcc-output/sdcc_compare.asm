;--------------------------------------------------------
; File Created by SDCC : free open source ANSI-C Compiler
; Version 4.2.0 #13081 (Linux)
;--------------------------------------------------------
	.module sdcc_compare
	.optsdcc -mz80
	
;--------------------------------------------------------
; Public variables in this module
;--------------------------------------------------------
	.globl _gcd
	.globl _abs_diff
	.globl _add16
	.globl _double_val
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
;/tmp/sdcc_compare.c:1: unsigned char add8(unsigned char a, unsigned char b) { return a + b; }
;	---------------------------------
; Function add8
; ---------------------------------
_add8::
	add	a, l
	ret
;/tmp/sdcc_compare.c:2: unsigned char double_val(unsigned char x) { return x + x; }
;	---------------------------------
; Function double_val
; ---------------------------------
_double_val::
	add	a, a
	ret
;/tmp/sdcc_compare.c:3: unsigned int add16(unsigned int a, unsigned int b) { return a + b; }
;	---------------------------------
; Function add16
; ---------------------------------
_add16::
	add	hl, de
	ex	de, hl
	ret
;/tmp/sdcc_compare.c:4: unsigned char abs_diff(unsigned char a, unsigned char b) {
;	---------------------------------
; Function abs_diff
; ---------------------------------
_abs_diff::
	ld	c, a
;/tmp/sdcc_compare.c:5: return a > b ? a - b : b - a;
	ld	a, l
	sub	a, c
	jr	NC, 00103$
	ld	a, c
	sub	a, l
	ret
00103$:
	ld	a, l
	sub	a, c
;/tmp/sdcc_compare.c:6: }
	ret
;/tmp/sdcc_compare.c:7: unsigned char gcd(unsigned char a, unsigned char b) {
;	---------------------------------
; Function gcd
; ---------------------------------
_gcd::
	ld	c, a
;/tmp/sdcc_compare.c:8: while (b) { unsigned char t = b; b = a % b; a = t; }
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
;/tmp/sdcc_compare.c:9: return a;
	ld	a, c
;/tmp/sdcc_compare.c:10: }
	ret
	.area _CODE
	.area _INITIALIZER
	.area _CABS (ABS)
