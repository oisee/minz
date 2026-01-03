;--------------------------------------------------------
; File Created by SDCC : free open source ANSI-C Compiler
; Version 4.2.0 #13081 (Linux)
;--------------------------------------------------------
	.module arithmetic
	.optsdcc -mz80
	
;--------------------------------------------------------
; Public variables in this module
;--------------------------------------------------------
	.globl _main
	.globl _mul8
	.globl _sub16
	.globl _add16
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
;/home/alice/dev/minz/minzc/benchmarks/c_vs_minz/arithmetic.c:4: unsigned int add16(unsigned int a, unsigned int b) {
;	---------------------------------
; Function add16
; ---------------------------------
_add16::
;/home/alice/dev/minz/minzc/benchmarks/c_vs_minz/arithmetic.c:5: return a + b;
	add	hl, de
	ex	de, hl
;/home/alice/dev/minz/minzc/benchmarks/c_vs_minz/arithmetic.c:6: }
	ret
;/home/alice/dev/minz/minzc/benchmarks/c_vs_minz/arithmetic.c:8: unsigned int sub16(unsigned int a, unsigned int b) {
;	---------------------------------
; Function sub16
; ---------------------------------
_sub16::
;/home/alice/dev/minz/minzc/benchmarks/c_vs_minz/arithmetic.c:9: return a - b;
	cp	a, a
	sbc	hl, de
	ex	de, hl
;/home/alice/dev/minz/minzc/benchmarks/c_vs_minz/arithmetic.c:10: }
	ret
;/home/alice/dev/minz/minzc/benchmarks/c_vs_minz/arithmetic.c:12: unsigned int mul8(unsigned char a, unsigned char b) {
;	---------------------------------
; Function mul8
; ---------------------------------
_mul8::
	ld	c, a
	ld	e, l
;/home/alice/dev/minz/minzc/benchmarks/c_vs_minz/arithmetic.c:13: return (unsigned int)a * (unsigned int)b;
	ld	l, c
;	spillPairReg hl
;	spillPairReg hl
;	spillPairReg hl
;	spillPairReg hl
	ld	h, #0x00
	ld	d, h
;/home/alice/dev/minz/minzc/benchmarks/c_vs_minz/arithmetic.c:14: }
	jp	__mulint
;/home/alice/dev/minz/minzc/benchmarks/c_vs_minz/arithmetic.c:16: void main(void) {
;	---------------------------------
; Function main
; ---------------------------------
_main::
;/home/alice/dev/minz/minzc/benchmarks/c_vs_minz/arithmetic.c:17: unsigned int x = add16(1000, 2000);
	ld	de, #0x07d0
	ld	hl, #0x03e8
	call	_add16
;/home/alice/dev/minz/minzc/benchmarks/c_vs_minz/arithmetic.c:18: unsigned int y = sub16(5000, 1000);
	ld	de, #0x03e8
	ld	hl, #0x1388
	call	_sub16
;/home/alice/dev/minz/minzc/benchmarks/c_vs_minz/arithmetic.c:19: unsigned int z = mul8(25, 10);
	ld	l, #0x0a
;	spillPairReg hl
;	spillPairReg hl
	ld	a, #0x19
;/home/alice/dev/minz/minzc/benchmarks/c_vs_minz/arithmetic.c:20: }
	jp	_mul8
	.area _CODE
	.area _INITIALIZER
	.area _CABS (ABS)
