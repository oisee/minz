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
;/home/alice/dev/minz/research/abi-paper/sdcc-output/sdcc_abs_diff.c:3: uint8_t abs_diff(uint8_t a, uint8_t b) {
;	---------------------------------
; Function abs_diff
; ---------------------------------
_abs_diff::
;/home/alice/dev/minz/research/abi-paper/sdcc-output/sdcc_abs_diff.c:4: if (a >= b) return a - b;
	ld	c, a
	sub	a, l
	jr	C, 00102$
	ld	a, c
	sub	a, l
	ret
00102$:
;/home/alice/dev/minz/research/abi-paper/sdcc-output/sdcc_abs_diff.c:5: return b - a;
	ld	a, l
	sub	a, c
;/home/alice/dev/minz/research/abi-paper/sdcc-output/sdcc_abs_diff.c:6: }
	ret
;/home/alice/dev/minz/research/abi-paper/sdcc-output/sdcc_abs_diff.c:8: uint8_t main(void) {
;	---------------------------------
; Function main
; ---------------------------------
_main::
;/home/alice/dev/minz/research/abi-paper/sdcc-output/sdcc_abs_diff.c:9: return abs_diff(10, 3);
	ld	l, #0x03
;	spillPairReg hl
;	spillPairReg hl
	ld	a, #0x0a
;/home/alice/dev/minz/research/abi-paper/sdcc-output/sdcc_abs_diff.c:10: }
	jp	_abs_diff
	.area _CODE
	.area _INITIALIZER
	.area _CABS (ABS)
