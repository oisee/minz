;--------------------------------------------------------
; File Created by SDCC : free open source ANSI-C Compiler
; Version 4.2.0 #13081 (Linux)
;--------------------------------------------------------
	.module copy_bytes
	.optsdcc -mz80
	
;--------------------------------------------------------
; Public variables in this module
;--------------------------------------------------------
	.globl _execute
	.globl _copy_bytes
	.globl _result
	.globl _n
	.globl _target
	.globl _source
;--------------------------------------------------------
; special function registers
;--------------------------------------------------------
;--------------------------------------------------------
; ram data
;--------------------------------------------------------
	.area _DATA
_source::
	.ds 64
_target::
	.ds 64
_n::
	.ds 1
_result::
	.ds 2
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
;/home/alice/dev/minz-artifacts/grace-workspaces/copy-fat12-z80/research/c89-benchmark/cases/copy_bytes.c:3: void copy_bytes(unsigned char *dst, unsigned char *src, unsigned char count) {
;	---------------------------------
; Function copy_bytes
; ---------------------------------
_copy_bytes::
	push	ix
	ld	ix,#0
	add	ix,sp
	push	af
	push	af
	ld	-2 (ix), l
	ld	-1 (ix), h
;/home/alice/dev/minz-artifacts/grace-workspaces/copy-fat12-z80/research/c89-benchmark/cases/copy_bytes.c:5: for (i = 0; i < count; i++) dst[i] = src[i];
	ld	c, #0x00
00103$:
	ld	a, c
	sub	a, 4 (ix)
	jr	NC, 00105$
	ld	a, -2 (ix)
	add	a, c
	ld	-4 (ix), a
	ld	a, -1 (ix)
	adc	a, #0x00
	ld	-3 (ix), a
	ld	l, c
	ld	h, #0x00
	add	hl, de
	ld	a, (hl)
	pop	hl
	push	hl
	ld	(hl), a
	inc	c
	jr	00103$
00105$:
;/home/alice/dev/minz-artifacts/grace-workspaces/copy-fat12-z80/research/c89-benchmark/cases/copy_bytes.c:6: }
	ld	sp, ix
	pop	ix
	pop	hl
	inc	sp
	jp	(hl)
;/home/alice/dev/minz-artifacts/grace-workspaces/copy-fat12-z80/research/c89-benchmark/cases/copy_bytes.c:7: void execute(void) { copy_bytes(target, source, n); result = n; }
;	---------------------------------
; Function execute
; ---------------------------------
_execute::
	ld	a, (_n+0)
	push	af
	inc	sp
	ld	de, #_source
	ld	hl, #_target
	call	_copy_bytes
	ld	a, (_n+0)
	ld	(_result+0), a
	ld	hl, #_result + 1
	ld	(hl), #0x00
	ret
	.area _CODE
	.area _INITIALIZER
	.area _CABS (ABS)
