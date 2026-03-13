;--------------------------------------------------------
; File Created by SDCC : free open source ANSI-C Compiler
; Version 4.2.0 #13081 (Linux)
;--------------------------------------------------------
	.module sdcc_fib
	.optsdcc -mz80
	
;--------------------------------------------------------
; Public variables in this module
;--------------------------------------------------------
	.globl _fib
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
;/tmp/sdcc_fib.c:3: uint16_t fib(uint8_t n) {
;	---------------------------------
; Function fib
; ---------------------------------
_fib::
	ld	e, a
;/tmp/sdcc_fib.c:4: if (n <= 1) return n;
	ld	a, #0x01
	sub	a, e
	jr	C, 00102$
	ld	d, #0x00
	ret
00102$:
;/tmp/sdcc_fib.c:5: return fib(n - 1) + fib(n - 2);
	ld	c, e
	dec	c
	push	de
	ld	a, c
	call	_fib
	ex	de, hl
	pop	de
	dec	e
	dec	e
	push	hl
	ld	a, e
	call	_fib
	pop	hl
	add	hl, de
	ex	de, hl
;/tmp/sdcc_fib.c:6: }
	ret
	.area _CODE
	.area _INITIALIZER
	.area _CABS (ABS)
