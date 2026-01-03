;--------------------------------------------------------
; File Created by SDCC : free open source ANSI-C Compiler
; Version 4.2.0 #13081 (Linux)
;--------------------------------------------------------
	.module loop_test
	.optsdcc -mz80
	
;--------------------------------------------------------
; Public variables in this module
;--------------------------------------------------------
	.globl _main
	.globl _sum_array
	.globl _fill_screen
	.globl _screen
;--------------------------------------------------------
; special function registers
;--------------------------------------------------------
;--------------------------------------------------------
; ram data
;--------------------------------------------------------
	.area _DATA
_screen::
	.ds 256
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
;/home/alice/dev/minz/minzc/benchmarks/c_vs_minz/loop_test.c:6: void fill_screen(unsigned char value) {
;	---------------------------------
; Function fill_screen
; ---------------------------------
_fill_screen::
	ld	c, a
;/home/alice/dev/minz/minzc/benchmarks/c_vs_minz/loop_test.c:8: for (i = 0; i < 255; i++) {
	ld	e, #0x00
00102$:
;/home/alice/dev/minz/minzc/benchmarks/c_vs_minz/loop_test.c:9: screen[i] = value;
	ld	hl, #_screen
	ld	d, #0x00
	add	hl, de
	ld	(hl), c
;/home/alice/dev/minz/minzc/benchmarks/c_vs_minz/loop_test.c:8: for (i = 0; i < 255; i++) {
	inc	e
	ld	a, e
	sub	a, #0xff
	jr	C, 00102$
;/home/alice/dev/minz/minzc/benchmarks/c_vs_minz/loop_test.c:11: }
	ret
;/home/alice/dev/minz/minzc/benchmarks/c_vs_minz/loop_test.c:13: unsigned int sum_array(void) {
;	---------------------------------
; Function sum_array
; ---------------------------------
_sum_array::
;/home/alice/dev/minz/minzc/benchmarks/c_vs_minz/loop_test.c:14: unsigned int total = 0;
	ld	de, #0x0000
;/home/alice/dev/minz/minzc/benchmarks/c_vs_minz/loop_test.c:16: for (i = 0; i < 255; i++) {
	ld	c, #0x00
00102$:
;/home/alice/dev/minz/minzc/benchmarks/c_vs_minz/loop_test.c:17: total += screen[i];
	ld	hl, #_screen
	ld	b, #0x00
	add	hl, bc
	ld	l, (hl)
;	spillPairReg hl
	ld	h, #0x00
;	spillPairReg hl
;	spillPairReg hl
	add	hl, de
	ex	de, hl
;/home/alice/dev/minz/minzc/benchmarks/c_vs_minz/loop_test.c:16: for (i = 0; i < 255; i++) {
	inc	c
	ld	a, c
	sub	a, #0xff
	jr	C, 00102$
;/home/alice/dev/minz/minzc/benchmarks/c_vs_minz/loop_test.c:19: return total;
;/home/alice/dev/minz/minzc/benchmarks/c_vs_minz/loop_test.c:20: }
	ret
;/home/alice/dev/minz/minzc/benchmarks/c_vs_minz/loop_test.c:22: void main(void) {
;	---------------------------------
; Function main
; ---------------------------------
_main::
;/home/alice/dev/minz/minzc/benchmarks/c_vs_minz/loop_test.c:23: fill_screen(42);
	ld	a, #0x2a
	call	_fill_screen
;/home/alice/dev/minz/minzc/benchmarks/c_vs_minz/loop_test.c:24: unsigned int result = sum_array();
;/home/alice/dev/minz/minzc/benchmarks/c_vs_minz/loop_test.c:25: }
	jp	_sum_array
	.area _CODE
	.area _INITIALIZER
	.area _CABS (ABS)
