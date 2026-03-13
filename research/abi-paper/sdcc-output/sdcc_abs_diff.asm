;--------------------------------------------------------
; File Created by SDCC : free open source ANSI-C Compiler
; Version 4.2.0 #13081 (Linux)
;--------------------------------------------------------
	.module sdcc_abs_diff
	.optsdcc -mz80
	
;--------------------------------------------------------
; Public variables in this module
;--------------------------------------------------------
	.globl _main
	.globl _abs_diff
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
;sdcc_abs_diff.c:3: uint8_t abs_diff(uint8_t a, uint8_t b) {
;	---------------------------------
; Function abs_diff
; ---------------------------------
_abs_diff::
;sdcc_abs_diff.c:4: if (a >= b) return a - b;
	ld	c, a
	sub	a, l
	jr	C, 00102$
	ld	a, c
	sub	a, l
	ret
00102$:
;sdcc_abs_diff.c:5: return b - a;
	ld	a, l
	sub	a, c
;sdcc_abs_diff.c:6: }
	ret
;sdcc_abs_diff.c:8: uint8_t main(void) {
;	---------------------------------
; Function main
; ---------------------------------
_main::
;sdcc_abs_diff.c:9: return abs_diff(10, 3);
	ld	l, #0x03
;	spillPairReg hl
;	spillPairReg hl
	ld	a, #0x0a
;sdcc_abs_diff.c:10: }
	jp	_abs_diff
	.area _CODE
	.area _INITIALIZER
	.area _CABS (ABS)
