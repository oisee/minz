;--------------------------------------------------------
; File Created by SDCC : free open source ANSI-C Compiler
; Version 4.2.0 #13081 (Linux)
;--------------------------------------------------------
	.module sdcc_gcd
	.optsdcc -mz80
	
;--------------------------------------------------------
; Public variables in this module
;--------------------------------------------------------
	.globl _gcd
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
;/tmp/sdcc_gcd.c:3: uint8_t gcd(uint8_t a, uint8_t b) {
;	---------------------------------
; Function gcd
; ---------------------------------
_gcd::
	ld	c, a
;/tmp/sdcc_gcd.c:4: while (a != b) {
00104$:
	ld	a, c
	sub	a, l
	jr	Z, 00106$
;/tmp/sdcc_gcd.c:5: if (a > b) a = a - b;
	ld	a, l
	sub	a, c
	jr	NC, 00102$
	ld	a, c
	sub	a, l
	ld	c, a
	jr	00104$
00102$:
;/tmp/sdcc_gcd.c:6: else       b = b - a;
	ld	a, l
	sub	a, c
	ld	l, a
;	spillPairReg hl
;	spillPairReg hl
	jr	00104$
00106$:
;/tmp/sdcc_gcd.c:8: return a;
	ld	a, c
;/tmp/sdcc_gcd.c:9: }
	ret
	.area _CODE
	.area _INITIALIZER
	.area _CABS (ABS)
