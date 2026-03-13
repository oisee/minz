;--------------------------------------------------------
; File Created by SDCC : free open source ANSI-C Compiler
; Version 4.2.0 #13081 (Linux)
;--------------------------------------------------------
	.module sdcc_abs_diff_u16
	.optsdcc -mz80
	
;--------------------------------------------------------
; Public variables in this module
;--------------------------------------------------------
	.globl _abs_diff_u16
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
;/tmp/sdcc_abs_diff_u16.c:3: uint16_t abs_diff_u16(uint16_t a, uint16_t b) {
;	---------------------------------
; Function abs_diff_u16
; ---------------------------------
_abs_diff_u16::
;/tmp/sdcc_abs_diff_u16.c:4: if (a < b) return b - a;
	ld	a, l
	sub	a, e
	ld	a, h
	sbc	a, d
	jr	NC, 00102$
	ld	a, e
	sub	a, l
	ld	e, a
	ld	a, d
	sbc	a, h
	ld	d, a
	ret
00102$:
;/tmp/sdcc_abs_diff_u16.c:5: return a - b;
	cp	a, a
	sbc	hl, de
	ex	de, hl
;/tmp/sdcc_abs_diff_u16.c:6: }
	ret
	.area _CODE
	.area _INITIALIZER
	.area _CABS (ABS)
