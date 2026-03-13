;--------------------------------------------------------
; File Created by SDCC : free open source ANSI-C Compiler
; Version 4.2.0 #13081 (Linux)
;--------------------------------------------------------
	.module sdcc_minmax
	.optsdcc -mz80
	
;--------------------------------------------------------
; Public variables in this module
;--------------------------------------------------------
	.globl _min_of
	.globl _minmax
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
;/tmp/sdcc_minmax.c:3: void minmax(uint16_t a, uint16_t b, uint16_t *lo, uint16_t *hi) {
;	---------------------------------
; Function minmax
; ---------------------------------
_minmax::
	push	ix
	ld	ix,#0
	add	ix,sp
	push	af
	ld	c, l
	ld	b, h
;/tmp/sdcc_minmax.c:4: if (a <= b) { *lo = a; *hi = b; }
	ld	l, 4 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	h, 5 (ix)
;	spillPairReg hl
;	spillPairReg hl
	ld	a, 6 (ix)
	ld	-2 (ix), a
	ld	a, 7 (ix)
	ld	-1 (ix), a
	ld	a, e
	sub	a, c
	ld	a, d
	sbc	a, b
	jr	C, 00102$
	ld	(hl), c
	inc	hl
	ld	(hl), b
	pop	hl
	push	hl
	ld	(hl), e
	inc	hl
	ld	(hl), d
	jr	00104$
00102$:
;/tmp/sdcc_minmax.c:5: else        { *lo = b; *hi = a; }
	ld	(hl), e
	inc	hl
	ld	(hl), d
	pop	hl
	push	hl
	ld	(hl), c
	inc	hl
	ld	(hl), b
00104$:
;/tmp/sdcc_minmax.c:6: }
	ld	sp, ix
	pop	ix
	pop	hl
	pop	af
	pop	af
	jp	(hl)
;/tmp/sdcc_minmax.c:8: uint16_t min_of(uint16_t a, uint16_t b) {
;	---------------------------------
; Function min_of
; ---------------------------------
_min_of::
	push	ix
	ld	ix,#0
	add	ix,sp
	push	af
	push	af
	ld	c, l
	ld	b, h
;/tmp/sdcc_minmax.c:10: minmax(a, b, &lo, &hi);
	ld	hl, #2
	add	hl, sp
	push	hl
	ld	hl, #2
	add	hl, sp
	push	hl
	ld	l, c
;	spillPairReg hl
;	spillPairReg hl
	ld	h, b
;	spillPairReg hl
;	spillPairReg hl
	call	_minmax
;/tmp/sdcc_minmax.c:11: return lo;
	pop	de
	push	de
;/tmp/sdcc_minmax.c:12: }
	ld	sp, ix
	pop	ix
	ret
	.area _CODE
	.area _INITIALIZER
	.area _CABS (ABS)
