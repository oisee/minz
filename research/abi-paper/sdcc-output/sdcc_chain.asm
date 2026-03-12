;--------------------------------------------------------
; File Created by SDCC : free open source ANSI-C Compiler
; Version 4.2.0 #13081 (Linux)
;--------------------------------------------------------
	.module sdcc_chain
	.optsdcc -mz80
	
;--------------------------------------------------------
; Public variables in this module
;--------------------------------------------------------
	.globl _main
	.globl _top
	.globl _middle
	.globl _leaf
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
;sdcc_chain.c:3: uint8_t leaf(uint8_t x) {
;	---------------------------------
; Function leaf
; ---------------------------------
_leaf::
;sdcc_chain.c:4: return x + x;
	add	a, a
;sdcc_chain.c:5: }
	ret
;sdcc_chain.c:7: uint8_t middle(uint8_t y) {
;	---------------------------------
; Function middle
; ---------------------------------
_middle::
;sdcc_chain.c:8: return leaf(y);
	ld	c, a
;sdcc_chain.c:9: }
	jp	_leaf
;sdcc_chain.c:11: uint8_t top(uint8_t z) {
;	---------------------------------
; Function top
; ---------------------------------
_top::
;sdcc_chain.c:12: return middle(z);
	ld	c, a
;sdcc_chain.c:13: }
	jp	_middle
;sdcc_chain.c:15: uint8_t main(void) {
;	---------------------------------
; Function main
; ---------------------------------
_main::
;sdcc_chain.c:16: return top(5);
	ld	a, #0x05
;sdcc_chain.c:17: }
	jp	_top
	.area _CODE
	.area _INITIALIZER
	.area _CABS (ABS)
