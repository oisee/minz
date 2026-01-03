;--------------------------------------------------------
; File Created by SDCC : free open source ANSI-C Compiler
; Version 4.2.0 #13081 (Linux)
;--------------------------------------------------------
	.module fibonacci
	.optsdcc -mz80
	
;--------------------------------------------------------
; Public variables in this module
;--------------------------------------------------------
	.globl _main
	.globl _fibonacci
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
;/home/alice/dev/minz/minzc/benchmarks/c_vs_minz/fibonacci.c:4: unsigned char fibonacci(unsigned char n) {
;	---------------------------------
; Function fibonacci
; ---------------------------------
_fibonacci::
	ld	c, a
;/home/alice/dev/minz/minzc/benchmarks/c_vs_minz/fibonacci.c:5: if (n <= 1) {
	ld	a, #0x01
	sub	a, c
	jr	C, 00102$
;/home/alice/dev/minz/minzc/benchmarks/c_vs_minz/fibonacci.c:6: return n;
	ld	a, c
	ret
00102$:
;/home/alice/dev/minz/minzc/benchmarks/c_vs_minz/fibonacci.c:8: return fibonacci(n - 1) + fibonacci(n - 2);
	ld	b, c
	dec	b
	push	bc
	ld	a, b
	call	_fibonacci
	ld	e, a
	pop	bc
	dec	c
	dec	c
	push	de
	ld	a, c
	call	_fibonacci
	pop	de
	add	a, e
;/home/alice/dev/minz/minzc/benchmarks/c_vs_minz/fibonacci.c:9: }
	ret
;/home/alice/dev/minz/minzc/benchmarks/c_vs_minz/fibonacci.c:11: void main(void) {
;	---------------------------------
; Function main
; ---------------------------------
_main::
;/home/alice/dev/minz/minzc/benchmarks/c_vs_minz/fibonacci.c:12: unsigned char result = fibonacci(10);
	ld	a, #0x0a
;/home/alice/dev/minz/minzc/benchmarks/c_vs_minz/fibonacci.c:14: }
	jp	_fibonacci
	.area _CODE
	.area _INITIALIZER
	.area _CABS (ABS)
