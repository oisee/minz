;--------------------------------------------------------
; File Created by SDCC : free open source ANSI-C Compiler
; Version 4.2.0 #13081 (Linux)
;--------------------------------------------------------
	.module sdcc_swap
	.optsdcc -mz80
	
;--------------------------------------------------------
; Public variables in this module
;--------------------------------------------------------
	.globl _swap
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
;/tmp/sdcc_swap.c:4: void swap(uint16_t a, uint16_t b, uint16_t *out_a, uint16_t *out_b) {
;	---------------------------------
; Function swap
; ---------------------------------
_swap::
	push	ix
	ld	ix,#0
	add	ix,sp
	ld	c, l
	ld	b, h
;/tmp/sdcc_swap.c:5: *out_a = b;
	ld	l, 4 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, 5 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	(hl), e
	inc	hl
	ld	(hl), d
;/tmp/sdcc_swap.c:6: *out_b = a;
	ld	l, 6 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, 7 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	(hl), c
	inc	hl
	ld	(hl), b
;/tmp/sdcc_swap.c:7: }
	pop	ix
	pop	hl
	pop	af
	pop	af
	jp	(hl)
	.area _CODE
	.area _INITIALIZER
	.area _CABS (ABS)
