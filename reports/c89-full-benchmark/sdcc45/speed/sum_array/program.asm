;--------------------------------------------------------
; File Created by SDCC : free open source ISO C Compiler
; Version 4.5.0 #15242 (Linux)
;--------------------------------------------------------
	.module sum_array
	
	.optsdcc -mz80 sdcccall(1)
;--------------------------------------------------------
; Public variables in this module
;--------------------------------------------------------
	.globl _execute
	.globl _sum_array
	.globl _result
	.globl _n
	.globl _values
;--------------------------------------------------------
; special function registers
;--------------------------------------------------------
;--------------------------------------------------------
; ram data
;--------------------------------------------------------
	.area _DATA
_values::
	.ds 256
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
;/home/alice/dev/minz-artifacts/grace-workspaces/copy-fat12-z80/research/c89-benchmark/cases/sum_array.c:3: unsigned short sum_array(unsigned char *p, unsigned char count) {
;	---------------------------------
; Function sum_array
; ---------------------------------
_sum_array::
	push	ix
	ld	ix,#0
	add	ix,sp
	push	hl
;/home/alice/dev/minz-artifacts/grace-workspaces/copy-fat12-z80/research/c89-benchmark/cases/sum_array.c:4: unsigned short sum = 0;
	ld	de, #0x0000
;/home/alice/dev/minz-artifacts/grace-workspaces/copy-fat12-z80/research/c89-benchmark/cases/sum_array.c:6: for (i = 0; i < count; i++) sum += p[i];
	ld	c, #0x00
00103$:
	ld	a, c
	sub	a, 4 (ix)
	jr	NC, 00101$
	pop	hl
	push	hl
	ld	b, #0x00
	add	hl, bc
	ld	l, (hl)
	ld	h, #0x00
	add	hl, de
	ex	de, hl
	inc	c
	jp	00103$
00101$:
;/home/alice/dev/minz-artifacts/grace-workspaces/copy-fat12-z80/research/c89-benchmark/cases/sum_array.c:7: return sum;
;/home/alice/dev/minz-artifacts/grace-workspaces/copy-fat12-z80/research/c89-benchmark/cases/sum_array.c:8: }
	ld	sp, ix
	pop	ix
	pop	hl
	inc	sp
	jp	(hl)
;/home/alice/dev/minz-artifacts/grace-workspaces/copy-fat12-z80/research/c89-benchmark/cases/sum_array.c:9: void execute(void) { result = sum_array(values, n); }
;	---------------------------------
; Function execute
; ---------------------------------
_execute::
	ld	a, (_n)
	push	af
	inc	sp
	ld	hl, #_values
	call	_sum_array
	ld	(_result), de
	ret
	.area _CODE
	.area _INITIALIZER
	.area _CABS (ABS)
