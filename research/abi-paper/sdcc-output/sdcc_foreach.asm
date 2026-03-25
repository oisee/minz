;--------------------------------------------------------
; File Created by SDCC : free open source ANSI-C Compiler
; Version 4.2.0 #13081 (Linux)
;--------------------------------------------------------
	.module sdcc_foreach
	.optsdcc -mz80
	
;--------------------------------------------------------
; Public variables in this module
;--------------------------------------------------------
	.globl _max_array
	.globl _sum_array
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
;/home/alice/dev/minz/research/abi-paper/sdcc-output/sdcc_foreach.c:3: uint8_t sum_array(uint8_t *buf, uint8_t n) {
;	---------------------------------
; Function sum_array
; ---------------------------------
_sum_array::
	push	ix
	ld	ix,#0
	add	ix,sp
	ex	de, hl
;/home/alice/dev/minz/research/abi-paper/sdcc-output/sdcc_foreach.c:4: uint8_t s = 0;
;/home/alice/dev/minz/research/abi-paper/sdcc-output/sdcc_foreach.c:5: for (uint8_t i = 0; i < n; i++) {
	ld	bc, #0x0
00103$:
	ld	a, b
	sub	a, 4 (ix)
	jr	NC, 00101$
;/home/alice/dev/minz/research/abi-paper/sdcc-output/sdcc_foreach.c:6: s += buf[i];
	ld	l, b
	ld	h, #0x00
	add	hl, de
	ld	a, (hl)
	add	a, c
	ld	c, a
;/home/alice/dev/minz/research/abi-paper/sdcc-output/sdcc_foreach.c:5: for (uint8_t i = 0; i < n; i++) {
	inc	b
	jr	00103$
00101$:
;/home/alice/dev/minz/research/abi-paper/sdcc-output/sdcc_foreach.c:8: return s;
	ld	a, c
;/home/alice/dev/minz/research/abi-paper/sdcc-output/sdcc_foreach.c:9: }
	pop	ix
	pop	hl
	inc	sp
	jp	(hl)
;/home/alice/dev/minz/research/abi-paper/sdcc-output/sdcc_foreach.c:11: uint8_t max_array(uint8_t *buf, uint8_t n) {
;	---------------------------------
; Function max_array
; ---------------------------------
_max_array::
	push	ix
	ld	ix,#0
	add	ix,sp
	ex	de, hl
;/home/alice/dev/minz/research/abi-paper/sdcc-output/sdcc_foreach.c:12: uint8_t m = 0;
;/home/alice/dev/minz/research/abi-paper/sdcc-output/sdcc_foreach.c:13: for (uint8_t i = 0; i < n; i++) {
	ld	bc, #0x0
00105$:
	ld	a, b
	sub	a, 4 (ix)
	jr	NC, 00103$
;/home/alice/dev/minz/research/abi-paper/sdcc-output/sdcc_foreach.c:14: if (buf[i] > m) m = buf[i];
	ld	l, b
	ld	h, #0x00
	add	hl, de
	ld	l, (hl)
;	spillPairReg hl
	ld	a, c
	sub	a, l
	jr	NC, 00106$
	ld	c, l
00106$:
;/home/alice/dev/minz/research/abi-paper/sdcc-output/sdcc_foreach.c:13: for (uint8_t i = 0; i < n; i++) {
	inc	b
	jr	00105$
00103$:
;/home/alice/dev/minz/research/abi-paper/sdcc-output/sdcc_foreach.c:16: return m;
	ld	a, c
;/home/alice/dev/minz/research/abi-paper/sdcc-output/sdcc_foreach.c:17: }
	pop	ix
	pop	hl
	inc	sp
	jp	(hl)
	.area _CODE
	.area _INITIALIZER
	.area _CABS (ABS)
